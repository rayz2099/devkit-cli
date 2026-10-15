package codeup

import "testing"

func TestParseOrgID(t *testing.T) {
	got, err := ParseOrgID("https://codeup.aliyun.com/org-example")
	if err != nil || got != "org-example" {
		t.Fatalf("org = %s %v", got, err)
	}
	if _, err := ParseOrgID("https://codeup.aliyun.com/org-example/group/repo"); err == nil || !contains(err.Error(), "organization url must be") {
		t.Fatalf("deep = %v", err)
	}
	if _, err := ParseOrgID("https://example.com/org-example"); err == nil || !contains(err.Error(), "host must be") {
		t.Fatalf("host = %v", err)
	}
	if _, err := ParseOrgID("https://codeup.aliyun.com/<organizationId>"); err == nil || !contains(err.Error(), "missing") {
		t.Fatalf("placeholder = %v", err)
	}
}

func TestEncodeRepoID(t *testing.T) {
	id, err := EncodeRepoID("12", "org-example")
	if err != nil || id != "12" {
		t.Fatalf("num = %s %v", id, err)
	}
	path, err := EncodeRepoID("group/demo", "org-example")
	if err != nil || path != "org-example%2Fgroup%2Fdemo" {
		t.Fatalf("path = %s %v", path, err)
	}
	full, err := EncodeRepoID("org-example/group/demo", "org-example")
	if err != nil || full != "org-example%2Fgroup%2Fdemo" {
		t.Fatalf("full = %s %v", full, err)
	}
	short, err := EncodeRepoID("demo", "org-example")
	if err != nil || short != "org-example%2Fdemo" {
		t.Fatalf("short = %s %v", short, err)
	}
}

func TestGitRemotePath(t *testing.T) {
	got, err := RepoFromRemotePath("org-example/group/demo", "org-example")
	if err != nil || got != "group/demo" {
		t.Fatalf("strip = %s %v", got, err)
	}
	plain, err := RepoFromRemotePath("group/demo", "org-example")
	if err != nil || plain != "group/demo" {
		t.Fatalf("plain = %s %v", plain, err)
	}
	remote, err := ParseGitRemote("git@codeup.aliyun.com:org-example/group/demo.git")
	if err != nil {
		t.Fatal(err)
	}
	if remote.Host != "codeup.aliyun.com" || remote.Path != "org-example/group/demo" {
		t.Fatalf("ssh = %+v", remote)
	}
	https, err := ParseGitRemote("https://codeup.aliyun.com/org-example/group/demo.git")
	if err != nil {
		t.Fatal(err)
	}
	if https.Path != "org-example/group/demo" {
		t.Fatalf("https = %+v", https)
	}
}

func TestProjectID(t *testing.T) {
	n, err := toProjectID("12")
	if err != nil || n != 12 {
		t.Fatalf("id = %d %v", n, err)
	}
	if _, err := toProjectID("12a"); err == nil || !contains(err.Error(), "invalid repository project id") {
		t.Fatalf("bad = %v", err)
	}
	body := crCreateBody(12, crCreate{Source: "feature/x", Target: "master", Title: "t"})
	if body.Description != nil || body.SourceProjectID != 12 || body.Title != "t" {
		t.Fatalf("create = %+v", body)
	}
	merge := crMergeBody(crMerge{Type: "ff-only", Message: strPtr("ok")})
	if merge.MergeType != "ff-only" || merge.MergeMessage == nil || *merge.MergeMessage != "ok" {
		t.Fatalf("merge = %+v", merge)
	}
}

func strPtr(v string) *string {
	return &v
}
