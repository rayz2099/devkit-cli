package td

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestVersionDoesNotTouchConfig(t *testing.T) {
	home := t.TempDir()
	setHome(t, home)
	cfgDir := filepath.Join(home, ".config")
	var stdout, stderr bytes.Buffer
	if code := executeAt(nil, &stdout, &stderr, time.Now()); code != 0 || !strings.Contains(stdout.String(), "td-cli") || stderr.Len() != 0 {
		t.Fatalf("code/out %d %q %q", code, stdout.String(), stderr.String())
	}
	if _, err := os.Stat(cfgDir); !os.IsNotExist(err) {
		t.Fatal("version created config dir")
	}
	stdout.Reset()
	stderr.Reset()
	if code := executeAt([]string{"version"}, &stdout, &stderr, time.Now()); code != 0 || !strings.Contains(stdout.String(), "td-cli 0.1.0") {
		t.Fatalf("version %d %q", code, stdout.String())
	}
	if _, err := os.Stat(cfgDir); !os.IsNotExist(err) {
		t.Fatal("version subcommand created config dir")
	}
}

func TestDoctorConfigOnly(t *testing.T) {
	home := t.TempDir()
	setHome(t, home)
	var stdout, stderr bytes.Buffer
	if code := executeAt([]string{"doctor"}, &stdout, &stderr, time.Now()); code == 0 {
		t.Fatal("missing config should fail")
	}
	if !strings.Contains(stdout.String(), "fail config:") || !strings.Contains(stdout.String(), "config missing") {
		t.Fatal(stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatal(stderr.String())
	}
	if _, err := os.Stat(filepath.Join(home, ".config")); !os.IsNotExist(err) {
		t.Fatal("doctor created config dir")
	}

	path := filepath.Join(home, "good.json")
	os.WriteFile(path, []byte(`{
	  // comment
	  "defaultProfile": "local",
	  "profiles": [{"name": "local", "url": "http://127.0.0.1:9", "username": "ray", "password": "secret",}]
	}`), 0o644)
	stdout.Reset()
	if code := executeAt([]string{"-c", path, "doctor"}, &stdout, &stderr, time.Now()); code != 0 {
		t.Fatalf("doctor %d %s", code, stdout.String())
	}
	if stdout.String() != "pass config\npass fields\n" {
		t.Fatal(stdout.String())
	}

	bad := filepath.Join(home, "bad.json")
	os.WriteFile(bad, []byte(`{"defaultProfile":"local","profiles":[{}]}`), 0o644)
	stdout.Reset()
	stderr.Reset()
	if code := executeAt([]string{"doctor", "-c", bad}, &stdout, &stderr, time.Now()); code == 0 || !strings.Contains(stdout.String(), "fail fields:") {
		t.Fatalf("%d %s", code, stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatal(stderr.String())
	}
}

func TestHelpAndErrors(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := executeAt([]string{"--help"}, &stdout, &stderr, time.Now()); code != 0 || !strings.Contains(stdout.String(), "completion fish") {
		t.Fatalf("%d %s %s", code, stdout.String(), stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := executeAt([]string{"query", "save"}, &stdout, &stderr, time.Now()); code != 1 || stderr.String() != "read-only cli, write is disabled\n" {
		t.Fatalf("%d %q %q", code, stdout.String(), stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := executeAt([]string{"agent", "nope"}, &stdout, &stderr, time.Now()); code != 1 || stderr.Len() != 0 || !strings.Contains(stdout.String(), `"ok": false`) {
		t.Fatalf("%d %s %s", code, stdout.String(), stderr.String())
	}
	stdout.Reset()
	if code := executeAt([]string{"__complete", ""}, &stdout, &stderr, time.Now()); code != 0 || !strings.HasPrefix(stdout.String(), "auth\n") {
		t.Fatal(stdout.String())
	}
	stdout.Reset()
	if code := executeAt([]string{"completion", "fish"}, &stdout, &stderr, time.Now()); code != 0 || !strings.Contains(stdout.String(), "complete -c td-cli") {
		t.Fatal(stdout.String(), code)
	}
}

func TestAuthAndCatalog(t *testing.T) {
	home := t.TempDir()
	setHome(t, home)
	fe := newFakeTE(t)
	path := writeCfg(t, home, fe.url)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	if code := executeAt([]string{"-c", path, "auth", "status"}, &stdout, &stderr, time.Now()); code != 0 {
		t.Fatal(code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "ray") || strings.Contains(stdout.String(), "secret") || fe.sawLogin {
		t.Fatalf("status leaked or logged in: %s", stdout.String())
	}

	stdout.Reset()
	if code := executeAt([]string{"-c", path, "auth", "login"}, &stdout, &stderr, time.Now()); code != 0 {
		t.Fatal(stderr.String())
	}
	if !strings.Contains(stdout.String(), "ok\ttrue") || !fe.sawLogin || fe.plainPassword {
		t.Fatalf("login %s plain=%v", stdout.String(), fe.plainPassword)
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Fatal("login rewrote config")
	}

	stdout.Reset()
	stderr.Reset()
	if code := executeAt([]string{"-c", path, "project", "ls"}, &stdout, &stderr, time.Now()); code != 0 || !strings.Contains(stdout.String(), "demo") {
		t.Fatalf("%d %s %s", code, stdout.String(), stderr.String())
	}
	stdout.Reset()
	if code := executeAt([]string{"-c", path, "meta", "events", "--project", "3"}, &stdout, &stderr, time.Now()); code != 0 || !strings.Contains(stdout.String(), "login") {
		t.Fatal(stdout.String(), stderr.String())
	}
	if !fe.sawStamp {
		t.Fatal("missing @t")
	}
	stdout.Reset()
	if code := executeAt([]string{"-c", path, "meta", "props", "--project", "3", "--event", "login"}, &stdout, &stderr, time.Now()); code != 0 || !strings.Contains(stdout.String(), "city") {
		t.Fatal(stdout.String(), stderr.String())
	}
	stdout.Reset()
	if code := executeAt([]string{"-c", path, "agent", "report", "get", "--project", "3", "--id", "9"}, &stdout, &stderr, time.Now()); code != 0 || !strings.Contains(stdout.String(), `"ok": true`) {
		t.Fatal(stdout.String(), stderr.String())
	}
}

func TestQueryOverWebsocket(t *testing.T) {
	home := t.TempDir()
	setHome(t, home)
	fe := newFakeTE(t)
	path := writeCfg(t, home, fe.url)
	now := time.Date(2026, 10, 14, 15, 4, 5, 0, time.Local)
	var stdout, stderr bytes.Buffer
	code := executeAt([]string{"-c", path, "query", "event", "--project", "3"}, &stdout, &stderr, now)
	if code != 0 {
		t.Fatal(code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), "login") || !strings.Contains(stdout.String(), "3") {
		t.Fatal(stdout.String())
	}
	if !strings.Contains(stderr.String(), "50 running") {
		t.Fatal(stderr.String())
	}
	if !strings.Contains(fe.qp, `"eventName":"anyEvent"`) || !strings.Contains(fe.qp, "2026-10-07 00:00:00") {
		t.Fatal(fe.qp)
	}
	if fe.killed {
		t.Fatal("unexpected kill")
	}
}

func TestQueryTimeoutSendsKill(t *testing.T) {
	killed := make(chan struct{}, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hj, ok := w.(http.Hijacker)
		if !ok {
			return
		}
		conn, rw, err := hj.Hijack()
		if err != nil {
			return
		}
		defer conn.Close()
		key := r.Header.Get("Sec-WebSocket-Key")
		fmt.Fprintf(rw, "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: %s\r\n\r\n", wsAccept(key))
		rw.Flush()
		if _, _, err := readFrame(rw.Reader); err != nil {
			return
		}
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		if _, payload, err := readFrame(rw.Reader); err == nil && strings.Contains(string(payload), "KILL") {
			killed <- struct{}{}
		}
	}))
	defer srv.Close()
	_, err := runQuery(queryIn{
		Host:       srv.URL,
		Token:      "tok",
		Prefix:     "WS_EVENT",
		ProjectID:  3,
		EventModel: 0,
		QP:         json.RawMessage(`{}`),
		Timeout:    200 * time.Millisecond,
	})
	if err == nil || err.Error() != "query timeout 200ms" {
		t.Fatal(err)
	}
	select {
	case <-killed:
	case <-time.After(2 * time.Second):
		t.Fatal("kill frame was not sent")
	}
}

func TestQueryRejectsSQLAfterLogin(t *testing.T) {
	home := t.TempDir()
	setHome(t, home)
	fe := newFakeTE(t)
	path := writeCfg(t, home, fe.url)
	qp := filepath.Join(home, "qp.json")
	os.WriteFile(qp, []byte(`{"events":[]}`), 0o644)
	var stdout, stderr bytes.Buffer
	code := executeAt([]string{"-c", path, "query", "run", "--project", "3", "--model", "10", "--qp", qp}, &stdout, &stderr, time.Now())
	if code != 1 || !strings.Contains(stderr.String(), "not a read-only statistic") {
		t.Fatal(code, stderr.String())
	}
	if fe.wsCount != 0 {
		t.Fatal("sql model opened websocket")
	}
}

func TestXDGAndExplicitConfig(t *testing.T) {
	home := t.TempDir()
	setHome(t, home)
	xdg := filepath.Join(home, "xdg")
	t.Setenv("XDG_CONFIG_HOME", xdg)
	dir := filepath.Join(xdg, "td-cli")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := []byte(`{"defaultProfile":"local","profiles":[{"name":"local","url":"http://127.0.0.1:9","username":"from-xdg","password":"secret"}]}`)
	if err := os.WriteFile(filepath.Join(dir, "config.json"), body, 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := executeAt([]string{"auth", "status"}, &stdout, &stderr, time.Now()); code != 0 || !strings.Contains(stdout.String(), "from-xdg") {
		t.Fatal(code, stdout.String(), stderr.String())
	}
	other := filepath.Join(home, "other.json")
	os.WriteFile(other, []byte(`{"defaultProfile":"local","profiles":[{"name":"local","url":"http://127.0.0.1:9","username":"from-flag","password":"secret"}]}`), 0o644)
	stdout.Reset()
	if code := executeAt([]string{"-c", other, "auth", "status"}, &stdout, &stderr, time.Now()); code != 0 || !strings.Contains(stdout.String(), "from-flag") {
		t.Fatal(stdout.String(), stderr.String())
	}
}

type fakeTE struct {
	url           string
	sawLogin      bool
	plainPassword bool
	sawStamp      bool
	qp            string
	killed        bool
	wsCount       int
}

func newFakeTE(t *testing.T) *fakeTE {
	t.Helper()
	fe := &fakeTE{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fe.serve(t, w, r)
	}))
	t.Cleanup(srv.Close)
	fe.url = srv.URL
	return fe
}

func (fe *fakeTE) serve(t *testing.T, w http.ResponseWriter, r *http.Request) {
	t.Helper()
	if strings.HasPrefix(r.URL.Path, "/v1/ta-websocket/") {
		fe.wsCount++
		fe.serveWS(t, w, r)
		return
	}
	if r.URL.Path != "/v1/oauth/loginForToken" {
		if r.Header.Get("Authorization") != "bearer tok" {
			http.Error(w, "no", http.StatusUnauthorized)
			return
		}
	}
	switch r.URL.Path {
	case "/v1/oauth/loginForToken":
		fe.serveLogin(w, r)
	case "/v1/oauth/check":
		writeJSON(w, `{"return_code":0,"data":{"projectInfoList":[{"projectId":3,"projectName":"demo"}]}}`)
	case "/v1/ta/event/catalog/listEvent":
		if r.URL.Query().Get("@t") != "" {
			fe.sawStamp = true
		}
		writeJSON(w, `{"return_code":0,"data":{"eventCatalog":[{"events":[{"eventName":"login","eventDesc":"登录"}]}]}}`)
	case "/v1/ta/event/model/meta/loadFiltProps":
		writeJSON(w, `{"return_code":0,"data":{"props":[{"quot":"city","quotDesc":"城市","tableType":"0"}]}}`)
	case "/v1/ta/event/reportsearch":
		writeJSON(w, `{"return_code":0,"data":{"reportModel":0,"reportId":9,"events":[{"analysis":"A100"}],"eventView":{"graphShape":"L0"}}}`)
	default:
		http.NotFound(w, r)
	}
}

func (fe *fakeTE) serveLogin(w http.ResponseWriter, r *http.Request) {
	buf, _ := io.ReadAll(r.Body)
	vals, _ := url.ParseQuery(string(buf))
	fe.sawLogin = true
	enc := vals.Get("enPassword")
	if enc == "" || enc == "secret" || strings.Contains(enc, "secret") {
		fe.plainPassword = true
	}
	writeJSON(w, `{"return_code":0,"data":{"tokenInfo":{"accessToken":"tok","loginName":"ray"},"oAuthLoginInfo":{"refreshToken":"r"}}}`)
}

func (fe *fakeTE) serveWS(t *testing.T, w http.ResponseWriter, r *http.Request) {
	t.Helper()
	hj, ok := w.(http.Hijacker)
	if !ok {
		t.Fatal("no hijack")
	}
	conn, rw, err := hj.Hijack()
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	key := r.Header.Get("Sec-WebSocket-Key")
	fmt.Fprintf(rw, "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: %s\r\n\r\n", wsAccept(key))
	if err := rw.Flush(); err != nil {
		t.Fatal(err)
	}
	_, payload, err := readFrame(rw.Reader)
	if err != nil {
		t.Fatal(err)
	}
	fe.qp = string(payload)
	var frame []json.RawMessage
	if err := json.Unmarshal(payload, &frame); err != nil || len(frame) < 2 {
		t.Fatal(err)
	}
	var body map[string]json.RawMessage
	if err := json.Unmarshal(frame[1], &body); err != nil {
		t.Fatal(err)
	}
	var id string
	json.Unmarshal(body["requestId"], &id)
	var qp string
	json.Unmarshal(body["qp"], &qp)
	fe.qp = qp
	prog := fmt.Sprintf(`["data",{"requestId":%q,"progress":50,"hintMsg":"running"}]`, id)
	if err := writeServerText(rw, prog); err != nil {
		t.Fatal(err)
	}
	done := fmt.Sprintf(`["data",{"requestId":%q,"result":{"data":{"x":["10-13"],"y":[{"eventName":"login","data":[3]}]}}}]`, id)
	if err := writeServerText(rw, done); err != nil {
		t.Fatal(err)
	}
	rw.Flush()
	_ = conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	if _, next, err := readFrame(rw.Reader); err == nil && strings.Contains(string(next), "KILL") {
		fe.killed = true
	}
}

func writeServerText(w *bufio.ReadWriter, text string) error {
	payload := []byte(text)
	hdr := []byte{0x81, byte(len(payload))}
	if len(payload) >= 126 {
		hdr = []byte{0x81, 126, byte(len(payload) >> 8), byte(len(payload))}
	}
	if _, err := w.Write(hdr); err != nil {
		return err
	}
	if _, err := w.Write(payload); err != nil {
		return err
	}
	return w.Flush()
}

func writeJSON(w http.ResponseWriter, body string) {
	w.Header().Set("Content-Type", "application/json")
	io.WriteString(w, body)
}

func setHome(t *testing.T, home string) {
	t.Helper()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, ".local", "state"))
}

func writeCfg(t *testing.T, home, rawURL string) string {
	t.Helper()
	path := filepath.Join(home, "td.json")
	body := fmt.Sprintf(`{"defaultProfile":"local","profiles":[{"name":"local","url":%q,"username":"ray","password":"secret"}]}`, rawURL)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
