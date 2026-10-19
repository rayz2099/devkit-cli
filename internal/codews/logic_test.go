package codews

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigResolve(t *testing.T) {
	raw, err := ParseConfigBytes([]byte(`{
	  "workspaceRoot": "$HOME/projects/workspaces",
	  "baseBranch": "master",
	  "remote": "origin",
	  "initAgentsTemplate": "work",
	  "profiles": {
	    "daily": {"name": "profile name can change", "agentsTemplate": "work-01", "repos": ["app-gw", "app-audit"]}
	  }
	}`))
	if err != nil {
		t.Fatal(err)
	}
	txt := strings.Join([]string{
		`home: "$HOME/projects"`,
		"projects:",
		"  - repo-name: app-gw",
		"    group: backend",
		"    description: app gateway",
		"  - repo-name: app-audit",
		"    group: frontend",
	}, "\n")
	cfg, err := ResolveConfig(raw, txt, ResolveOpts{Home: "/home/dev", Tpl: "/tpl"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.WsRoot != "/home/dev/projects/workspaces" {
		t.Fatal(cfg.WsRoot)
	}
	if cfg.InitTpl != "/tpl/work" {
		t.Fatal(cfg.InitTpl)
	}
	repos := cfg.Profiles["daily"].Repos
	if len(repos) != 2 || repos[0].Path != "/home/dev/projects/backend/app-gw" || repos[0].Desc != "app gateway" || repos[0].Branch != "master" {
		t.Fatalf("%+v", repos)
	}
	if repos[1].Path != "/home/dev/projects/frontend/app-audit" || repos[1].Desc != "" {
		t.Fatalf("%+v", repos[1])
	}
	_, err = ParseConfigBytes([]byte(`{"baseBranch":"m","remote":"o","initAgentsTemplate":"w","profiles":{}}`))
	if err == nil || !strings.Contains(err.Error(), "workspaceRoot must be a non-empty string") {
		t.Fatal(err)
	}
}

func TestMerge(t *testing.T) {
	specs, err := ParseFrom([]string{"my-room=V0.0.1", "other=main"})
	if err != nil || specs["my-room"] != "V0.0.1" {
		t.Fatal(err, specs)
	}
	_, err = ParseFrom([]string{"my-room=a", "my-room=b"})
	if err == nil || err.Error() != "--from repeated for repo: my-room" {
		t.Fatal(err)
	}
	_, err = ParseFrom([]string{"my-room"})
	if err == nil || !strings.Contains(err.Error(), "--from requires repo=branch: my-room") {
		t.Fatal(err)
	}
	repos := []Repo{{Name: "my-room", Branch: "master"}, {Name: "app", Branch: "master"}}
	out, err := ApplyMerge(repos, map[string]string{"my-room": "V0.0.1"}, func(repo Repo, ref string) (string, error) {
		return ref, nil
	})
	if err != nil || out[0].Merge != "V0.0.1" || out[1].Merge != "master" {
		t.Fatal(err, out)
	}
	_, err = ApplyMerge(repos, map[string]string{"missing": "x"}, func(Repo, string) (string, error) { return "", nil })
	if err == nil || !strings.Contains(err.Error(), "not selected: missing") {
		t.Fatal(err)
	}
}

func TestGitPlan(t *testing.T) {
	repo := Repo{Name: "app-gw", Path: "/src/app-gw", Group: "backend", Branch: "master"}
	plan := BuildGitPlan(PlanArgs{
		Branch: "feature/spec101", Base: "master", Remote: "origin",
		WsDir: "/tmp/spec101-workspace", Repos: []Repo{repo},
		BranchExists: func(Repo, string) bool { return false },
	})
	if plan[0].Step != "worktree-prune" || plan[0].Args[2] != "prune" {
		t.Fatal(plan[0])
	}
	last := plan[len(plan)-1]
	want := []string{"git", "worktree", "add", "-b", "feature/spec101", "/tmp/spec101-workspace/app-gw", "origin/master"}
	if strings.Join(last.Args, " ") != strings.Join(want, " ") {
		t.Fatal(last.Args)
	}
	plan = BuildGitPlan(PlanArgs{
		Branch: "feature/spec101", Base: "master", Remote: "origin",
		WsDir: "/tmp/spec101-workspace", Repos: []Repo{repo},
		BranchExists: func(Repo, string) bool { return true },
	})
	if strings.Join(plan[len(plan)-1].Args, " ") != "git worktree add /tmp/spec101-workspace/app-gw feature/spec101" {
		t.Fatal(plan[len(plan)-1].Args)
	}
	fixed := repo
	fixed.Checkout = "release"
	plan = BuildGitPlan(PlanArgs{
		Branch: "feature/spec101", Base: "master", Remote: "origin",
		WsDir: "/ws", Repos: []Repo{fixed},
		BranchExists: func(Repo, string) bool { return false },
	})
	if !strings.Contains(strings.Join(plan[len(plan)-1].Args, " "), "--detach") {
		t.Fatal(plan[len(plan)-1].Args)
	}
	sync, err := BuildSyncPlan(SyncArgs{
		Base: "master", Remote: "origin", Repos: []Repo{repo},
		Checkout: func(Repo, string) (string, bool, error) { return "", false, nil },
	})
	if err != nil || sync[0].Args[3] != "master:master" {
		t.Fatal(err, sync[0].Args)
	}
	if !PatchMerged("- a\n- b\n", "0") {
		t.Fatal("patch")
	}
	if PatchMerged("+ a\n", "0") || PatchMerged("- a\n", "1") {
		t.Fatal("not merged")
	}
	if MatchedHead("abc\trefs/heads/V0.0.1\n", "V0.0.1") != "V0.0.1" {
		t.Fatal("head")
	}
	if MatchedHead("abc\trefs/heads/v0.0.1\n", "V0.0.1") != "" {
		t.Fatal("case")
	}
	names := RemoteNames("  origin/main\n  origin/HEAD -> origin/main\n  origin/feature/a\n", "origin")
	if len(names) != 2 || names[0] != "main" || names[1] != "feature/a" {
		t.Fatal(names)
	}
}

func TestWorkspaceYAML(t *testing.T) {
	name, err := WorkspaceName("feature/spec101")
	if err != nil || name != "spec101-workspace" {
		t.Fatal(name, err)
	}
	repos := []Repo{{
		Name: "app", Group: "backend", Branch: "master", Desc: "api", Checkout: "rel", Merge: "main",
	}}
	txt := CreateProjectYAML("feature/x", repos, []string{"it-env-no:"})
	ws, err := ParseWsProject(txt)
	if err != nil {
		t.Fatal(err)
	}
	if ws.Branch != "feature/x" || len(ws.Repos) != 1 || ws.Checkout["app"] != "rel" || ws.Merges["app"] != "main" {
		t.Fatalf("%+v", ws)
	}
	if !ShouldDelete("y") || ShouldDelete("yes") || ShouldDelete("") {
		t.Fatal("confirm")
	}
}

func TestMdSearchSymbolCF(t *testing.T) {
	src := "docs/postgres/README.md"
	if ResolveMdHref(src, "architecture.md", "link") != "/docs/postgres/architecture.md" {
		t.Fatal(ResolveMdHref(src, "architecture.md", "link"))
	}
	if ResolveMdHref(src, "img/er.png", "image") != "/raw/docs/postgres/img/er.png" {
		t.Fatal("img")
	}
	if ResolveMdHref(src, "../assets/x.png", "image") != "/raw/docs/assets/x.png" {
		t.Fatal("up")
	}
	if ResolveMdHref(src, "#install", "link") != "#install" {
		t.Fatal("hash")
	}
	if ResolveMdHref("README.md", "../../secret.md", "link") != "/secret.md" {
		t.Fatal("clamp")
	}
	q := ParseQuery("readme .md")
	if len(q.Terms) != 1 || q.Terms[0] != "readme" || len(q.Exts) != 1 || q.Exts[0] != ".md" {
		t.Fatal(q)
	}
	q = ParseQuery("type:kt OssApi")
	if q.Terms[0] != "OssApi" || strings.Join(q.Exts, ",") != ".kt,.kts" {
		t.Fatal(q)
	}
	if FuzzyScore("dt-metadata.md", "readme.md") != -1 {
		t.Fatal("fuzzy")
	}
	files := []string{
		"saturn/README.md",
		"kt-file/README.md",
		"kt-file/libs/kt-file-tools/README.md",
		"tasks/tools-cdn-replace-cdn-host-util/dt-metadata.md",
		"kt-file/libs/kt-file-tools/src/main/kotlin/OssApi.kt",
	}
	ranked := RankPaths(files, "oss .kt", nil, 50)
	if len(ranked) != 1 || ranked[0].Path != "kt-file/libs/kt-file-tools/src/main/kotlin/OssApi.kt" {
		t.Fatal(ranked)
	}
	recent := RankPaths(files, "", []string{"kt-file/libs/kt-file-tools/README.md", "missing.md"}, 3)
	if len(recent) != 3 || recent[0].Path != "kt-file/libs/kt-file-tools/README.md" {
		t.Fatal(recent)
	}
	if QuickArgs(7001)[3] != "http://127.0.0.1:7001" {
		t.Fatal(QuickArgs(7001))
	}
	log := "INF |  \u001b[1mhttps://demo-ray.trycloudflare.com\u001b[0m  |\nINF https://later-one.trycloudflare.com"
	if ExtractTunnel(log) != "https://demo-ray.trycloudflare.com" {
		t.Fatal(ExtractTunnel(log))
	}
	if ExtractTunnel(`ERR "https://api.trycloudflare.com/tunnel"`) != "" {
		t.Fatal("api")
	}
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, "src", "Foo.kt"), []byte("package demo\nclass Foo {\n  fun bar() {}\n}\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "src", "Use.kt"), []byte("import demo.Foo\nfun main() { Foo() }\n"), 0o644)
	idx := ListIndex(dir, 0)
	found := SearchSymbol(dir, idx, "Foo")
	def := ""
	line := 0
	ref := false
	for _, hit := range found.Hits {
		if hit.Kind == "def" && def == "" {
			def = hit.Path
			line = hit.Line
		}
		if hit.Kind == "ref" && hit.Path == "src/Use.kt" {
			ref = true
		}
	}
	if def != "src/Foo.kt" || line != 2 || !ref {
		t.Fatalf("%+v", found)
	}
	short := SearchSymbol(dir, idx, "F")
	if len(short.Hits) != 0 || short.Truncated {
		t.Fatal(short)
	}
}

func TestServeFSAndHTTP(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, "docs", "a.md"), []byte("# hi\n"), 0o644)
	os.WriteFile(filepath.Join(dir, ".env"), []byte("SECRET=1\n"), 0o644)
	os.MkdirAll(filepath.Join(dir, ".git"), 0o755)
	os.WriteFile(filepath.Join(dir, ".git", "config"), []byte("x\n"), 0o644)
	os.MkdirAll(filepath.Join(dir, "node_modules", "pkg"), 0o755)
	os.WriteFile(filepath.Join(dir, "node_modules", "pkg", "index.js"), []byte("x\n"), 0o644)
	if !DeniedRel(".git/config") || !DeniedRel("app/.env") || !DeniedRel("certs/server.pem") || DeniedRel("src/main.ts") {
		t.Fatal("deny")
	}
	if _, err := UnderRoot(dir, "../etc/passwd"); err == nil {
		t.Fatal("traversal")
	}
	blob, err := ReadBlob(dir, "")
	if err != nil || blob.Type != "dir" {
		t.Fatal(err, blob.Type)
	}
	for _, ent := range blob.Entries {
		if ent.Name == ".env" || ent.Name == ".git" {
			t.Fatal(ent.Name)
		}
	}
	md, err := ReadBlob(dir, "docs/a.md")
	if err != nil || md.Content == nil || !strings.Contains(*md.Content, "# hi") || md.Lang != "markdown" || md.Preview != "markdown" {
		t.Fatal(err, md)
	}
	files := ListIndex(dir, 0)
	joined := strings.Join(files, ",")
	if !strings.Contains(joined, "docs/a.md") || strings.Contains(joined, ".env") || strings.Contains(joined, "node_modules") {
		t.Fatal(files)
	}
	start, end, ok := ParseByteRange("bytes=0-3", 10)
	if !ok || start != 0 || end != 3 {
		t.Fatal(start, end, ok)
	}
	start, end, ok = ParseByteRange("bytes=-4", 10)
	if !ok || start != 6 || end != 9 {
		t.Fatal(start, end)
	}
	if _, _, ok = ParseByteRange("bytes=0-1,2-3", 10); ok {
		t.Fatal("multi")
	}
	paths := ScanWatch(dir)
	if !paths["docs"] || !paths["docs/a.md"] || paths[".env"] || paths["node_modules"] {
		t.Fatal(paths)
	}
	known := ScanWatch(dir)
	os.WriteFile(filepath.Join(dir, "docs", "a.md"), []byte("# after\n"), 0o644)
	ev := ReconcileWatch(ReconcileOpts{Root: dir, Known: known, Paths: map[string]bool{"docs/a.md": true}})
	if ev == nil || ev.Type != "files-changed" || ev.Changes[0].Kind != "change" {
		t.Fatal(ev)
	}
	page := RenderPage(`a<b`)
	if !strings.Contains(page, "<title>a&lt;b · code-ws serve</title>") {
		t.Fatal("title")
	}
	if !strings.Contains(page, `rootName: "a<b"`) && !strings.Contains(page, `rootName: "a<b"`) {
		t.Fatal("json root")
	}
	if strings.Contains(page, rootSentinel) {
		t.Fatal("sentinel left")
	}
	if !strings.Contains(page, "palette-input") || !strings.Contains(page, "marked.min.js") {
		t.Fatal("ui stripped")
	}
}

func TestInitTarget(t *testing.T) {
	cfg := Cfg{
		InitTpl: "/templates/work",
		Projects: []Repo{
			{Name: "app", Path: "/src/app", Branch: "master"},
			{Name: "other", Path: "/src/other", Branch: "master"},
		},
		Profiles: map[string]Profile{
			"daily": {Agents: "/templates/default", Repos: []Repo{{Name: "app", Path: "/src/app"}}},
		},
	}
	tpl, repos, err := InitTarget(cfg, Parsed{HasProj: true, Project: "app"})
	if err != nil || tpl != "/templates/work" || len(repos) != 1 || repos[0].Name != "app" {
		t.Fatal(err, tpl, repos)
	}
	tpl, repos, err = InitTarget(cfg, Parsed{HasProf: true, Profile: "daily", HasProj: true, Project: "other"})
	if err != nil || tpl != "/templates/default" || repos[0].Name != "other" {
		t.Fatal(err, tpl, repos)
	}
	_, _, err = InitTarget(cfg, Parsed{HasProf: true, Profile: "missing"})
	if err == nil || !strings.Contains(err.Error(), "profile not found: missing") {
		t.Fatal(err)
	}
}

func TestAgentsLink(t *testing.T) {
	root := t.TempDir()
	tpl := filepath.Join(root, "templates", "agents", "work")
	if err := os.MkdirAll(tpl, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tpl, "AGENTS.md"), []byte("hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ws := filepath.Join(root, "ws")
	if err := ApplyAgents(tpl, ws); err != nil {
		t.Fatal(err)
	}
	target, err := os.Readlink(filepath.Join(ws, "AGENTS.md"))
	if err != nil || target != filepath.Join(tpl, "AGENTS.md") {
		t.Fatal(target, err)
	}
	if _, err := os.Stat(filepath.Join(ws, "spec", "context.md")); err != nil {
		t.Fatal(err)
	}
	if err := ApplyAgents(filepath.Join(root, "missing"), ws); err == nil || !strings.Contains(err.Error(), "agents template not found:") {
		t.Fatal(err)
	}
	wsNoTpl := filepath.Join(root, "ws-no-tpl")
	if err := ApplyAgents("", wsNoTpl); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(wsNoTpl, "AGENTS.md")); err == nil {
		t.Fatal("AGENTS.md should not be linked when template is empty")
	}
	if _, err := os.Stat(filepath.Join(wsNoTpl, "spec", "context.md")); err != nil {
		t.Fatal("context.md should still be created")
	}
}

// TestEmptyProfileAndNoTemplate 验证支持空 repos 与无 template 的 profile 解析、初始化全流程。
func TestEmptyProfileAndNoTemplate(t *testing.T) {
	raw, err := ParseConfigBytes([]byte(`{
	  "workspaceRoot": "$HOME/projects/workspaces",
	  "baseBranch": "master",
	  "remote": "origin",
	  "profiles": {
	    "default": {"name": "default", "agentsTemplate": "", "repos": []}
	  }
	}`))
	if err != nil {
		t.Fatal(err)
	}
	catalog := "home: \"$HOME/projects\"\nprojects:\n"
	cfg, err := ResolveConfig(raw, catalog, ResolveOpts{Home: "/home/dev", Tpl: "/tpl"})
	if err != nil {
		t.Fatal(err)
	}
	prof := cfg.Profiles["default"]
	if prof.Agents != "" {
		t.Fatalf("expected empty agents, got %q", prof.Agents)
	}
	if len(prof.Repos) != 0 {
		t.Fatalf("expected 0 repos, got %d", len(prof.Repos))
	}
	tpl, repos, err := InitTarget(cfg, Parsed{HasProf: true, Profile: "default"})
	if err != nil || tpl != "" || len(repos) != 0 {
		t.Fatalf("InitTarget failed: err=%v, tpl=%q, repos=%v", err, tpl, repos)
	}
	if err := checkAgents(tpl); err != nil {
		t.Fatalf("checkAgents should accept empty template: %v", err)
	}
}

// TestInitEmptyWorkspace 验证 code-ws init 在使用 default profile 初始化空项目且无模板时的端到端执行结果。
func TestInitEmptyWorkspace(t *testing.T) {
	root := t.TempDir()
	confDir := filepath.Join(root, ".config", "code-ws")
	if err := os.MkdirAll(confDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfgJSON := `{
	  "workspaceRoot": "` + filepath.Join(root, "workspaces") + `",
	  "baseBranch": "master",
	  "remote": "origin",
	  "profiles": {
	    "default": {"name": "default", "agentsTemplate": "", "repos": []}
	  }
	}`
	cfgPath := filepath.Join(confDir, "config.json")
	if err := os.WriteFile(cfgPath, []byte(cfgJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	projYML := "home: \"" + filepath.Join(root, "projects") + "\"\nprojects:\n"
	if err := os.WriteFile(filepath.Join(confDir, "project.yml"), []byte(projYML), 0o644); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	err := Run(strings.NewReader(""), &out, io.Discard, []string{"init", "feature/empty-test", "-c", cfgPath})
	if err != nil {
		t.Fatalf("Run init failed: %v\nOutput: %s", err, out.String())
	}
	wsDir := filepath.Join(root, "workspaces", "empty-test-workspace")
	if _, err := os.Stat(filepath.Join(wsDir, "empty-test.code-workspace")); err != nil {
		t.Fatalf("workspace file missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(wsDir, "project.yml")); err != nil {
		t.Fatalf("project.yml missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(wsDir, "spec", "context.md")); err != nil {
		t.Fatalf("context.md missing: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(wsDir, "AGENTS.md")); err == nil {
		t.Fatal("AGENTS.md should not exist when template is empty")
	}
}
