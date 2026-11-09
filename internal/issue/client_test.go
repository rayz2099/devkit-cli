package issue

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// roundTripFunc 让网络失败可确定复现，不能依赖外部地址或超时竞争。
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestRedirectNeverForwarded(t *testing.T) {
	hits := 0
	dst := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		t.Error("redirect target must not receive token")
	}))
	defer dst.Close()
	src := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Header().Set("Location", dst.URL); w.WriteHeader(307) }))
	defer src.Close()
	cfg := Config{URL: src.URL, Token: testToken}
	cli, err := NewClient(cfg, src.Client())
	if err != nil {
		t.Fatal(err)
	}
	_, err = cli.Do(context.Background(), "POST", "work-items", nil, map[string]any{"title": "example"})
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Status != 307 || hits != 0 {
		t.Fatalf("hits=%d err=%v", hits, err)
	}
}

func TestTransportFailureNeverRetriesOrLeaks(t *testing.T) {
	hits := 0
	hc := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		hits++
		return nil, errors.New("transport included " + testToken)
	})}
	cfg := Config{URL: "https://example.invalid", Token: testToken}
	cli, err := NewClient(cfg, hc)
	if err != nil {
		t.Fatal(err)
	}
	_, err = cli.Do(context.Background(), "PATCH", "work-items/a", nil, map[string]any{"expectedVersion": 1})
	if err == nil || strings.Contains(err.Error(), testToken) || hits != 1 {
		t.Fatalf("hits=%d err=%v", hits, err)
	}
}

func TestMalformedAndServerErrors(t *testing.T) {
	for _, tc := range []struct {
		status int
		body   string
		code   string
	}{{200, "not-json", ""}, {401, `{"code":"TOKEN_INVALID","message":"已撤销","requestId":"req"}`, "TOKEN_INVALID"}, {403, `{"code":"TOKEN_SCOPE_DENIED","message":"只读"}`, "TOKEN_SCOPE_DENIED"}, {500, "<html>bad</html>", "HTTP_ERROR"}} {
		hits := 0
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hits++
			w.WriteHeader(tc.status)
			_, _ = io.WriteString(w, tc.body)
		}))
		cfg := Config{URL: srv.URL, Token: testToken}
		cli, err := NewClient(cfg, srv.Client())
		if err != nil {
			t.Fatal(err)
		}
		_, err = cli.Do(context.Background(), "POST", "work-items", nil, map[string]any{})
		srv.Close()
		if err == nil || hits != 1 {
			t.Fatalf("hits=%d err=%v", hits, err)
		}
		if tc.code != "" {
			var apiErr *APIError
			if !errors.As(err, &apiErr) || apiErr.Code != tc.code || apiErr.Status != tc.status {
				t.Fatal(err)
			}
		}
	}
}

func TestResponseRedactionAfterDecoding(t *testing.T) {
	// 短占位符可与 JSON 关键字重名，脱敏不能在原始字节上破坏 JSON 语法。
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"ok":true,"text":"true","true":"true"}`)
	}))
	defer srv.Close()
	cfg := Config{URL: srv.URL, Token: "true"}
	cli, err := NewClient(cfg, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	value, err := cli.Do(context.Background(), "GET", "snapshot", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	got := value.(map[string]any)
	if got["ok"] != true || got["text"] != "[REDACTED]" || got["[REDACTED]"] != "[REDACTED]" {
		t.Fatal(got)
	}
}

func TestActivityCursorPreserved(t *testing.T) {
	cursor := `[123,"id-a"]`
	opts, out, errOut := harness(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("cursor") != cursor {
			t.Error(r.URL.RawQuery)
		}
		_, _ = io.WriteString(w, `{"items":[],"nextCursor":[120,"id-b"]}`)
	})
	args := []string{"activities", "feed", "--query", "cursor=" + cursor, "--audience", "agent"}
	if Execute(args, opts) != 0 || !strings.Contains(out.String(), `"nextCursor":[120,"id-b"]`) {
		t.Fatalf("out=%s err=%s", out, errOut)
	}
}

func TestSubtreeExcludesOtherBranches(t *testing.T) {
	opts, out, errOut := harness(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"items":[{"id":"a","parentId":null},{"id":"b","parentId":"a"},{"id":"c","parentId":"b"},{"id":"other","parentId":null}]}`)
	})
	if Execute([]string{"items", "subtree", "b", "--audience", "agent"}, opts) != 0 {
		t.Fatal(errOut)
	}
	if strings.Contains(out.String(), "other") || strings.Contains(out.String(), `"id":"a"`) || !strings.Contains(out.String(), `"count":2`) {
		t.Fatal(out)
	}
}
