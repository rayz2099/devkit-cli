package rclone

import (
	"strings"
	"testing"
)

func planFor(t *testing.T, action string, body map[string]any, given string, audience string, dry bool) *Plan {
	t.Helper()
	cfg := parseSample(t, body)
	in := planIn{
		cfg: cfg, action: action, task: "photograph-123", dryRun: dry, audience: audience, cwd: "/tmp/x",
	}
	if given != "" {
		in.hasGiven = true
		in.given = given
	}
	plan, err := makePlan(in)
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

func TestPushArgv(t *testing.T) {
	plan := planFor(t, "push", sampleCfg(nil), "", "human", false)
	remote, err := BuildRemote(plan.Target, Secrets{Pass: "obscured"}, plan.Rel, plan.NameEnc, "none", "")
	if err != nil {
		t.Fatal(err)
	}
	argv, err := BuildArgv(plan, "/tmp/rclone-cli-empty.conf", remote, plan.Local)
	if err != nil {
		t.Fatal(err)
	}
	if argv[0] != "--config" || argv[1] != "/tmp/rclone-cli-empty.conf" || argv[2] != "copy" {
		t.Fatalf("%v", argv[:3])
	}
	joined := strings.Join(argv, " ")
	if strings.Contains(joined, "sync") || strings.Contains(joined, "rclone/rclone.conf") || strings.Contains(joined, "_work/**") {
		t.Fatal(joined)
	}
	for _, pattern := range append(append([]string{}, builtinExcludes...), hiddenExcludes...) {
		if !containsStr(argv, pattern) {
			t.Fatal(pattern)
		}
	}
	if !containsStr(argv, "--skip-links") || !containsStr(argv, "--size-only") || !containsStr(argv, "--progress") {
		t.Fatal(argv)
	}
	if argv[indexOf(argv, "--retries")+1] != "10" {
		t.Fatal(argv)
	}
}

func TestArgvEdges(t *testing.T) {
	plan := planFor(t, "push", sampleCfg(map[string]any{"skipHidden": false}), "", "human", false)
	argv, err := BuildArgv(plan, "c", "REMOTE", plan.Local)
	if err != nil {
		t.Fatal(err)
	}
	for _, pattern := range hiddenExcludes {
		if containsStr(argv, pattern) {
			t.Fatal(pattern)
		}
	}
	pulled := planFor(t, "pull", sampleCfg(nil), "", "human", false)
	pullArgv, err := BuildArgv(pulled, "c", "REMOTE", pulled.Local)
	if err != nil {
		t.Fatal(err)
	}
	if pullArgv[2] != "copy" || pullArgv[3] != "REMOTE" || pullArgv[4] != pulled.Local || containsStr(pullArgv, "sync") {
		t.Fatal(pullArgv)
	}
	agent := planFor(t, "push", sampleCfg(map[string]any{"sizeOnly": false, "skipLinks": false}), "", "agent", true)
	argv, err = BuildArgv(agent, "c", "REMOTE", agent.Local)
	if err != nil {
		t.Fatal(err)
	}
	if containsStr(argv, "--progress") || containsStr(argv, "--size-only") || containsStr(argv, "--skip-links") || !containsStr(argv, "--dry-run") {
		t.Fatal(argv)
	}
	over := sampleCfg(map[string]any{
		"crypt": []any{samplePair(nil)}, "encryptFilenames": true,
		"task": map[string]any{"encryptFilenames": false, "transfers": 8, "tpslimit": 8},
	})
	plan = planFor(t, "push", over, "", "human", false)
	remote, err := BuildRemote(plan.Target, Secrets{Pass: "obscured", Crypt: []CryptSecret{{Name: "v1", Password: "one", Password2: "two"}}}, plan.Rel, plan.NameEnc, "none", "")
	if err != nil {
		t.Fatal(err)
	}
	if plan.NameEnc != "off" || !strings.Contains(remote, "filename_encryption=off") || !strings.Contains(remote, "suffix='none'") {
		t.Fatal(remote)
	}
	argv, err = BuildArgv(plan, "c", remote, plan.Local)
	if err != nil {
		t.Fatal(err)
	}
	if argv[indexOf(argv, "--transfers")+1] != "8" || argv[indexOf(argv, "--tpslimit")+1] != "8" {
		t.Fatal(argv)
	}
	got := planFor(t, "get", sampleCfg(nil), "album/DSC_0001.NEF", "human", false)
	argv, err = BuildArgv(got, "c", "REMOTE", got.Local)
	if err != nil {
		t.Fatal(err)
	}
	if argv[2] != "copyto" || argv[3] != "REMOTE" || argv[4] != got.Local || containsStr(argv, "--exclude") || !containsStr(argv, "-q") || containsStr(argv, "--progress") {
		t.Fatal(argv)
	}
	if argv[indexOf(argv, "--retries")+1] != "1" || argv[indexOf(argv, "--low-level-retries")+1] != "1" {
		t.Fatal(argv)
	}
}

func TestPlanPaths(t *testing.T) {
	cfg := parseSample(t, sampleCfg(nil))
	in := planIn{cfg: cfg, action: "download", task: "photograph-123", hasGiven: true, given: "album/DSC_0001.NEF", audience: "human", cwd: "/tmp/x"}
	plan, err := makePlan(in)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Local != "/tmp/x/DSC_0001.NEF" {
		t.Fatal(plan.Local)
	}
	in.verbose = true
	in.given = "album/a.JPG"
	plan, err = makePlan(in)
	if err != nil {
		t.Fatal(err)
	}
	argv, err := BuildArgv(plan, "c", "REMOTE", plan.Local)
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Verbose || !containsStr(argv, "-v") || containsStr(argv, "-q") || !containsStr(argv, "--progress") {
		t.Fatal(argv)
	}
	loud, err := parseArgs([]string{"download", "photograph-123", "a.JPG", ".", "-v"})
	if err != nil || !loud.verbose || loud.dest != "." {
		t.Fatal(loud, err)
	}
	in = planIn{cfg: cfg, action: "download", task: "photograph-123", hasGiven: true, given: "2025-04-04/DSC_0746.JPG.bin", audience: "human", cwd: "/tmp/x"}
	plan, err = makePlan(in)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Rel != "2025-04-04/DSC_0746.JPG" {
		t.Fatal(plan.Rel)
	}
	if _, err := makePlan(planIn{cfg: cfg, action: "pull", task: "photograph-123", hasGiven: true, given: "album/DSC_0001.NEF", audience: "human", cwd: "/tmp"}); err == nil || !strings.Contains(err.Error(), "download one file") {
		t.Fatal(err)
	}
	body := sampleCfg(nil)
	body["targets"].([]any)[0].(map[string]any)["root"] = "photograph-crypt"
	bad := parseSample(t, body)
	if _, err := makePlan(planIn{cfg: bad, action: "push", task: "photograph-123", audience: "human", cwd: "/tmp"}); err == nil || !strings.Contains(err.Error(), "root must match source folder photograph") {
		t.Fatal(err)
	}
	if StripBin("album/DSC_0746.JPG.bin") != "album/DSC_0746.JPG" || StripBin("album/DSC_0746.JPG") != "album/DSC_0746.JPG" {
		t.Fatal("strip")
	}
	local, rel, err := resolveRel("/src", "a/b")
	if err != nil || local != "/src/a/b" || rel != "a/b" {
		t.Fatal(local, rel, err)
	}
	if _, rel, err = resolveRel("/src", ""); err != nil || rel != "" {
		t.Fatal(rel, err)
	}
	if _, _, err = resolveRel("/src", "../etc"); err == nil {
		t.Fatal("expected outside")
	}
	if _, _, err = resolveRel("/src", "/etc"); err == nil || !strings.Contains(err.Error(), "path outside source") {
		t.Fatal(err)
	}
	stats := parseStats(strings.Join([]string{
		"Transferred:   \t 1.000 KiB / 1.000 KiB, 100%, 1 KiB/s, ETA 0s",
		"Checks:                 2 / 2, 100%",
		"Transferred:            3 / 3, 100%",
		"Elapsed time:         1.0s",
	}, "\n"))
	if stats.bytes == nil || *stats.bytes != 1024 || stats.checks == nil || *stats.checks != 2 || stats.transfers == nil || *stats.transfers != 3 {
		t.Fatalf("%#v", stats)
	}
	empty := parseStats("no stats")
	if empty.bytes != nil || empty.checks != nil || empty.transfers != nil {
		t.Fatal(empty)
	}
}

func TestConnRoundTrip(t *testing.T) {
	cfg := parseSample(t, sampleCfg(nil))
	remote, err := BuildRemote(cfg.Targets[0], Secrets{Pass: plainPass}, "album", "standard", "none", "")
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseConn(remote)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Config["pass"] != plainPass || parsed.Config["pacer_min_sleep"] != "200ms" || parsed.Path != "photograph/album" {
		t.Fatalf("%#v", parsed)
	}
	if !strings.HasPrefix(remote, ":webdav,") {
		t.Fatal(remote)
	}
	cryptCfg := parseSample(t, sampleCfg(map[string]any{"crypt": []any{samplePair(map[string]any{"key": "one", "salt": "two,x:y"})}}))
	remote, err = BuildRemote(cryptCfg.Targets[0], Secrets{
		Pass: plainPass, Crypt: []CryptSecret{{Name: "v1", Password: "one'one", Password2: "two,x:y"}},
	}, "album", "standard", "none", "")
	if err != nil {
		t.Fatal(err)
	}
	outer, err := ParseConn(remote)
	if err != nil {
		t.Fatal(err)
	}
	if outer.Name != ":crypt" || outer.Path != "album" || outer.Config["password"] != "one'one" || outer.Config["password2"] != "two,x:y" {
		t.Fatalf("%#v", outer)
	}
	inner, err := ParseConn(outer.Config["remote"])
	if err != nil {
		t.Fatal(err)
	}
	if inner.Config["pass"] != plainPass || inner.Path != "photograph" || strings.Contains(remote, "photograph-crypt") {
		t.Fatal(inner, remote)
	}
	remote, err = BuildRemote(cryptCfg.Targets[0], Secrets{Pass: plainPass, Crypt: []CryptSecret{{Name: "v1", Password: "one", Password2: "two"}}}, "", "off", "none", "")
	if err != nil {
		t.Fatal(err)
	}
	outer, err = ParseConn(remote)
	if err != nil {
		t.Fatal(err)
	}
	if outer.Config["filename_encryption"] != "off" || outer.Config["suffix"] != "none" || outer.Path != "" {
		t.Fatalf("%#v", outer)
	}
	plain := BuildPlainRemote(cryptCfg.Targets[0], plainPass, "album/DSC_0746.JPG.bin")
	if !strings.HasPrefix(plain, ":webdav,") || !strings.Contains(plain, "photograph/album/DSC_0746.JPG.bin") || strings.Contains(plain, ":crypt") {
		t.Fatal(plain)
	}
	local := BuildLocalCrypt("/tmp/blob", CryptSecret{Name: "v1", Password: "one", Password2: "two"}, "DSC_0746.JPG.bin")
	parsed, err = ParseConn(local)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Name != ":crypt" || parsed.Path != "DSC_0746.JPG.bin" || parsed.Config["remote"] != "/tmp/blob" || parsed.Config["suffix"] != "none" {
		t.Fatalf("%#v", parsed)
	}
	remote, err = BuildRemote(cryptCfg.Targets[0], Secrets{Pass: plainPass, Crypt: []CryptSecret{{Name: "v1", Password: "one", Password2: "two"}}}, "album/DSC_0001.NEF", "off", ".bin", "")
	if err != nil {
		t.Fatal(err)
	}
	outer, err = ParseConn(remote)
	if err != nil {
		t.Fatal(err)
	}
	if outer.Config["suffix"] != ".bin" || outer.Path != "album/DSC_0001.NEF" {
		t.Fatalf("%#v", outer)
	}
}

func TestListing(t *testing.T) {
	items, err := parseLsJson(`[{"Path":"album/a.NEF","Size":10,"IsDir":false},{"Path":"album","Size":0,"IsDir":true},{"Path":"album/b.NEF.bin","Size":20}]`)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].Path != "album/a.NEF" || items[1].Size != 20 {
		t.Fatal(items)
	}
	report, err := ComputeInv("photograph-123", "", []FileEnt{
		{Path: "a.NEF", Size: 10}, {Path: "b.NEF", Size: 20}, {Path: "c.NEF", Size: 30},
	}, []FileEnt{
		{Path: "a.NEF", Size: 10}, {Path: "b.NEF.bin", Size: 20}, {Path: "c.NEF", Size: 99}, {Path: "only-remote.JPG", Size: 1},
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	if report.Counts.OK != 1 || report.Counts.OKBin != 1 || report.Counts.Differ != 1 || report.Counts.Extra != 1 || !checkFailed(report) {
		t.Fatalf("%#v", report.Counts)
	}
	same, err := ComputeInv("t", "", []FileEnt{{Path: "a.NEF", Size: 10}}, []FileEnt{{Path: "a.NEF", Size: 10}, {Path: "a.NEF.bin", Size: 10}}, true)
	if err != nil || same.Counts.OK != 1 || same.Counts.Extra != 1 || checkFailed(same) {
		t.Fatal(err, same)
	}
	clipped := clipInv(report, 2)
	if len(clipped.Rows) != 2 || clipped.Total == nil || *clipped.Total != 4 || clipped.Counts.Differ != 1 {
		t.Fatalf("%#v", clipped)
	}
	plain, err := ComputeInv("t", "", []FileEnt{{Path: "a.NEF", Size: 10}}, []FileEnt{{Path: "a.NEF.bin", Size: 10}}, false)
	if err != nil || plain.Counts.Missing != 1 || plain.Counts.Extra != 1 || !checkFailed(plain) {
		t.Fatal(err, plain)
	}
}

func indexOf(items []string, want string) int {
	for i, item := range items {
		if item == want {
			return i
		}
	}
	return -1
}
