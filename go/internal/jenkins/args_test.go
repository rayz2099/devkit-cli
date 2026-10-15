package jenkins

import "testing"

func TestParseAudienceAndProfile(t *testing.T) {
	human, err := parseArgs([]string{"run", "ls", "dt-vrec/feature/x"})
	if err != nil {
		t.Fatal(err)
	}
	if human.kind != "run-ls" || human.audience != "human" || human.job != "dt-vrec/feature/x" {
		t.Fatalf("human = %+v", human)
	}
	agent, err := parseArgs([]string{"agent", "trigger", "dt-vrec/feature/x"})
	if err != nil {
		t.Fatal(err)
	}
	if agent.kind != "run-start" || agent.audience != "agent" || agent.job != "dt-vrec/feature/x" {
		t.Fatalf("agent = %+v", agent)
	}
}

func TestHumanHasNoOldVerbs(t *testing.T) {
	_, err := parseArgs([]string{"trigger", "dt-vrec/feature/x"})
	if err == nil || err.Error() != "unknown command: trigger" {
		t.Fatalf("err = %v", err)
	}
}

func TestStatusDefaultsLastBuild(t *testing.T) {
	got, err := parseArgs([]string{"agent", "status", "dt-vrec/feature/x"})
	if err != nil {
		t.Fatal(err)
	}
	if got.kind != "run-view" || !got.slim || got.build != "lastBuild" {
		t.Fatalf("status = %+v", got)
	}
}

func TestProfileAndConfigFlags(t *testing.T) {
	got, err := parseArgs([]string{"-p", "prod", "job", "ls"})
	if err != nil {
		t.Fatal(err)
	}
	if got.kind != "job-ls" || got.profile != "prod" || got.folder != "" {
		t.Fatalf("profile = %+v", got)
	}
	got, err = parseArgs([]string{"-c", "/tmp/jenkins.json", "job", "ls"})
	if err != nil {
		t.Fatal(err)
	}
	if got.config != "/tmp/jenkins.json" {
		t.Fatalf("config = %q", got.config)
	}
	got, err = parseArgs([]string{"job", "ls", "--config", "/tmp/jenkins.json"})
	if err != nil {
		t.Fatal(err)
	}
	if got.kind != "job-ls" || got.config != "/tmp/jenkins.json" {
		t.Fatalf("late config = %+v", got)
	}
}

func TestFlagErrors(t *testing.T) {
	_, err := parseArgs([]string{"job", "ls", "--limit=2"})
	if err == nil || err.Error() != "unknown flag: --limit=2" {
		t.Fatalf("equals = %v", err)
	}
	_, err = parseArgs([]string{"-p"})
	if err == nil || err.Error() != "-p requires a value" {
		t.Fatalf("missing = %v", err)
	}
	_, err = parseArgs([]string{"run", "ls", "a/b", "--limit", "0"})
	if err == nil || err.Error() != "invalid number: 0" {
		t.Fatalf("zero = %v", err)
	}
	got, err := parseArgs([]string{"run", "ls", "a/b", "--limit", "8abc"})
	if err != nil || got.limit != 8 {
		t.Fatalf("parseInt = %+v %v", got, err)
	}
	_, err = parseArgs([]string{"job", "ls", "--limit", "0"})
	if err != nil {
		t.Fatalf("unused limit should be ignored: %v", err)
	}
}

func TestHelpTopics(t *testing.T) {
	cases := []struct {
		args []string
		want string
	}{
		{nil, helpText("")},
		{[]string{"job"}, helpText("job")},
		{[]string{"run", "help"}, helpText("run")},
		{[]string{"help", "agent"}, helpText("agent")},
		{[]string{"agent"}, helpText("")},
		{[]string{"agent", "--help"}, helpText("agent")},
		{[]string{"queue"}, helpText("")},
		{[]string{"--help", "job"}, helpText("job")},
	}
	for _, tc := range cases {
		got, err := parseArgs(tc.args)
		if err != nil {
			t.Fatalf("%v: %v", tc.args, err)
		}
		if got.kind != "help" || helpText(got.topic) != tc.want {
			t.Fatalf("%v topic %q", tc.args, got.topic)
		}
	}
}

func TestPathHelpers(t *testing.T) {
	got, err := JobURL("dt-vrec/feature/search_wm")
	if err != nil || got != "/job/dt-vrec/job/feature/job/search_wm" {
		t.Fatalf("url = %s %v", got, err)
	}
	enc, err := JobURL("a b/c!")
	if err != nil || enc != "/job/a%20b/job/c!" {
		t.Fatalf("enc = %s %v", enc, err)
	}
	if !IsMainline("dt-vrec/master") || !IsMainline("dt-vrec/main") {
		t.Fatal("mainline")
	}
	if IsMainline("dt-vrec/feature/search_wm") || IsMainline("X_dt-vrec") {
		t.Fatal("not mainline")
	}
	if RunResult(true, strPtr("SUCCESS")) != "IN_PROGRESS" {
		t.Fatal("building")
	}
	if RunResult(false, nil) != "UNKNOWN" {
		t.Fatal("unknown")
	}
}

func strPtr(s string) *string { return &s }
