package nacos

import (
	"testing"
)

func TestResolveRuntimePrecedence(t *testing.T) {
	file, err := parseFile([]byte(`{
		"nacos_server_addr": "file:8848",
		"nacos_namespace": "file-ns",
		"namespaces": ["prepare", "online"],
		"nacos_output": "text"
	}`), "/tmp/config.json")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("nacos_namespace", "env-ns")
	t.Setenv("NACOS_OUTPUT", "json")
	server := "flag:8848"
	rt, err := resolveRuntime(globals{server: &server}, file)
	if err != nil {
		t.Fatal(err)
	}
	if rt.server != "flag:8848" || rt.namespace != "env-ns" || rt.output != "json" {
		t.Fatalf("runtime = %+v", rt)
	}
}

func TestResolveNamespaceFallsBackToList(t *testing.T) {
	file := fileConfig{namespaces: []string{"prepare", "online"}}
	rt, err := resolveRuntime(globals{}, file)
	if err != nil {
		t.Fatal(err)
	}
	if rt.namespace != "prepare" || rt.server != defaultServer {
		t.Fatalf("runtime = %+v", rt)
	}
}

func TestNSCandsDedup(t *testing.T) {
	file, err := parseFile([]byte(`{"namespaces":["prepare","public"]}`), "/tmp/config.json")
	if err != nil {
		t.Fatal(err)
	}
	got := nsCands(file)
	if len(got) != 2 || got[0] != "prepare" || got[1] != "public" {
		t.Fatalf("cands = %#v", got)
	}
}

func TestParseFileRejectsBadNamespaces(t *testing.T) {
	_, err := parseFile([]byte(`{"namespaces":"nope"}`), "/tmp/config.json")
	if err == nil || err.Error() != "invalid config file /tmp/config.json: namespaces must be an array" {
		t.Fatalf("err = %v", err)
	}
	_, err = parseFile([]byte(`{"namespaces":null}`), "/tmp/config.json")
	if err == nil {
		t.Fatal("null namespaces should fail")
	}
}

func TestParseAddress(t *testing.T) {
	ep, err := parseAddress("http://127.0.0.1:8848")
	if err != nil {
		t.Fatal(err)
	}
	if ep.scheme != "http" || ep.host != "127.0.0.1" || ep.port != 8848 {
		t.Fatalf("ep = %+v", ep)
	}
	ep, err = parseAddress("https://127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	if ep.scheme != "https" || ep.host != "127.0.0.1" || ep.port != 8848 {
		t.Fatalf("ep = %+v", ep)
	}
}

func TestRenderTablePadsColumns(t *testing.T) {
	text := renderTable("Total: 1  Page: 1/1", []string{"DATA_ID", "GROUP"}, [][]string{{"d1", "g1"}})
	want := "Total: 1  Page: 1/1\nDATA_ID  GROUP\nd1       g1   "
	if text != want {
		t.Fatalf("table = %q", text)
	}
}

func TestRenderJSONDoesNotEscapeHTML(t *testing.T) {
	got, err := render("json", "", gotView{DataID: "id", Group: "g", Content: "<a>"})
	if err != nil {
		t.Fatal(err)
	}
	if got != "{\n  \"dataId\": \"id\",\n  \"group\": \"g\",\n  \"content\": \"<a>\"\n}\n" {
		t.Fatalf("json = %q", got)
	}
}
