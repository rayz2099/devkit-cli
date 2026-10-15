package dsn

import (
	"strings"
	"testing"
)

func TestParseFileJSONC(t *testing.T) {
	sample := `{
	  "profiles": [
	    {
	      "name": "buy",
	      "kind": "mysql",
	      "url": "mysql://readonly@127.0.0.1:3306/buy",
	    },
	    {
	      "name": "dw",
	      "kind": "doris",
	      "url": "mysql://readonly@127.0.0.1:9030/dw",
	      "access": "write",
	    }
	  ]
	}`
	file, err := Parse([]byte(sample), "/tmp/config.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(file.Profiles) != 2 || len(file.Problems) != 0 {
		t.Fatalf("profiles=%d problems=%d", len(file.Profiles), len(file.Problems))
	}
	if file.Profiles[0].Access != "read" || file.Profiles[1].Access != "write" || file.Profiles[1].Kind != "doris" {
		t.Fatalf("%+v", file.Profiles)
	}
}

func TestParseSkipsBadProfiles(t *testing.T) {
	file, err := Parse([]byte(`{ "profiles": [{ "name": "x", "kind": "mysql", "url": "redis://127.0.0.1:6379/0" }] }`), "/tmp/x.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(file.Profiles) != 0 || len(file.Problems) != 1 {
		t.Fatalf("%+v", file)
	}
	mixed, err := Parse([]byte(`{
	  "profiles": [
	    { "name": "pg-it", "kind": "oracle", "url": "oracle://127.0.0.1/db" },
	    { "name": "kf-biz", "kind": "kafka", "url": "kafka://127.0.0.1:9092" }
	  ]
	}`), "/tmp/x.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(mixed.Profiles) != 1 || mixed.Profiles[0].Name != "kf-biz" {
		t.Fatalf("%+v", mixed.Profiles)
	}
}

func TestParseURLSchemes(t *testing.T) {
	ok, err := Parse([]byte(`{ "profiles": [
	  { "name": "pg", "kind": "postgres", "url": "postgres://readonly@127.0.0.1:5432/app" },
	  { "name": "pg-iana", "kind": "postgres", "url": "postgresql://readonly@127.0.0.1:5432/app" },
	  { "name": "kf", "kind": "kafka", "url": "kafkas://127.0.0.1:9093" },
	  { "name": "tls", "kind": "redis", "url": "rediss://:s3cret@127.0.0.1:6380/0" }
	] }`), "/tmp/ok.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(ok.Profiles) != 4 || len(ok.Problems) != 0 {
		t.Fatalf("profiles=%d problems=%d", len(ok.Profiles), len(ok.Problems))
	}
	bad, err := Parse([]byte(`{ "profiles": [{ "name": "x", "kind": "kafka", "url": "http://127.0.0.1:9092" }] }`), "/tmp/k.json")
	if err != nil || len(bad.Profiles) != 0 {
		t.Fatalf("err=%v profiles=%d", err, len(bad.Profiles))
	}
}

func TestParseDescription(t *testing.T) {
	file, err := Parse([]byte(`{ "profiles": [
	  { "name": "buy", "kind": "mysql", "url": "mysql://readonly@127.0.0.1:3306/buy", "description": "交易库只读" },
	  { "name": "dw", "kind": "doris", "url": "mysql://readonly@127.0.0.1:9030/dw", "description": "  " }
	] }`), "/tmp/desc.json")
	if err != nil {
		t.Fatal(err)
	}
	if file.Profiles[0].Description != "交易库只读" || file.Profiles[1].Description != "" {
		t.Fatalf("%+v", file.Profiles)
	}
	skipped, err := Parse([]byte(`{ "profiles": [
	  { "name": "buy", "kind": "mysql", "url": "mysql://readonly@127.0.0.1:3306/buy", "description": 1 },
	  { "name": "kf-biz", "kind": "kafka", "url": "kafka://127.0.0.1:9092" }
	] }`), "/tmp/desc.json")
	if err != nil || len(skipped.Profiles) != 1 || skipped.Profiles[0].Name != "kf-biz" {
		t.Fatalf("%+v %v", skipped, err)
	}
}

func TestMongoSeedList(t *testing.T) {
	file, err := Parse([]byte(`{ "profiles": [
	  { "name": "rs-biz", "kind": "mongodb",
	    "url": "mongodb://biz:secret@10.0.16.60:27017,10.0.16.61:27017,10.0.16.62:27017?readPreference=secondary" },
	  { "name": "rs-meta", "kind": "mongodb",
	    "url": "mongodb://biz_read:secret@10.0.16.70:27017,10.0.16.71:27017,10.0.16.72:27017?authSource=admin&readPreference=secondary" }
	] }`), "/tmp/mongo.json")
	if err != nil || len(file.Profiles) != 2 {
		t.Fatalf("%v %+v", err, file)
	}
	bad, err := Parse([]byte(`{ "profiles": [{ "name": "x", "kind": "mongodb", "url": "redis://127.0.0.1:6379/0" }] }`), "/tmp/mongo.json")
	if err != nil || len(bad.Profiles) != 0 {
		t.Fatal(bad)
	}
	db, err := mongoDBName("mongodb://biz:secret@10.0.16.60:27017,10.0.16.61:27017,10.0.16.62:27017?readPreference=secondary")
	if err != nil || db != "" {
		t.Fatalf("db=%q err=%v", db, err)
	}
	db, err = mongoDBName("mongodb://u:p@10.0.16.70:27017,10.0.16.71:27017/app?authSource=admin")
	if err != nil || db != "app" {
		t.Fatalf("db=%q err=%v", db, err)
	}
	db, err = mongoDBName("mongodb+srv://app:secret@cluster.mongodb.net/app")
	if err != nil || db != "app" {
		t.Fatalf("db=%q err=%v", db, err)
	}
}

func TestDSOutHidesURL(t *testing.T) {
	file, err := Parse([]byte(`{ "profiles": [{ "name": "buy", "kind": "mysql", "url": "mysql://secret@127.0.0.1:3306/buy", "description": "交易库只读" }] }`), "/tmp/desc.json")
	if err != nil {
		t.Fatal(err)
	}
	out := dsOut(file.Profiles)
	if len(out.Columns) != 4 || out.Columns[0] != "name" || out.Columns[3] != "description" {
		t.Fatal(out.Columns)
	}
	if strings.Contains(Render("human", "json", out, false, false), "secret") {
		t.Fatal(Render("human", "json", out, false, false))
	}
}

func TestGate(t *testing.T) {
	if err := gateQuery("mysql", "read", "  /* c */ SELECT 1"); err != nil {
		t.Fatal(err)
	}
	if err := gateQuery("mysql", "read", "-- hi\nSHOW TABLES"); err != nil {
		t.Fatal(err)
	}
	if err := gateQuery("doris", "read", "SWITCH CATALOG hive"); err != nil {
		t.Fatal(err)
	}
	if msg := blocked("mysql", "INSERT INTO t VALUES (1)"); !strings.Contains(msg, "INSERT") {
		t.Fatal(msg)
	}
	if msg := blocked("mysql", "SELECT * FROM t FOR UPDATE"); !strings.Contains(msg, "FOR UPDATE") {
		t.Fatal(msg)
	}
	if msg := blocked("mysql", "SELECT 1 INTO OUTFILE '/tmp/x'"); !strings.Contains(msg, "INTO OUTFILE") {
		t.Fatal(msg)
	}
	if msg := blocked("mysql", "SELECT 1; DROP TABLE t"); !strings.Contains(msg, "multi-statement") {
		t.Fatal(msg)
	}
	if msg := blocked("mysql", "SWITCH CATALOG hive"); !strings.Contains(msg, "SWITCH") {
		t.Fatal(msg)
	}
	if err := gateQuery("postgres", "read", "tables like '%order%'"); err != nil {
		t.Fatal(err)
	}
	if err := gateQuery("postgres", "read", "SELECT $$; not a second stmt $$"); err != nil {
		t.Fatal(err)
	}
	if err := gateQuery("postgres", "read", "EXPLAIN SELECT * FROM analyze"); err != nil {
		t.Fatal(err)
	}
	if msg := blocked("postgres", "SHOW TABLES"); !strings.Contains(msg, "TABLES LIKE") {
		t.Fatal(msg)
	}
	if msg := blocked("postgres", "EXPLAIN ANALYZE SELECT 1"); !strings.Contains(msg, "ANALYZE") {
		t.Fatal(msg)
	}
	if msg := blocked("postgres", "SELECT a, b INTO tmp FROM t"); !strings.Contains(msg, "INTO") {
		t.Fatal(msg)
	}
	if err := gateQuery("mysql", "write", "INSERT INTO t VALUES (1)"); err != nil {
		t.Fatal(err)
	}
	if err := gateQuery("redis", "read", "GET k"); err != nil {
		t.Fatal(err)
	}
	if msg := blocked("redis", "SET k v"); !strings.Contains(msg, "SET") {
		t.Fatal(msg)
	}
	if err := gateQuery("mongodb", "read", `{"find":"orders","filter":{}}`); err != nil {
		t.Fatal(err)
	}
	if msg := blocked("mongodb", `{"insert":"orders","documents":[]}`); !strings.Contains(msg, "insert") {
		t.Fatal(msg)
	}
	if err := gateQuery("elasticsearch", "read", `GET /orders/_search {"query":{"match_all":{}}}`); err != nil {
		t.Fatal(err)
	}
	if msg := blocked("elasticsearch", "POST /orders/_search"); !strings.Contains(msg, "POST") {
		t.Fatal(msg)
	}
	if err := gateQuery("kafka", "read", "peek album_audit_log 50 partition 0"); err != nil {
		t.Fatal(err)
	}
	if msg := blocked("kafka", "peek album_audit_log 501"); !strings.Contains(msg, "1..500") {
		t.Fatal(msg)
	}
	if msg := blocked("kafka", "PEEK album_audit_log"); !strings.Contains(msg, "PEEK") {
		t.Fatal(msg)
	}
}

func blocked(kind, stmt string) string {
	err := gateQuery(kind, "read", stmt)
	if err == nil {
		return "expected reject"
	}
	ex, ok := err.(*Exit)
	if !ok || ex.Code != 2 {
		return err.Error()
	}
	return err.Error()
}

func TestOutput(t *testing.T) {
	data := QueryOut{
		Columns: []string{"id", "name"},
		Rows: []map[string]any{
			{"id": 1, "name": "a"},
			{"id": 2, "name": "b"},
		},
	}
	if agentLimit(nil) != 1000 || agentLimit(intPtr(0)) != 0 {
		t.Fatal("limit")
	}
	rows, trunc := capRows(data.Rows, 1)
	if !trunc || len(rows) != 1 || rows[0]["id"] != 1 {
		t.Fatal(rows, trunc)
	}
	human := Render("human", "json", data, false, false)
	if human != "{\"id\":1,\"name\":\"a\"}\n{\"id\":2,\"name\":\"b\"}\n" {
		t.Fatalf("human=%q", human)
	}
	agent := Render("agent", "csv", data, true, false)
	if !strings.Contains(agent, `"truncated": true`) || !strings.Contains(agent, `"rows"`) {
		t.Fatal(agent)
	}
	pretty := Render("human", "json", data, false, true)
	want := "[\n  {\n    \"id\": 1,\n    \"name\": \"a\"\n  },\n  {\n    \"id\": 2,\n    \"name\": \"b\"\n  }\n]\n"
	if pretty != want {
		t.Fatalf("pretty=%q", pretty)
	}
}

func intPtr(n int) *int { return &n }

func TestKafkaStmt(t *testing.T) {
	got, err := parseKafkaStmt("topics")
	if err != nil || got.Head != "topics" {
		t.Fatal(err, got)
	}
	got, err = parseKafkaStmt("peek t")
	if err != nil || got.N != 50 || got.Partition != nil || got.Topic != "t" {
		t.Fatal(got, err)
	}
	got, err = parseKafkaStmt("peek t 3 partition 1")
	if err != nil || got.N != 3 || got.Partition == nil || *got.Partition != 1 {
		t.Fatal(got, err)
	}
	got, err = parseKafkaStmt("listen t partition 0")
	if err != nil || got.Head != "listen" || got.Partition == nil || *got.Partition != 0 {
		t.Fatal(got, err)
	}
	if _, err = parseKafkaStmt("PEEK t"); err == nil || !strings.Contains(err.Error(), "PEEK") {
		t.Fatal(err)
	}
	if _, err = parseKafkaStmt("peek t 501"); err == nil || !strings.Contains(err.Error(), "1..500") {
		t.Fatal(err)
	}
	if strings.Join(kafkaComplete(nil, []string{"a", "b"}), ",") != "topics,peek,listen" {
		t.Fatal(kafkaComplete(nil, []string{"a", "b"}))
	}
	if strings.Join(kafkaComplete([]string{"peek"}, []string{"a", "b"}), ",") != "a,b" {
		t.Fatal(kafkaComplete([]string{"peek"}, []string{"a", "b"}))
	}
	recs := []kafkaRec{
		{Partition: 0, Offset: "1", Timestamp: 10},
		{Partition: 1, Offset: "9", Timestamp: 50},
		{Partition: 0, Offset: "2", Timestamp: 20},
		{Partition: 1, Offset: "8", Timestamp: 40},
	}
	newest := pickNewest(recs, 2)
	if newest[0].Offset != "8" || newest[1].Offset != "9" {
		t.Fatal(newest)
	}
	if decodeBytes(nil) != nil {
		t.Fatal("nil")
	}
	gotObj := decodeBytes([]byte(`{"a":1}`))
	obj, ok := gotObj.(map[string]any)
	if !ok || obj["a"] != float64(1) {
		t.Fatalf("%#v", gotObj)
	}
	if decodeBytes([]byte("hello")) != "hello" {
		t.Fatal(decodeBytes([]byte("hello")))
	}
	bin := decodeBytes([]byte{0xff, 0xfe, 0x00})
	if bin != "///4A" && bin != "/v4A" {
		// base64 of ff fe 00 is //4A
		if bin != "//4A" {
			t.Fatal(bin)
		}
	}
}

func TestPgCatalogParse(t *testing.T) {
	got, err := parsePgCatalog("tables")
	if err != nil || got.Head != "tables" || got.HasLike {
		t.Fatal(got, err)
	}
	got, err = parsePgCatalog("TABLES LIKE order%")
	if err != nil || got.Like != "order%" {
		t.Fatal(got, err)
	}
	got, err = parsePgCatalog("desc public.orders")
	if err != nil || got.Head != "columns" || got.Rel != "public.orders" {
		t.Fatal(got, err)
	}
	got, err = parsePgCatalog("ddl orders")
	if err != nil || got.Head != "ddl" || got.Rel != "orders" {
		t.Fatal(got, err)
	}
	if pgCatalogHead("SELECT 1") != "" || pgCatalogHead("TABLE orders") != "" {
		t.Fatal("head")
	}
	if _, err = parsePgCatalog("tables foo"); err == nil {
		t.Fatal("tables foo")
	}
	if _, err = parsePgCatalog("columns pay-order"); err == nil {
		t.Fatal("bad name")
	}
	if strings.Join(pgCatalogComplete(nil), ",") != "tables,columns,ddl,desc" {
		t.Fatal(pgCatalogComplete(nil))
	}
	if strings.Join(pgCatalogComplete([]string{"tables"}), ",") != "like" {
		t.Fatal(pgCatalogComplete([]string{"tables"}))
	}
}

func TestConsoleSpec(t *testing.T) {
	spec, err := consoleSpec("doris", "mysql://u:p@fe:9030/dw")
	if err != nil {
		t.Fatal(err)
	}
	if spec.Bin != "mysql" || strings.Join(spec.Args, " ") != "-h fe -P 9030 -u u -pp dw" {
		t.Fatalf("%s %v", spec.Bin, spec.Args)
	}
	spec, err = consoleSpec("postgres", "postgres://u:p@127.0.0.1:5432/app?sslmode=require")
	if err != nil || spec.Bin != "psql" || len(spec.Args) != 1 {
		t.Fatal(spec, err)
	}
	spec, err = consoleSpec("redis", "redis://:s3cret@127.0.0.1:6379/2")
	if err != nil || strings.Join(spec.Args, " ") != "-h 127.0.0.1 -p 6379 -a s3cret -n 2" {
		t.Fatal(spec.Args, err)
	}
	spec, err = consoleSpec("mongodb", "mongodb://127.0.0.1:27017/app")
	if err != nil || spec.Bin != "mongosh" || spec.Args[0] != "mongodb://127.0.0.1:27017/app" {
		t.Fatal(spec, err)
	}
	if _, err = consoleSpec("kafka", "kafka://127.0.0.1:9092"); err == nil || !strings.Contains(err.Error(), "no Console") {
		t.Fatal(err)
	}
	if _, err = consoleSpec("elasticsearch", "http://127.0.0.1:9200"); err == nil {
		t.Fatal("es console")
	}
}

func TestParseArgs(t *testing.T) {
	got, err := parseArgs(nil)
	if err != nil || got.kind != "help" {
		t.Fatal(got, err)
	}
	got, err = parseArgs([]string{"-p", "buy"})
	if err != nil || got.kind != "console" || got.profile != "buy" || got.audience != audHuman {
		t.Fatal(got, err)
	}
	got, err = parseArgs([]string{"-c", "/tmp/dsn.json", "-p", "buy"})
	if err != nil || got.config != "/tmp/dsn.json" || got.kind != "console" {
		t.Fatal(got, err)
	}
	got, err = parseArgs([]string{"-p", "buy", "query", "SELECT", "1"})
	if err != nil || got.stmt != "SELECT 1" || got.exec != 30 || got.connect != 3 || got.output != "table" {
		t.Fatal(got, err)
	}
	if _, err = parseArgs([]string{"query", "SELECT 1"}); err == nil || !strings.Contains(err.Error(), "-p is required") {
		t.Fatal(err)
	}
	got, err = parseArgs([]string{"-p", "kf-biz", "query", "peek", "album_audit_log", "5"})
	if err != nil || got.stmt != "peek album_audit_log 5" {
		t.Fatal(got, err)
	}
	got, err = parseArgs([]string{"agent", "-p", "buy", "query", "PING", "--limit", "10"})
	if err != nil || got.audience != audAgent || got.limit == nil || *got.limit != 10 {
		t.Fatal(got, err)
	}
	got, err = parseArgs([]string{"-p", "buy", "query", "SELECT 1", "--timeout", "120", "--connect-timeout", "2"})
	if err != nil || got.exec != 120 || got.connect != 2 {
		t.Fatal(got, err)
	}
	got, err = parseArgs([]string{"doctor"})
	if err != nil || got.kind != "doctor" || got.hasProf || got.exec != 5 || got.connect != 3 {
		t.Fatal(got, err)
	}
	if _, err = parseArgs([]string{"doctor", "extra"}); err == nil || !strings.Contains(err.Error(), "unexpected argument") {
		t.Fatal(err)
	}
	got, err = parseArgs([]string{"-p", "kf-biz", "query", "peek", "t", "5", "--output", "json", "--pretty"})
	if err != nil || !got.pretty || got.output != "json" {
		t.Fatal(got, err)
	}
	if _, err = parseArgs([]string{"-p", "kf-biz", "query", "peek", "t", "--pretty"}); err == nil {
		t.Fatal("pretty")
	}
	got, err = parseArgs([]string{"-p", "ds-read", "-h", "ds"})
	if err != nil || got.kind != "help" || got.topic != "ds" || got.profile != "ds-read" {
		t.Fatal(got, err)
	}
	got, err = parseArgs([]string{"ds"})
	if err != nil || got.kind != "ds" || got.hasProf {
		t.Fatal(got, err)
	}
	got, err = parseArgs([]string{"version"})
	if err != nil || got.kind != "version" {
		t.Fatal(got, err)
	}
}

func TestCompleteRoot(t *testing.T) {
	root := completeValues(nil, "")
	if !contains(root, "doctor") || !contains(root, "query") {
		t.Fatal(root)
	}
	after := completeValues([]string{"agent"}, "")
	if strings.Join(after, ",") != "query,doctor,ds" {
		t.Fatal(after)
	}
	lines := completeLines([]string{"agent"}, "")
	if lines != "query\ndoctor\nds\n" {
		t.Fatalf("%q", lines)
	}
}

func contains(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
