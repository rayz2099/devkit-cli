package nacos

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func isolate(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, ".local", "state"))
	for _, key := range []string{
		"nacos_server_addr", "NACOS_SERVER_ADDR",
		"nacos_username", "NACOS_USERNAME",
		"nacos_password", "NACOS_PASSWORD",
		"nacos_namespace", "NACOS_NAMESPACE",
		"nacos_output", "NACOS_OUTPUT",
	} {
		t.Setenv(key, "unset-me")
		if err := os.Unsetenv(key); err != nil {
			t.Fatal(err)
		}
	}
	return home
}

func runCLI(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	err := Execute(args, &stdout, &stderr)
	return stdout.String(), stderr.String(), err
}

func writeCfg(t *testing.T, home string, body string) {
	t.Helper()
	dir := filepath.Join(home, ".config", "nacos-cli")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestHelpVersionAndUnknown(t *testing.T) {
	isolate(t)
	stdout, stderr, err := runCLI(t)
	if err != nil || stderr != "" || !strings.HasSuffix(stdout, "\n\n") || !strings.Contains(stdout, "nacos-cli completion fish\n") {
		t.Fatalf("help err=%v stderr=%q stdout=%q", err, stderr, stdout)
	}
	stdout, stderr, err = runCLI(t, "config")
	if err != nil || !strings.Contains(stdout, "nacos-cli config get [data-id] [group]\n") {
		t.Fatalf("config help = %q err=%v", stdout, err)
	}
	stdout, stderr, err = runCLI(t, "version", "-c", filepath.Join(t.TempDir(), "missing.json"))
	if err != nil || stderr != "" || stdout != "nacos-cli 0.1.0\n" {
		t.Fatalf("version stdout=%q stderr=%q err=%v", stdout, stderr, err)
	}
	_, stderr, err = runCLI(t, "nope")
	if err == nil || err.Error() != "unknown command: nope" || stderr != "" {
		t.Fatalf("unknown err=%v stderr=%q", err, stderr)
	}
	_, _, err = runCLI(t, "config", "foo")
	if err == nil || err.Error() != "unknown config command: foo" {
		t.Fatalf("config err=%v", err)
	}
	_, _, err = runCLI(t, "completion")
	if err == nil || err.Error() != "usage: nacos-cli completion fish" {
		t.Fatalf("completion err=%v", err)
	}
	stdout, _, err = runCLI(t, "completion", "fish")
	if err != nil || stdout != fishScript() {
		t.Fatalf("fish script mismatch err=%v", err)
	}
}

func TestDoctor(t *testing.T) {
	home := isolate(t)
	stdout, stderr, err := runCLI(t, "doctor")
	if err == nil || !Quiet(err) || stderr != "" || !strings.Contains(stdout, "fail config: config missing:") {
		t.Fatalf("missing doctor stdout=%q stderr=%q err=%v", stdout, stderr, err)
	}
	if _, statErr := os.Stat(filepath.Join(home, ".config", "nacos-cli")); !os.IsNotExist(statErr) {
		t.Fatalf("doctor created config dir: %v", statErr)
	}

	writeCfg(t, home, `{"nacos_server_addr":"127.0.0.1:8848","nacos_output":"text"}`)
	stdout, _, err = runCLI(t, "doctor")
	if err != nil || stdout != "pass config\npass fields\n" {
		t.Fatalf("pass doctor stdout=%q err=%v", stdout, err)
	}

	writeCfg(t, home, `{"nacos_output":"xml"}`)
	stdout, _, err = runCLI(t, "doctor")
	if err == nil || !Quiet(err) || stdout != "pass config\nfail fields: invalid output: xml\n" {
		t.Fatalf("bad output stdout=%q err=%v", stdout, err)
	}
}

func TestConfigGetAndList(t *testing.T) {
	home := isolate(t)
	var got []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = append(got, r.URL.Query().Get("tenant")+" "+r.URL.RequestURI())
		if r.URL.Path != "/nacos/v1/cs/configs" {
			http.NotFound(w, r)
			return
		}
		if r.Method == http.MethodGet && r.URL.Query().Get("dataId") == "app-rpc" {
			_, _ = w.Write([]byte("ok"))
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"totalCount":     1,
			"pageNumber":     1,
			"pagesAvailable": 1,
			"pageItems":      []map[string]string{{"dataId": "d1", "group": "g1"}},
		})
	}))
	defer srv.Close()

	stdout, stderr, err := runCLI(t, "--server-addr", srv.URL, "config", "get", "app-rpc")
	if err != nil || stderr != "" || stdout != "ok\n" {
		t.Fatalf("get stdout=%q stderr=%q err=%v", stdout, stderr, err)
	}
	if !strings.Contains(got[0], "dataId=app-rpc") || !strings.Contains(got[0], "group=COMMON") || !strings.HasPrefix(got[0], "public ") {
		t.Fatalf("query = %s", got[0])
	}
	stdout, _, err = runCLI(t, "-ns", "runtime", "config", "get", "--server-addr", srv.URL, "app-rpc")
	if err != nil || stdout != "ok\n" || !strings.HasPrefix(got[len(got)-1], "runtime ") {
		t.Fatalf("ns get = %q last=%s err=%v", stdout, got[len(got)-1], err)
	}
	if _, statErr := os.Stat(filepath.Join(home, ".config", "nacos-cli")); !os.IsNotExist(statErr) {
		t.Fatalf("get created config dir: %v", statErr)
	}

	stdout, _, err = runCLI(t, "-o", "json", "--server-addr", srv.URL, "config", "get", "app-rpc")
	if err != nil || !strings.Contains(stdout, "\"content\": \"ok\"") {
		t.Fatalf("json get = %q err=%v", stdout, err)
	}

	stdout, _, err = runCLI(t, "--server-addr", srv.URL, "config", "list")
	if err != nil || stdout != "Total: 1  Page: 1/1\nDATA_ID  GROUP\nd1       g1   \n" {
		t.Fatalf("list = %q err=%v", stdout, err)
	}
}

func TestNamespaceShortFlagAndLogin(t *testing.T) {
	isolate(t)
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		if strings.HasSuffix(r.URL.Path, "/login") {
			_, _ = w.Write([]byte(`{"accessToken":"tok"}`))
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if r.URL.Query().Get("accessToken") != "tok" || r.PostForm.Get("tenant") != "runtime" || r.PostForm.Get("dataId") != "d1" {
			http.Error(w, "bad publish", http.StatusForbidden)
			return
		}
		_, _ = w.Write([]byte("true"))
	}))
	defer srv.Close()

	stdout, _, err := runCLI(t, "-ns", "runtime", "--username", "nacos", "--password", "nacos", "--server-addr", srv.URL, "config", "put", "--data-id", "d1", "--group", "g1", "--content", "v1")
	if err != nil || stdout != "ok\n" {
		t.Fatalf("put stdout=%q err=%v", stdout, err)
	}
	if len(paths) != 2 || paths[0] != "/nacos/v1/auth/login" || paths[1] != "/nacos/v1/cs/configs" {
		t.Fatalf("paths = %#v", paths)
	}
}

func TestNamingInstancesAndRegister(t *testing.T) {
	isolate(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/nacos/v1/ns/instance/list":
			if r.URL.Query().Get("healthyOnly") != "true" || r.URL.Query().Get("serviceName") != "svc1" {
				http.Error(w, "bad query", http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte(`{"hosts":[{"ip":"1.1.1.1","port":8080}]}`))
		case "/nacos/v1/ns/instance":
			q := r.URL.Query()
			if q.Get("weight") != "1" || q.Get("ephemeral") != "true" || q.Get("clusterName") != "DEFAULT" || q.Get("groupName") != "DEFAULT_GROUP" {
				http.Error(w, "bad register", http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte("ok"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	stdout, _, err := runCLI(t, "--server-addr", srv.URL, "naming", "instances", "--service", "svc1")
	if err != nil || stdout != "count: 1\n1.1.1.1:8080\n" {
		t.Fatalf("instances = %q err=%v", stdout, err)
	}
	stdout, _, err = runCLI(t, "--server-addr", srv.URL, "naming", "register", "--service", "svc1", "--ip", "1.1.1.1", "--port", "8080")
	if err != nil || stdout != "ok\n" {
		t.Fatalf("register = %q err=%v", stdout, err)
	}
	_, _, err = runCLI(t, "naming", "register", "--service", "svc", "--ip", "1.1.1.1")
	if err == nil || err.Error() != "port is required" {
		t.Fatalf("port err=%v", err)
	}
	_, _, err = runCLI(t, "config", "list", "--search", "fuzzy")
	if err == nil || err.Error() != "search must be one of: accurate, blur" {
		t.Fatalf("search err=%v", err)
	}
}

func TestCompleteProtocol(t *testing.T) {
	home := isolate(t)
	writeCfg(t, home, `{"namespaces":["prepare_hwc","online_hwc","public"]}`)
	stdout, stderr, err := runCLI(t, "__complete", "config", "get", "--to-complete", "")
	if err != nil || stderr != "" || stdout != "-ns\n:4\n" {
		t.Fatalf("ns complete = %q stderr=%q err=%v", stdout, stderr, err)
	}

	stdout, _, err = runCLI(t, "__complete", "--namespace", "--to-complete", "pre")
	if err != nil || stdout != "prepare_hwc\n:4\n" {
		t.Fatalf("namespace values = %q err=%v", stdout, err)
	}

	var calls int
	var tenants []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		tenants = append(tenants, r.URL.Query().Get("tenant"))
		if r.URL.Query().Get("pageSize") != "1000" {
			http.Error(w, "bad list", http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`{"pageItems":[{"dataId":"app-rpc","group":"COMMON"},{"dataId":"app-infra","group":"COMMON"},{"dataId":"biz-kv","group":"COMMON"}]}`))
	}))
	defer srv.Close()

	stdout, _, err = runCLI(t, "__complete", "--server-addr", srv.URL, "config", "get", "-ns", "online_hwc", "--to-complete", "app")
	if err != nil || stdout != "app-rpc\napp-infra\n:4\n" || len(tenants) != 1 || tenants[0] != "online_hwc" {
		t.Fatalf("dataId = %q tenants=%v err=%v", stdout, tenants, err)
	}
	stdout, _, err = runCLI(t, "__complete", "--server-addr", srv.URL, "config", "get", "-ns", "online_hwc", "app")
	if err != nil || stdout != "app-rpc\napp-infra\n:4\n" || calls != 1 {
		t.Fatalf("cached dataId = %q calls=%d err=%v", stdout, calls, err)
	}
	cache := filepath.Join(home, ".cache", "nacos-cli", "config-list.json")
	if _, statErr := os.Stat(cache); statErr != nil {
		t.Fatalf("cache missing: %v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(home, ".config", "nacos-cli", "cache")); !os.IsNotExist(statErr) {
		t.Fatal("cache written under config dir")
	}

	stdout, _, err = runCLI(t, "__complete", "--server-addr", srv.URL, "config", "get", "app-rpc", "--to-complete", "CO")
	if err != nil || stdout != "COMMON\n:4\n" {
		t.Fatalf("group = %q err=%v", stdout, err)
	}
}

func TestDevLogStaysOutOfConfig(t *testing.T) {
	home := isolate(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()
	if _, _, err := runCLI(t, "--dev", "--server-addr", srv.URL, "config", "get", "app"); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(home, ".local", "state", "nacos-cli", "log", "nacos-cli.log")
	buf, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(buf), "GET /nacos/v1/cs/configs 200 ") {
		t.Fatalf("log = %s", buf)
	}
	if _, statErr := os.Stat(filepath.Join(home, ".config", "nacos-cli")); !os.IsNotExist(statErr) {
		t.Fatal("dev log created config dir")
	}
}

func TestRefuseRuntimeInsideConfig(t *testing.T) {
	home := isolate(t)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".config", "cache"))
	_, _, err := runCLI(t, "__fish_complete", "config-data-ids", "--prefix", "a")
	if err == nil || !strings.Contains(err.Error(), "refusing path inside config dir") {
		t.Fatalf("err = %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(home, ".config", "cache")); !os.IsNotExist(statErr) {
		t.Fatal("refused path was still created")
	}
}

func TestXDGConfigHome(t *testing.T) {
	home := isolate(t)
	cfgRoot := filepath.Join(home, "xdg")
	t.Setenv("XDG_CONFIG_HOME", cfgRoot)
	if err := os.MkdirAll(filepath.Join(cfgRoot, "nacos-cli"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"nacos_namespace":"from-xdg","namespaces":["from-xdg"]}`
	if err := os.WriteFile(filepath.Join(cfgRoot, "nacos-cli", "config.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, _, err := runCLI(t, "__fish_complete", "namespaces", "--prefix", "from")
	if err != nil || stdout != "from-xdg\n" {
		t.Fatalf("xdg ns = %q err=%v", stdout, err)
	}
	_, _, err = runCLI(t, "-c", filepath.Join(home, "missing.json"), "config", "get", "app")
	if err == nil || !strings.Contains(err.Error(), "config file not found:") {
		t.Fatalf("missing -c err=%v", err)
	}
}
