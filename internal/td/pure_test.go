package td

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

func TestParseDefaultsAndRejectsWrite(t *testing.T) {
	human, err := parseArgs([]string{"project", "ls"})
	if err != nil || human.Kind != kindProjects || human.Audience != "human" {
		t.Fatalf("human = %#v %v", human, err)
	}
	agent, err := parseArgs([]string{"agent", "project", "ls"})
	if err != nil || agent.Kind != kindProjects || agent.Audience != "agent" {
		t.Fatalf("agent = %#v %v", agent, err)
	}
	if _, err := parseArgs([]string{"report", "save"}); err == nil || !strings.Contains(err.Error(), "read-only") {
		t.Fatal(err)
	}
	if _, err := parseArgs([]string{"query", "download"}); err == nil || !strings.Contains(err.Error(), "read-only") {
		t.Fatal(err)
	}
	got, err := parseArgs([]string{"query", "event", "--project", "3"})
	if err != nil {
		t.Fatal(err)
	}
	if got.EventName != "anyEvent" || got.Analysis != "A100" || got.Particle != "T1" || got.Timeout != 120*time.Second {
		t.Fatalf("%#v", got)
	}
	if _, err := parseArgs([]string{"query", "event", "--project", "1", "--qp", "a.json", "--event", "login"}); err == nil || err.Error() != "--qp replaces other query flags" {
		t.Fatal(err)
	}
	prof, err := parseArgs([]string{"-p", "local", "auth", "status"})
	if err != nil || prof.Kind != kindStatus || prof.Profile != "local" {
		t.Fatalf("%#v %v", prof, err)
	}
}

func TestConfigJSONC(t *testing.T) {
	raw := []byte(`{
	  "defaultProfile": "local",
	  "profiles": [
	    {
	      "name": "local",
	      "url": "http://10.0.0.1:8993",
	      "username": "ray",
	      "password": "secret",
	    }
	  ]
	}`)
	file, err := parseFileCfg(raw, "config.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(file.Profiles) != 1 || file.Profiles[0].URL != "http://10.0.0.1:8993" || file.Profiles[0].Username != "ray" {
		t.Fatalf("%#v", file)
	}
	broken := []byte(strings.Replace(string(raw), `"url": "http://10.0.0.1:8993",`, "", 1))
	if _, err := parseFileCfg(broken, "config.json"); err == nil || !strings.Contains(err.Error(), "url is required") {
		t.Fatal(err)
	}
	noUser := []byte(strings.Replace(string(raw), `"username": "ray",`, "", 1))
	if _, err := parseFileCfg(noUser, "config.json"); err == nil || !strings.Contains(err.Error(), "username is required") {
		t.Fatal(err)
	}
}

func TestOutputShapes(t *testing.T) {
	text, err := emitValue("agent", map[string]int{"n": 1}, "ignored")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, `"ok": true`) || !strings.Contains(text, `"n": 1`) {
		t.Fatal(text)
	}
	body := errBody("unauthorized")
	if !strings.Contains(body, `"type": "auth"`) || !strings.Contains(body, `"unauthorized"`) {
		t.Fatal(body)
	}
	if got := errBody("未登录"); !strings.Contains(got, `"type": "auth"`) || strings.Contains(got, `\u`) {
		t.Fatal(got)
	}
	human, err := emitRaw("human", []byte(`{}`), "ID  NAME\n")
	if err != nil || human != "ID  NAME\n" {
		t.Fatal(human, err)
	}
	rows, err := eventRows([]byte(`{"eventCatalog":[{"events":[{"eventName":"login","eventDesc":"登录"}]}]}`))
	if err != nil || len(rows) != 1 || rows[0][0] != "login" || rows[0][1] != "登录" {
		t.Fatal(rows, err)
	}
	q, err := queryText([]byte(`{"data":{"x":["10-13"],"y":[{"eventName":"login","data":[3]}]}}`))
	if err != nil || !strings.Contains(q, "login") || !strings.Contains(q, "3") {
		t.Fatal(q, err)
	}
}

func TestQueryModels(t *testing.T) {
	raw, err := propertyQP(propIn{Analysis: "A101", Quota: ""})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"analysis":"A101"`) || !strings.Contains(string(raw), `"tableType":"1"`) {
		t.Fatal(string(raw))
	}
	ev, err := eventQP(eventIn{
		EventName: "login",
		Analysis:  "A101",
		Particle:  "T1",
		Span:      Span{RecentDay: "1-1", StartTime: "2026-10-13 00:00:00", EndTime: "2026-10-13 23:59:59"},
	})
	if err != nil {
		t.Fatal(err)
	}
	text := string(ev)
	for _, piece := range []string{`"eventName":"login"`, `"analysisDesc":"触发用户数"`, `"timeParticleSize":"T1"`, `"startTime":"2026-10-13 00:00:00"`} {
		if !strings.Contains(text, piece) {
			t.Fatal(text)
		}
	}
	if err := assertReadModel(10); err == nil || !strings.Contains(err.Error(), "not a read-only statistic") {
		t.Fatal(err)
	}
	if err := assertReadModel(0); err != nil {
		t.Fatal(err)
	}
	model, qp, err := takeReport([]byte(`{"reportModel":3,"events":[{"analysis":"A101"}],"eventView":{"graphShape":"L1"}}`), nil)
	if err != nil || model != 3 || !strings.Contains(string(qp), `"analysis":"A101"`) {
		t.Fatal(model, string(qp), err)
	}
	if _, _, err := takeReport([]byte(`{"reportModel":0,"qp":{"events":[]}}`), &[]int{3}[0]); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatal(err)
	}
}

func TestRecentDays(t *testing.T) {
	now := time.Date(2026, 10, 14, 15, 4, 5, 0, time.Local)
	got, err := resolveRecent("1-7", now)
	if err != nil {
		t.Fatal(err)
	}
	if got.StartTime != "2026-10-07 00:00:00" || got.EndTime != "2026-10-13 23:59:59" || got.RecentDay != "1-7" {
		t.Fatal(got)
	}
	week, err := resolveRecent("0-7", now)
	if err != nil || week.StartTime != "2026-10-08 00:00:00" || week.EndTime != "2026-10-14 23:59:59" {
		t.Fatal(week, err)
	}
	def, err := resolveSpan(nil, nil, nil, true, now)
	if err != nil || def == nil || def.RecentDay != "1-7" {
		t.Fatal(def, err)
	}
	none, err := resolveSpan(nil, nil, nil, false, now)
	if err != nil || none != nil {
		t.Fatal(none, err)
	}
	from := "2026-10-01 00:00:00"
	if _, err := resolveSpan(nil, &from, nil, false, now); err == nil || err.Error() != "--from and --to are both required" {
		t.Fatal(err)
	}
}

func TestHTTPAndRSA(t *testing.T) {
	got, err := unwrap(200, []byte(`{"return_code":0,"data":{"n":1}}`))
	if err != nil || string(got) != `{"n":1}` {
		t.Fatal(string(got), err)
	}
	if _, err := unwrap(200, []byte(`{"return_code":-1,"return_message":"bad qp"}`)); err == nil || err.Error() != "bad qp" {
		t.Fatal(err)
	}
	if _, err := unwrap(401, []byte(`{"status":401,"error":"Unauthorized"}`)); err == nil || err.Error() != "unauthorized" {
		t.Fatal(err)
	}
	enc, err := encryptPwd("secret")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(enc, "secret") || len(enc) < 20 {
		t.Fatal(enc)
	}
	buf, err := base64.StdEncoding.DecodeString(enc)
	if err != nil || len(buf) != 128 {
		t.Fatalf("len %d %v", len(buf), err)
	}
}

func TestFrames(t *testing.T) {
	frame, err := buildFrame(queryPayload{RequestID: "WS_PROPERTY@@1", ProjectID: 3, EventModel: 3, QP: "{}", SearchSource: "model_search", QuerySource: "module"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(frame, `"requestId":"WS_PROPERTY@@1"`) || !strings.Contains(frame, `"channel":"ta"`) {
		t.Fatal(frame)
	}
	kill, err := killFrame("WS_PROPERTY@@1")
	if err != nil || strings.Contains(kill, "channel") || !strings.Contains(kill, `"queryStatus":"KILL"`) {
		t.Fatal(kill, err)
	}
	done := parseWS(`["data",{"requestId":"a","progress":100,"result":{"x":[]}},{"channel":"ta"}]`)
	if done.Kind != wsDone || done.ID != "a" || string(done.Result) != `{"x":[]}` {
		t.Fatalf("%#v", done)
	}
	failed := parseWS(`["data",{"status":"failed","errorMsg":"未登录"}]`)
	if failed.Kind != wsFailed || failed.ID != "" || failed.Message != "未登录" {
		t.Fatalf("%#v", failed)
	}
	if parseWS(`["heartbeat",""]`).Kind != wsHeartbeat {
		t.Fatal("heartbeat")
	}
	plain, err := wsURL("http://10.0.0.1:8993", "tok", false)
	if err != nil || plain != "ws://10.0.0.1:8993/v1/ta-websocket/query/tok" {
		t.Fatal(plain, err)
	}
	bff, err := wsURL("https://te.example", "a b", true)
	if err != nil || bff != "wss://te.example/bff/v1/ta-websocket/query?token=a%20b" {
		t.Fatal(bff, err)
	}
}

func TestCompleteProtocol(t *testing.T) {
	root := completeLines(nil, "")
	if root != "auth\nproject\nmeta\nreport\nquery\ncompletion\nagent\nhuman\nhelp\n" {
		t.Fatal(root)
	}
	if got := completeLines(nil, "au"); got != "auth\n" {
		t.Fatal(got)
	}
	if got := completeLines([]string{"auth"}, ""); got != "login\nstatus\n" {
		t.Fatal(got)
	}
	if got := completeLines([]string{"-p"}, ""); got != "" {
		t.Fatal(got)
	}
	if fishScript() == "" || !strings.Contains(fishScript(), "__complete") {
		t.Fatal("fish")
	}
}
