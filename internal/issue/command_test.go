package issue

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const testToken = "example-token-not-a-real-credential"

// harness 每次使用独立 httptest 和内存配置，防止测试触碰真实账户。
func harness(t *testing.T, h http.HandlerFunc) (Options, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	out, errOut := new(bytes.Buffer), new(bytes.Buffer)
	opts := Options{Out: out, Err: errOut, HTTP: srv.Client(), Load: func(string) (Config, error) { return Config{URL: srv.URL, Token: testToken}, nil }, ReadFile: func(string) ([]byte, error) {
		return []byte(`{"expectedVersion":2,"expectedTreeVersion":7,"changes":{},"kind":"tag"}`), nil
	}}
	return opts, out, errOut
}

func TestResourceContractRoutes(t *testing.T) {
	// 此表固定服务实际路径，而非从实现推导预期值，避免路由拼错仍自证通过。
	cases := []struct{ args, method, path string }{
		{"items list", "GET", "/api/work-items"},
		{"items get a", "GET", "/api/work-items/a"},
		{"items children a", "GET", "/api/work-items/a/children"},
		{"items preview a", "GET", "/api/work-items/a/preview"},
		{"items create", "POST", "/api/work-items"},
		{"items update a", "PATCH", "/api/work-items/a"},
		{"items move a", "POST", "/api/work-items/a/move"},
		{"items delete a", "DELETE", "/api/work-items/a"},
		{"dashboard my-work", "GET", "/api/dashboard/my-work"},
		{"dashboard owned", "GET", "/api/dashboard/owned"},
		{"dashboard issues", "GET", "/api/dashboard/issues"},
		{"activities list a", "GET", "/api/work-items/a/activities"},
		{"activities feed", "GET", "/api/activities"},
		{"activities create a", "POST", "/api/work-items/a/activities"},
		{"activities update a", "PATCH", "/api/activities/a"},
		{"activities revisions a", "GET", "/api/activities/a/revisions"},
		{"knowledge get a", "GET", "/api/work-items/a/knowledge"},
		{"knowledge update a", "PATCH", "/api/work-items/a/knowledge"},
		{"relations list a", "GET", "/api/work-items/a/relations"},
		{"relations create a", "POST", "/api/work-items/a/relations"},
		{"relations update a b", "PATCH", "/api/work-items/a/relations/b"},
		{"relations delete a b", "DELETE", "/api/work-items/a/relations/b"},
		{"successors create a", "POST", "/api/work-items/a/successors"},
		{"templates list", "GET", "/api/templates"},
		{"templates save", "POST", "/api/templates"},
		{"templates update a", "PATCH", "/api/templates/a"},
		{"templates preview a", "POST", "/api/templates/a/preview"},
		{"templates instantiate a", "POST", "/api/templates/a/instantiate"},
		{"trash list", "GET", "/api/trash/batches"},
		{"trash get a", "GET", "/api/trash/batches/a"},
		{"trash restore a", "POST", "/api/trash/batches/a/restore"},
		{"workspace snapshot", "GET", "/api/snapshot"},
		{"workspace workflow", "POST", "/api/workspace/workflows"},
		{"workspace workgroups", "POST", "/api/workspace/workgroups"},
		{"workspace dictionary", "POST", "/api/workspace/dictionaries"},
	}
	for _, tc := range cases {
		t.Run(tc.args, func(t *testing.T) {
			calls := 0
			opts, out, errOut := harness(t, func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != tc.method || r.URL.Path != tc.path {
					t.Errorf("got %s %s", r.Method, r.URL.Path)
				}
				if r.Header.Get("Authorization") != "Bearer "+testToken {
					t.Error("missing Bearer")
				}
				if tc.method != "GET" {
					if r.Header.Get("Content-Type") != "application/json" {
						t.Error("missing content type")
					}
					var body map[string]any
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Error(err)
					}
					if tc.args == "items delete a" && body["confirmed"] != true {
						t.Error("missing confirmed")
					}
				}
				_, _ = io.WriteString(w, `{"ok":true}`)
			})
			args := strings.Fields(tc.args)
			if tc.method != "GET" {
				args = append(args, "--body-file", "body.json", "--confirm")
			}
			args = append(args, "--audience", "agent")
			if code := Execute(args, opts); code != 0 {
				t.Fatalf("exit %d: %s", code, errOut)
			}
			if calls != 1 || strings.TrimSpace(out.String()) != `{"ok":true}` || errOut.Len() != 0 {
				t.Fatalf("calls=%d stdout=%s stderr=%s", calls, out, errOut)
			}
		})
	}
}

func TestUnsafeInputNeverCallsServer(t *testing.T) {
	cases := []struct {
		args []string
		body string
	}{
		{[]string{"items", "delete", "a", "--body-file", "x"}, `{"expectedVersion":1,"expectedTreeVersion":1,"confirmed":true}`},
		{[]string{"items", "update", "a", "--body-file", "x"}, `{"changes":{}}`},
		{[]string{"items", "move", "a", "--body-file", "x"}, `{"expectedVersion":1}`},
		{[]string{"items", "update", "a", "--body-file", "x"}, `{"expectedVersion":1,"changes":{},"confirmUnfinishedDescendants":true,"expectedTreeVersion":2}`},
		{[]string{"items", "update", "a", "--body-file", "x", "--confirm"}, `{"expectedVersion":1,"changes":{},"confirmUnfinishedDescendants":true}`},
		{[]string{"items", "update", "a", "--body-file", "x"}, `{"expectedVersion":1.5}`},
		{[]string{"templates", "save", "--body-file", "x"}, `{"id":"a"}`},
		{[]string{"workspace", "workflow", "--body-file", "x"}, `{"sourceId":"a"}`},
		{[]string{"workspace", "dictionary", "--body-file", "x"}, `{"kind":"role"}`},
		{[]string{"workspace", "dictionary", "--body-file", "x"}, `{"kind":"tag","id":"a"}`},
		{[]string{"knowledge", "update", "a", "--body-file", "x"}, `null`},
		{[]string{"items", "create", "--body-file", "x"}, `{} {}`},
		{[]string{"items", "create"}, `{}`},
		{[]string{"items", "get", "../sa"}, `{}`},
		{[]string{"items", "get", "a%2fb"}, `{}`},
		{[]string{"items", "list", "--query", "limit=101"}, `{}`},
		{[]string{"items", "list", "--query", "offset=-1"}, `{}`},
		{[]string{"items", "list", "--query", "wat=x"}, `{}`},
		{[]string{"items", "list", "--query", "q=a", "--query", "q=b"}, `{}`},
		{[]string{"items", "get"}, `{}`},
		{[]string{"account"}, `{}`},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			opts, out, errOut := harness(t, func(w http.ResponseWriter, r *http.Request) { t.Error("unexpected HTTP request") })
			opts.ReadFile = func(string) ([]byte, error) { return []byte(tc.body), nil }
			args := append(tc.args, "--audience", "agent")
			if code := Execute(args, opts); code == 0 {
				t.Fatal("must fail")
			}
			if out.Len() != 0 {
				t.Fatalf("unexpected stdout: %s", out)
			}
			var doc map[string]any
			if err := json.Unmarshal(errOut.Bytes(), &doc); err != nil {
				t.Fatalf("not JSON: %s", errOut)
			}
		})
	}
}

func TestConflictNoRetryAndRedaction(t *testing.T) {
	calls := 0
	opts, out, errOut := harness(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(409)
		// JSON 转义后的凭据也必须脱敏。
		tok := string(rune(92)) + "u0065" + testToken[1:]
		fmt.Fprintf(w, `{"code":"VERSION_CONFLICT","message":"%s","requestId":"trace-1","details":{"secret":"%s"}}`, tok, tok)
	})
	args := []string{"items", "update", "a", "--body-file", "x", "--audience", "agent"}
	if Execute(args, opts) != 1 || calls != 1 || out.Len() != 0 {
		t.Fatalf("calls=%d out=%s", calls, out)
	}
	if strings.Contains(errOut.String(), testToken) || !strings.Contains(errOut.String(), "[REDACTED]") || !strings.Contains(errOut.String(), "trace-1") || !strings.Contains(errOut.String(), "409") {
		t.Fatal(errOut.String())
	}
}

func TestSearchKeepsTreeAndPaging(t *testing.T) {
	doc := `{"items":[{"id":"root","status":"TODO"}],"treeItems":[{"id":"root"},{"id":"child","status":"COMPLETED"}],"total":23,"nextOffset":10}`
	opts, out, errOut := harness(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("q") != "a&b" || r.URL.Query().Get("limit") != "10" || r.URL.Query().Get("offset") != "0" {
			t.Error(r.URL.RawQuery)
		}
		_, _ = io.WriteString(w, doc)
	})
	args := []string{"items", "list", "--query", "q=a&b", "--query", "limit=10", "--query", "offset=0", "--audience", "agent"}
	if Execute(args, opts) != 0 {
		t.Fatal(errOut)
	}
	var got map[string]any
	_ = json.Unmarshal(out.Bytes(), &got)
	if len(got["items"].([]any)) != 1 || len(got["treeItems"].([]any)) != 2 || got["nextOffset"] != float64(10) {
		t.Fatal(out)
	}
}

func TestTransitionAndKnowledgeInitialVersion(t *testing.T) {
	for _, tc := range []struct {
		args []string
		body string
	}{
		{[]string{"items", "transition", "a", "--confirm"}, `{"expectedVersion":3,"expectedTreeVersion":8,"changes":{},"transitionTarget":{"type":"STATE","id":"done"},"confirmUnfinishedDescendants":true}`},
		{[]string{"knowledge", "update", "a"}, `{"action":"save","expectedVersion":0,"links":[],"conclusion":"结论"}`},
	} {
		opts, _, errOut := harness(t, func(w http.ResponseWriter, r *http.Request) {
			raw, _ := io.ReadAll(r.Body)
			var want, got any
			_ = json.Unmarshal([]byte(tc.body), &want)
			_ = json.Unmarshal(raw, &got)
			a, _ := json.Marshal(want)
			b, _ := json.Marshal(got)
			if !bytes.Equal(a, b) {
				t.Errorf("payload changed: %s", raw)
			}
			_, _ = io.WriteString(w, `{"version":4}`)
		})
		opts.ReadFile = func(string) ([]byte, error) { return []byte(tc.body), nil }
		args := append(tc.args, "--body-file", "x")
		if Execute(args, opts) != 0 {
			t.Fatal(errOut)
		}
	}
}

func TestConfigReadOnlyAndSanitized(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "missing.json")
	if _, err := LoadConfig(path); err == nil {
		t.Fatal("missing config accepted")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("created config")
	}
	cases := []struct {
		body string
		ok   bool
	}{
		{`{"url":"https://example.invalid/","token":"EXAMPLE_TOKEN"}`, true},
		{`{"url":"https://example.invalid","token":"EXAMPLE_TOKEN","profile":"p"}`, false},
		{`{"url":"https://u:EXAMPLE_TOKEN@example.invalid","token":"EXAMPLE_TOKEN"}`, false},
		{`{"url":"https://example.invalid/api","token":"EXAMPLE_TOKEN"}`, false},
		{`{"url":"https://example.invalid?secret=EXAMPLE_TOKEN","token":"EXAMPLE_TOKEN"}`, false},
		{`{"url":"https://example.invalid","token":""}`, false},
		{`{"url":"https://example.invalid","token":"EXAMPLE_TOKEN"} {}`, false},
	}
	for _, tc := range cases {
		if err := os.WriteFile(path, []byte(tc.body), 0600); err != nil {
			t.Fatal(err)
		}
		_, err := LoadConfig(path)
		if (err == nil) != tc.ok {
			t.Fatalf("ok=%v err=%v", tc.ok, err)
		}
		if err != nil && strings.Contains(err.Error(), "EXAMPLE_TOKEN") {
			t.Fatal("leaked token")
		}
		raw, _ := os.ReadFile(path)
		if string(raw) != tc.body {
			t.Fatal("config changed")
		}
	}
}

func TestDoctorOfflineAndHuman(t *testing.T) {
	opts, out, errOut := harness(t, func(w http.ResponseWriter, r *http.Request) { t.Error("doctor must not call HTTP") })
	if Execute([]string{"doctor"}, opts) != 0 || out.String() != "pass config\n" || errOut.Len() != 0 {
		t.Fatalf("out=%s err=%s", out, errOut)
	}
	out.Reset()
	if Execute([]string{"doctor", "--audience", "agent"}, opts) != 0 || !json.Valid(out.Bytes()) {
		t.Fatal(out)
	}
}

func TestHumanReadableOutputAndTokenRedaction(t *testing.T) {
	opts, out, errOut := harness(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"items":[{"title":"发布服务","id":"a","secret":"%s"}],"nextOffset":null}`, testToken)
	})
	if Execute([]string{"items", "list"}, opts) != 0 {
		t.Fatal(errOut)
	}
	text := out.String()
	if !strings.Contains(text, "title: 发布服务") || strings.Contains(text, testToken) || !strings.Contains(text, "[REDACTED]") {
		t.Fatal(text)
	}
}

func TestViewsSelectExactOwnerNodes(t *testing.T) {
	doc := `{"user":{"id":"me"},"workspace":{"timezone":"Asia/Shanghai"},"items":[
 {"id":"root","ownerId":"other","status":"TODO","trackingActive":true},
 {"id":"mine","parentId":"root","ownerId":"me","status":"IN_PROGRESS","trackingActive":true,"deadlineDate":"2026-11-10","selfIssues":[]},
 {"id":"late","parentId":"root","ownerId":"me","status":"TODO","trackingActive":true,"deadlineDate":"2026-11-08","selfIssues":[{"type":"OVERDUE"}]},
 {"id":"done","parentId":"root","ownerId":"me","status":"COMPLETED","trackingActive":false},
 {"id":"cancel","ownerId":"me","status":"CANCELLED","trackingActive":false},
 {"id":"suspended","ownerId":"me","status":"IN_PROGRESS","trackingActive":false}
 ]}`
	for _, tc := range []struct {
		view  string
		count int
		id    string
	}{{"unfinished", 3, ""}, {"doing", 1, "mine"}, {"due", 1, "mine"}, {"overdue", 1, "late"}, {"done", 1, "done"}} {
		t.Run(tc.view, func(t *testing.T) {
			opts, out, errOut := harness(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/snapshot" {
					t.Error(r.URL.Path)
				}
				_, _ = io.WriteString(w, doc)
			})
			opts.Now = func() time.Time { return time.Date(2026, 11, 9, 1, 0, 0, 0, time.UTC) }
			if Execute([]string{"me", tc.view, "--audience", "agent"}, opts) != 0 {
				t.Fatal(errOut)
			}
			var got map[string]any
			_ = json.Unmarshal(out.Bytes(), &got)
			vals := got["items"].([]any)
			if len(vals) != tc.count {
				t.Fatal(out)
			}
			if tc.id != "" && vals[0].(map[string]any)["id"] != tc.id {
				t.Fatal(out)
			}
			for _, v := range vals {
				if v.(map[string]any)["ownerId"] != "me" {
					t.Fatal("view leaked other owner")
				}
			}
		})
	}
}
