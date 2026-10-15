package jenkins

import (
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientRejectsMainlineWithoutRequest(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	client := testClient(srv.URL)
	_, err := client.startRun("dt-vrec/master")
	if err == nil || ExitCode(err) != 1 || !strings.Contains(err.Error(), "refusing to start mainline") {
		t.Fatalf("start = %v", err)
	}
	_, err = client.rerun("dt-vrec/main", "1")
	if err == nil || !strings.Contains(err.Error(), "refusing to rerun mainline") {
		t.Fatalf("rerun = %v", err)
	}
	if hits != 0 {
		t.Fatalf("hits = %d", hits)
	}
}

func TestListRunsInProgress(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.RequestURI(), "builds[number,result,building,timestamp,duration]{0,5}") {
			t.Errorf("uri = %s", r.URL.RequestURI())
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"builds":[{"number":3,"result":null,"building":true,"timestamp":1,"duration":0}]}`)
	}))
	defer srv.Close()
	items, err := testClient(srv.URL).listRuns("dt-vrec/feature/x", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Result != "IN_PROGRESS" {
		t.Fatalf("items = %+v", items)
	}
}

func TestStartSendsCrumbAndKeepsRedirect(t *testing.T) {
	var saw []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		saw = append(saw, r.Method+" "+r.URL.Path)
		user := "Basic " + base64.StdEncoding.EncodeToString([]byte("admin:token"))
		if r.Header.Get("Authorization") != user {
			t.Errorf("auth = %s", r.Header.Get("Authorization"))
		}
		if r.URL.Path == "/crumbIssuer/api/json" {
			_, _ = io.WriteString(w, `{"crumb":"abc","crumbRequestField":"Jenkins-Crumb"}`)
			return
		}
		if r.Header.Get("Jenkins-Crumb") != "abc" {
			t.Errorf("crumb = %s", r.Header.Get("Jenkins-Crumb"))
		}
		w.Header().Set("Location", srvLoc(r)+"/queue/item/9/")
		w.WriteHeader(http.StatusFound)
	}))
	defer srv.Close()
	out, err := testClient(srv.URL).startRun("dt-vrec/feature/x")
	if err != nil {
		t.Fatal(err)
	}
	if out.Status != http.StatusFound || out.QueueLocation == nil || !strings.HasSuffix(*out.QueueLocation, "/queue/item/9/") {
		t.Fatalf("out = %+v", out)
	}
	if strings.Join(saw, ",") != "GET /crumbIssuer/api/json,POST /job/dt-vrec/job/feature/job/x/build" {
		t.Fatalf("saw = %v", saw)
	}
}

func TestRerunPostsParameters(t *testing.T) {
	var body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/api/json"):
			_, _ = io.WriteString(w, `{"number":4,"result":"SUCCESS","actions":[{"parameters":[{"name":"BRANCH","value":"feat"},{"name":"N","value":2}]}]}`)
		case r.URL.Path == "/crumbIssuer/api/json":
			w.WriteHeader(http.StatusNotFound)
		default:
			buf, _ := io.ReadAll(r.Body)
			body = string(buf)
			if r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
				t.Errorf("type = %s", r.Header.Get("Content-Type"))
			}
			w.WriteHeader(http.StatusCreated)
		}
	}))
	defer srv.Close()
	out, err := testClient(srv.URL).rerun("app/feature", "4")
	if err != nil {
		t.Fatal(err)
	}
	if out.Status != http.StatusCreated || body != "BRANCH=feat&N=2" {
		t.Fatalf("status %d body %q", out.Status, body)
	}
}

func TestCancelNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/crumbIssuer/api/json" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	err := testClient(srv.URL).cancelRun("app/feature", "9")
	if err == nil || ExitCode(err) != 2 || err.Error() != "Build not found: app/feature #9" {
		t.Fatalf("err = %v", err)
	}
}

func TestLogTailKeepsTrailingNewline(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "a\nb\n")
	}))
	defer srv.Close()
	n := 2
	text, err := testClient(srv.URL).readLog("app/feature", "1", &n)
	if err != nil {
		t.Fatal(err)
	}
	if text != "b\n\n" {
		t.Fatalf("tail = %q", text)
	}
}

func TestNotFoundCarriesTree(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	_, err := testClient(srv.URL).viewJob("missing")
	if err == nil || ExitCode(err) != 2 || !strings.Contains(err.Error(), "not found: /job/missing/api/json?tree=") {
		t.Fatalf("err = %v", err)
	}
}

func testClient(base string) *Client {
	return newClient(runtime{
		aud:    "agent",
		secret: "token",
		prof: Profile{
			Name:     "prod",
			URL:      base,
			User:     "admin",
			APIToken: "token",
		},
	})
}

func srvLoc(r *http.Request) string {
	return "http://" + r.Host
}
