package har

import "testing"

func TestEvalAndAndMissingBody(t *testing.T) {
	ctx := jObj(func() *ordMap {
		m := newMap()
		body := newMap()
		body.put("status", jNum(1))
		m.put("body", jObj(body))
		http := newMap()
		http.put("status", jNum(403))
		m.put("http", jObj(http))
		return m
	}())
	ok, err := Eval("body.status == 1 and http.status == 403", ctx)
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}

	missing := newMap()
	http := newMap()
	http.put("status", jNum(403))
	missing.put("http", jObj(http))
	ok, err = Eval("body.status == 1", jObj(missing))
	if err != nil || ok {
		t.Fatalf("missing ok=%v err=%v", ok, err)
	}
}

func TestEvalOperators(t *testing.T) {
	ctx := jObj(func() *ordMap {
		m := newMap()
		http := newMap()
		http.put("status", jNum(200))
		http.put("method", jStr("GET"))
		m.put("http", jObj(http))
		return m
	}())
	cases := []struct {
		expr string
		want bool
	}{
		{"http.status != 404", true},
		{"http.status > 100 and http.status < 300", true},
		{"not http.status == 500", true},
		{"http.method == 'GET' || http.method == 'POST'", true},
		{"(http.status == 500 or http.status == 200) and http.method == \"GET\"", true},
		{"http.status >= 200 and http.status <= 200", true},
		{"body.status == 1", false},
	}
	for _, tc := range cases {
		got, err := Eval(tc.expr, ctx)
		if err != nil || got != tc.want {
			t.Fatalf("%s got=%v err=%v", tc.expr, got, err)
		}
	}
	if _, err := Eval("http.status ==", ctx); err == nil {
		t.Fatal("expected expression error")
	}
}

func TestJSNumberDepth(t *testing.T) {
	if n, err := parseDepth("2.0", "-p"); err != nil || n != 2 {
		t.Fatalf("2.0 n=%d err=%v", n, err)
	}
	if n, err := parseDepth("0x10", "-p"); err != nil || n != 16 {
		t.Fatalf("hex n=%d err=%v", n, err)
	}
	if _, err := parseDepth("1.5", "-p"); err == nil {
		t.Fatal("expected reject 1.5")
	}
}
