package codeup

import "testing"

func TestParseArgsAudienceAndFlags(t *testing.T) {
	human, err := ParseArgs([]string{"repos"})
	if err != nil {
		t.Fatal(err)
	}
	if human.Kind != kindRepos || human.Audience != audienceHuman || human.Page != 1 || human.PerPage != 50 {
		t.Fatalf("human repos = %+v", human)
	}
	agent, err := ParseArgs([]string{"agent", "cr", "list"})
	if err != nil {
		t.Fatal(err)
	}
	if agent.Kind != kindCrList || agent.Audience != audienceAgent || agent.State != "opened" {
		t.Fatalf("agent cr = %+v", agent)
	}
	hook, err := ParseArgs([]string{"-p", "work", "webhook", "list", "--show-secrets"})
	if err != nil {
		t.Fatal(err)
	}
	if hook.Kind != kindHookList || hook.Profile != "work" || !hook.ShowSecrets {
		t.Fatalf("webhook = %+v", hook)
	}
	withCfg, err := ParseArgs([]string{"-c", "/tmp/codeup.json", "repos"})
	if err != nil {
		t.Fatal(err)
	}
	if withCfg.Config != "/tmp/codeup.json" {
		t.Fatalf("config = %s", withCfg.Config)
	}
	initCmd, err := ParseArgs([]string{"init", "--config", "/tmp/codeup.json"})
	if err != nil {
		t.Fatal(err)
	}
	if initCmd.Kind != kindInit || initCmd.Config != "/tmp/codeup.json" {
		t.Fatalf("init = %+v", initCmd)
	}
}

func TestParsePushAndMerge(t *testing.T) {
	plain, err := ParseArgs([]string{"push"})
	if err != nil {
		t.Fatal(err)
	}
	if plain.Remote != "origin" || plain.HasBranch {
		t.Fatalf("push = %+v", plain)
	}
	branched, err := ParseArgs([]string{"push", "feature/x"})
	if err != nil {
		t.Fatal(err)
	}
	if branched.Branch != "feature/x" || branched.Remote != "origin" {
		t.Fatalf("branch = %+v", branched)
	}
	remote, err := ParseArgs([]string{"push", "--remote", "pub", "feature/x"})
	if err != nil {
		t.Fatal(err)
	}
	if remote.Remote != "pub" || remote.Branch != "feature/x" {
		t.Fatalf("remote = %+v", remote)
	}
	if _, err := ParseArgs([]string{"cr", "create", "--title", "x"}); err == nil || !contains(err.Error(), "missing --source") {
		t.Fatalf("source err = %v", err)
	}
	if _, err := ParseArgs([]string{"cr", "create", "--source", "x"}); err == nil || !contains(err.Error(), "missing --title") {
		t.Fatalf("title err = %v", err)
	}
	if _, err := ParseArgs([]string{"cr", "merge"}); err == nil || !contains(err.Error(), "missing <localId>") {
		t.Fatalf("merge err = %v", err)
	}
	merged, err := ParseArgs([]string{"cr", "merge", "12"})
	if err != nil {
		t.Fatal(err)
	}
	if merged.Kind != kindCrMerge || merged.LocalID != "12" || merged.MergeType != "squash" {
		t.Fatalf("merge = %+v", merged)
	}
	full, err := ParseArgs([]string{"agent", "cr", "merge", "12", "--repo", "g/p", "--type", "rebase", "--message", "ok"})
	if err != nil {
		t.Fatal(err)
	}
	if full.Audience != audienceAgent || full.Repo != "g/p" || full.MergeType != "rebase" || full.Message == nil || *full.Message != "ok" {
		t.Fatalf("full = %+v", full)
	}
	if _, err := ParseArgs([]string{"cr", "merge", "12", "--type", "ours"}); err == nil || !contains(err.Error(), "unknown --type") {
		t.Fatalf("type err = %v", err)
	}
	if _, err := ParseArgs([]string{"merge"}); err == nil || !contains(err.Error(), "unknown command") {
		t.Fatalf("unknown = %v", err)
	}
}

func TestHelpTextMentionsMerge(t *testing.T) {
	if !contains(HelpText(""), "cr list|get|create|merge") {
		t.Fatal("root help missing cr")
	}
	if !contains(HelpText("cr"), "--type squash|ff-only|no-fast-forward|rebase") {
		t.Fatal("cr help missing type")
	}
	if !contains(HelpText("agent"), "cr list|get|create|merge") {
		t.Fatal("agent help missing cr")
	}
	if !contains(HelpText(""), "doctor") || !contains(HelpText(""), "version") {
		t.Fatal("help missing doctor or version")
	}
}

func TestParseIntRejectsZero(t *testing.T) {
	if _, err := ParseArgs([]string{"repos", "--page", "0"}); err == nil || !contains(err.Error(), "invalid number") {
		t.Fatalf("page = %v", err)
	}
	if _, err := ParseArgs([]string{"--bogus", "repos"}); err == nil || !contains(err.Error(), "unknown flag") {
		t.Fatalf("flag = %v", err)
	}
}

func contains(text, part string) bool {
	return len(part) == 0 || (len(text) >= len(part) && (text == part || len(text) > 0 && stringIndex(text, part) >= 0))
}

func stringIndex(text, part string) int {
	for i := 0; i+len(part) <= len(text); i++ {
		if text[i:i+len(part)] == part {
			return i
		}
	}
	return -1
}
