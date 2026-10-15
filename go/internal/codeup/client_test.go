package codeup

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestClientListCreateAndHooks(t *testing.T) {
	var mu sync.Mutex
	var hits []string
	var createBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		hits = append(hits, r.Method+" "+r.RequestURI)
		mu.Unlock()
		if r.Header.Get("x-yunxiao-token") != "tok" {
			t.Errorf("token = %s", r.Header.Get("x-yunxiao-token"))
		}
		switch {
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/repositories") && !strings.Contains(r.URL.Path, "/changeRequests") && !strings.Contains(r.URL.Path, "/webhooks") && !strings.Contains(r.URL.Path, "/branches"):
			if strings.Contains(r.URL.RawQuery, "page=") {
				w.Header().Set("x-page", "1")
				w.Header().Set("x-total-pages", "1")
				w.Header().Set("x-total", "1")
				writeJSON(w, []map[string]any{repoJSON()})
				return
			}
			if !strings.Contains(r.RequestURI, "org-example%2Fgroup%2Fdemo") && !strings.Contains(r.RequestURI, "/repositories/12") {
				t.Errorf("repo uri = %s", r.RequestURI)
			}
			writeJSON(w, repoJSON())
		case strings.Contains(r.URL.Path, "/branches/"):
			if strings.Contains(r.URL.EscapedPath(), "missing") {
				http.NotFound(w, r)
				return
			}
			writeJSON(w, map[string]any{"name": "feature/x"})
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/changeRequests") && !strings.Contains(r.URL.Path, "/merge"):
			buf, _ := io.ReadAll(r.Body)
			mu.Lock()
			createBody = buf
			mu.Unlock()
			writeJSON(w, crJSON())
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/merge"):
			writeJSON(w, map[string]any{"ok": true})
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/changeRequests/"):
			writeJSON(w, crJSON())
		case strings.Contains(r.URL.Path, "/webhooks"):
			if !strings.Contains(r.URL.Path, "/repositories/12/webhooks") {
				t.Errorf("hook path = %s", r.URL.Path)
			}
			writeJSON(w, []map[string]any{hookJSON()})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	cli := testClient(srv)

	repos, err := cli.ListRepos(repoQuery{Page: 1, PerPage: 50, Search: "demo"})
	if err != nil || len(repos) != 1 || repos[0].PathNs != "org-example/group/demo" {
		t.Fatalf("repos = %+v %v", repos, err)
	}
	mu.Lock()
	listURI := hits[0]
	mu.Unlock()
	if !strings.Contains(listURI, "search=demo") || !strings.Contains(listURI, "orderBy=name") || !strings.Contains(listURI, "sort=asc") {
		t.Fatalf("list uri = %s", listURI)
	}

	cr, err := cli.CreateCr(crCreate{Repo: "group/demo", Source: "feature/x", Target: "master", Title: "t"})
	if err != nil {
		t.Fatal(err)
	}
	if cr.LocalID != "7" || cr.CrURL != "https://codeup.aliyun.com/org-example/group/demo/change/7" {
		t.Fatalf("cr = %+v", cr)
	}
	if string(createBody) != `{"title":"t","sourceBranch":"feature/x","targetBranch":"master","sourceProjectId":12,"targetProjectId":12}` {
		t.Fatalf("body = %s", createBody)
	}

	merged, err := cli.MergeCr("group/demo", "7", crMerge{Type: "squash"})
	if err != nil || merged.State != "merged" {
		t.Fatalf("merged = %+v %v", merged, err)
	}

	hooks, err := cli.ListHooks("group/demo", 1, 50)
	if err != nil || hooks[0].SecretToken != "s3cret" || !hooks[0].PushEvents {
		t.Fatalf("hooks = %+v %v", hooks, err)
	}
}

func TestClientBranchMissingAndRedact(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/branches/") {
			http.Error(w, "org-example missing", http.StatusNotFound)
			return
		}
		if strings.Contains(r.URL.RawQuery, "perPage=100") {
			w.Header().Set("x-page", "1")
			w.Header().Set("x-total-pages", "201")
			w.Header().Set("x-total", "1")
			writeJSON(w, []any{})
			return
		}
		if strings.Contains(r.URL.RawQuery, "page=") {
			w.Header().Set("x-page", "9")
			w.Header().Set("x-total-pages", "1")
			w.Header().Set("x-total", "0")
			writeJSON(w, []any{})
			return
		}
		writeJSON(w, repoJSON())
	}))
	defer srv.Close()
	cli := testClient(srv)
	_, err := cli.CreateCr(crCreate{Repo: "group/demo", Source: "missing", Target: "master", Title: "t"})
	var exit *ExitErr
	if err == nil || !asErr(err, &exit) || exit.Code != 1 || !contains(err.Error(), "source branch not on remote: missing") {
		t.Fatalf("branch = %v", err)
	}
	if contains(err.Error(), "org-example") {
		t.Fatalf("org leaked: %s", err.Error())
	}
	_, err = cli.ListRepos(repoQuery{Page: 1, PerPage: 10})
	if err == nil || !asErr(err, &exit) || exit.Code != 3 || !contains(err.Error(), "page mismatch") {
		t.Fatalf("page = %v", err)
	}
	_, err = cli.ListAllRepos()
	if err == nil || !contains(err.Error(), "exceeded 200 pages") {
		t.Fatalf("pages = %v", err)
	}
}

func TestDispatchInitDoesNotTouchConfig(t *testing.T) {
	home := setHome(t)
	var pages []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pages = append(pages, r.URL.Query().Get("page"))
		w.Header().Set("x-page", r.URL.Query().Get("page"))
		w.Header().Set("x-total-pages", "2")
		w.Header().Set("x-total", "2")
		name := "a"
		if r.URL.Query().Get("page") == "2" {
			name = "b"
		}
		writeJSON(w, []map[string]any{{
			"id": json.Number("1"), "name": name, "path": name,
			"pathWithNamespace": "org-example/group/" + name,
			"webUrl":            "https://example/" + name, "defaultBranch": "master",
		}})
	}))
	defer srv.Close()
	rt := Runtime{
		Audience: audienceHuman,
		Profile:  Profile{Name: "work", URL: "https://codeup.aliyun.com/org-example", Token: "tok"},
		OrgID:    "org-example",
		Token:    "tok",
		APIBase:  APIBase,
	}
	text, err := Dispatch(Cmd{Kind: kindInit, Audience: audienceHuman}, rt, runEnv{Client: testClient(srv)})
	if err != nil {
		t.Fatal(err)
	}
	if text != "indexed 2 repositories\n" {
		t.Fatalf("text = %q", text)
	}
	if len(pages) != 2 {
		t.Fatalf("pages = %v", pages)
	}
	if _, err := osStat(home + "/.config/codeup-cli"); !osNotExist(err) {
		t.Fatalf("config written: %v", err)
	}
}

func testClient(srv *httptest.Server) *Client {
	return &Client{
		RT:   Runtime{OrgID: "org-example", Token: "tok", APIBase: APIBase},
		Base: srv.URL,
		HTTP: srv.Client(),
	}
}

func repoJSON() map[string]any {
	return map[string]any{
		"id":                json.Number("12"),
		"name":              "demo",
		"path":              "demo",
		"pathWithNamespace": "org-example/group/demo",
		"webUrl":            "https://codeup.aliyun.com/org-example/group/demo",
		"defaultBranch":     "master",
	}
}

func crJSON() map[string]any {
	return map[string]any{
		"localId":        json.Number("7"),
		"title":          "t",
		"state":          "merged",
		"sourceBranch":   "feature/x",
		"targetBranch":   "master",
		"authorUserName": "ray",
		"createdAt":      "c",
		"updatedAt":      "u",
		"webUrl":         "https://codeup.aliyun.com/org-example/group/demo",
		"description":    "",
	}
}

func hookJSON() map[string]any {
	return map[string]any{
		"id":                 json.Number("3"),
		"url":                "https://example.com/hook?token=s3cret",
		"token":              "s3cret",
		"pushEvents":         true,
		"mergeRequestEvents": false,
		"tagPushEvents":      false,
		"noteEvents":         true,
		"createdAt":          "c",
		"updatedAt":          "u",
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	_ = enc.Encode(v)
}

func asErr(err error, dest **ExitErr) bool {
	exit, ok := err.(*ExitErr)
	if !ok {
		return false
	}
	*dest = exit
	return true
}
