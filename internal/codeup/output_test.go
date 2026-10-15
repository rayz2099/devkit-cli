package codeup

import "testing"

func TestMaskHooks(t *testing.T) {
	hook := Webhook{
		ID:                  "1",
		URL:                 "https://example.com/hook",
		SecretToken:         "s3cret",
		PushEvents:          true,
		MergeRequestsEvents: true,
		TagPushEvents:       false,
		NoteEvents:          false,
		CreatedAt:           "2026-01-01T00:00:00+08:00",
		UpdatedAt:           "2026-01-02T00:00:00+08:00",
	}
	masked := MaskHooks([]Webhook{hook}, false)
	if masked[0].SecretToken != "***" {
		t.Fatalf("secret = %s", masked[0].SecretToken)
	}
	shown := MaskHooks([]Webhook{hook}, true)
	if shown[0].SecretToken != "s3cret" {
		t.Fatalf("shown = %s", shown[0].SecretToken)
	}
	withQuery := hook
	withQuery.URL = "https://example.com/invoke?token=s3cret"
	got := MaskHooks([]Webhook{withQuery}, false)
	if got[0].URL != "https://example.com/invoke?token=***" {
		t.Fatalf("url = %s", got[0].URL)
	}
	empty := hook
	empty.SecretToken = ""
	if MaskHooks([]Webhook{empty}, false)[0].SecretToken != "" {
		t.Fatal("empty secret was masked")
	}
}

func TestReposTable(t *testing.T) {
	text := reposText(nil)
	if text != "No repositories found\n" {
		t.Fatalf("empty = %q", text)
	}
	text = reposText([]Repo{{
		ID: "1", Name: "demo", PathNs: "backend/demo-api", DefBranch: "master", WebURL: "https://example",
	}})
	want := "" +
		"ID  NAME  PATH              DEFAULT  URL            \n" +
		"1   demo  backend/demo-api  master   https://example\n"
	if text != want {
		t.Fatalf("table\n%s\nwant\n%s", text, want)
	}
}

func TestCRViewAndClip(t *testing.T) {
	cr := ChangeRequest{
		LocalID: "9", Title: "hello", State: "opened", SrcBranch: "a", TgtBranch: "b",
		Author: "ray", CreatedAt: "c", UpdatedAt: "u", CrURL: "https://cr/9", Description: "body",
	}
	text := crViewText("group/demo", cr)
	if !contains(text, "repo\tgroup/demo") || !contains(text, "url\thttps://cr/9") || !contains(text, "\n\nbody\n") {
		t.Fatalf("view = %q", text)
	}
	if clip("abcdef", 3) != "abc" {
		t.Fatal("clip")
	}
}
