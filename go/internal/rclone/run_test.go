package rclone

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPushNoTask(t *testing.T) {
	home := withHome(t)
	writeCfg(t, home, sampleCfg(nil), 0o600)
	got := runCLI(t, home, stubOf(0, ""), "push")
	if got.code != 2 || !strings.Contains(got.stderr, "photograph-123") || !strings.Contains(got.stderr, "no default task") {
		t.Fatalf("%+v", got)
	}
	recs, err := ReadRuns()
	if err != nil || len(recs) != 0 {
		t.Fatal(err, recs)
	}
}

func TestFailedPushRecords(t *testing.T) {
	home := withHome(t)
	writeCfg(t, home, sampleCfg(nil), 0o600)
	spawn, copies := stubSpawn(7, "boom\n")
	got := runCLI(t, home, spawn, "push", "photograph-123")
	if got.code != 7 || !strings.Contains(got.stdout, "failed (7)") {
		t.Fatalf("%+v", got)
	}
	if len(*copies) != 1 || (*copies)[0][2] != "copy" || containsStr((*copies)[0], "sync") {
		t.Fatal(*copies)
	}
	joined := strings.Join((*copies)[0], "\n")
	if strings.Contains(joined, "rclone/rclone.conf") || strings.Contains(joined, plainPass) {
		t.Fatal(joined)
	}
	cfgArg := (*copies)[0][indexOf((*copies)[0], "--config")+1]
	if !strings.HasSuffix(cfgArg, "/rclone-cli/empty.conf") {
		t.Fatal(cfgArg)
	}
	buf, err := os.ReadFile(cfgArg)
	if err != nil || string(buf) != "" {
		t.Fatal(err, string(buf))
	}
	if strings.Contains(cfgArg, "/.config/") {
		t.Fatal("empty.conf landed in config", cfgArg)
	}
	recs, err := ReadRuns()
	if err != nil || len(recs) != 1 || recs[0].Code != 7 || recs[0].Task != "photograph-123" {
		t.Fatal(err, recs)
	}
	if !strings.Contains(recs[0].Error, "boom") || strings.Contains(strings.Join(recs[0].Args, "\n"), plainPass) || recs[0].Remote != "123:photograph" {
		t.Fatalf("%#v", recs[0])
	}
}

func TestDryRunAgent(t *testing.T) {
	home := withHome(t)
	writeCfg(t, home, sampleCfg(nil), 0o600)
	stderr := strings.Join([]string{
		"Transferred:   \t 0 B / 0 B, -, 0 B/s, ETA -",
		"Checks:                 4 / 4, -",
		"Transferred:            0 / 0, -",
	}, "\n")
	spawn, copies := stubSpawn(0, stderr)
	got := runCLI(t, home, spawn, "agent", "push", "photograph-123", "album", "--dry-run")
	if got.code != 0 {
		t.Fatal(got)
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(got.stdout), &body); err != nil {
		t.Fatal(err, got.stdout)
	}
	if body["dryRun"] != true || body["checks"] != float64(4) || body["remote"] != "123:photograph/album" {
		t.Fatal(body)
	}
	argv := (*copies)[0]
	if !containsStr(argv, "--dry-run") || containsStr(argv, "--progress") || containsStr(argv, "sync") {
		t.Fatal(argv)
	}
	if !strings.Contains(argv[3], "/album") || !strings.HasSuffix(argv[4], ":photograph/album") {
		t.Fatal(argv[3], argv[4])
	}
}

func TestLsHidesPassword(t *testing.T) {
	home := withHome(t)
	writeCfg(t, home, sampleCfg(nil), 0o600)
	called := false
	spawn := func(string, []string, ErrSink) (SpawnRet, error) {
		called = true
		return SpawnRet{}, nil
	}
	got := runCLI(t, home, spawn, "ls")
	if called || got.code != 0 || !strings.Contains(got.stdout, "photograph-123") || strings.Contains(got.stdout, plainPass) {
		t.Fatal(called, got)
	}
}

func TestPathOutsideStillRecords(t *testing.T) {
	home := withHome(t)
	writeCfg(t, home, sampleCfg(nil), 0o600)
	spawn, copies := stubSpawn(0, "")
	got := runCLI(t, home, spawn, "push", "photograph-123", "/etc")
	if got.code != 1 || !strings.Contains(got.stdout, "outside") || len(*copies) != 0 {
		t.Fatal(got, *copies)
	}
	recs, err := ReadRuns()
	if err != nil || len(recs) != 1 || recs[0].Code != 1 {
		t.Fatal(err, recs)
	}
}

func TestPullAndCheck(t *testing.T) {
	home := withHome(t)
	writeCfg(t, home, sampleCfg(nil), 0o600)
	spawn, copies := stubSpawn(0, "")
	runCLI(t, home, spawn, "pull", "photograph-123")
	checked := runCLI(t, home, spawn, "check", "photograph-123")
	pull := (*copies)[0]
	if pull[2] != "copy" || !strings.HasPrefix(pull[3], ":webdav") || pull[4] != sampleSrc || containsStr(pull, "sync") {
		t.Fatal(pull)
	}
	if !strings.Contains(checked.stdout, "check photograph-123") || !strings.Contains(checked.stdout, "ok") {
		t.Fatal(checked.stdout)
	}
	var lists [][]string
	for _, item := range *copies {
		if item[2] == "lsjson" {
			lists = append(lists, item)
		}
	}
	if len(lists) < 2 || containsStr(lists[0], "sync") || !containsStr(lists[0], "--recursive") || containsStr(lists[0], "--progress") {
		t.Fatal(lists)
	}
}

func TestRunsNewestFirst(t *testing.T) {
	home := withHome(t)
	writeCfg(t, home, sampleCfg(nil), 0o600)
	spawn, _ := stubSpawn(0, "")
	runCLI(t, home, spawn, "push", "photograph-123")
	runCLI(t, home, spawn, "check", "photograph-123")
	got := runCLI(t, home, spawn, "runs")
	lines := strings.Split(strings.TrimSuffix(got.stdout, "\n"), "\n")
	if len(lines) < 2 || !strings.Contains(lines[0], "check") || !strings.Contains(lines[1], "push") {
		t.Fatal(got.stdout)
	}
}

func TestCorruptRun(t *testing.T) {
	home := withHome(t)
	path, err := runsPath()
	if err != nil || !strings.HasPrefix(path, home) {
		t.Fatal(err, path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{not json}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadRuns(); err == nil || !strings.Contains(err.Error(), "runs.jsonl line 1 is invalid") {
		t.Fatal(err)
	}
}

func TestPullFileAndDownload(t *testing.T) {
	home := withHome(t)
	writeCfg(t, home, sampleCfg(nil), 0o600)
	spawn, copies := stubSpawn(0, "")
	got := runCLI(t, home, spawn, "pull", "photograph-123", "album/DSC_0001.NEF")
	if got.code != 2 || !strings.Contains(got.stdout, "download one file") || len(*copies) != 0 {
		t.Fatal(got, *copies)
	}
	deps := testDeps(home, spawn)
	deps.Cwd = "/tmp/x"
	out := &nopBuf{}
	errBuf := &nopBuf{}
	deps.Stdout = out
	deps.Stderr = errBuf
	code := Execute([]string{"download", "photograph-123", "album/DSC_0746.JPG"}, deps)
	if code != 0 || (*copies)[0][2] != "copyto" || (*copies)[0][4] != "/tmp/x/DSC_0746.JPG" {
		t.Fatal(code, *copies, out.String())
	}
	if containsStr((*copies)[0], "--exclude") || !containsStr((*copies)[0], "-q") || containsStr((*copies)[0], "--progress") {
		t.Fatal((*copies)[0])
	}
}

func TestDownloadMissing(t *testing.T) {
	home := withHome(t)
	writeCfg(t, home, sampleCfg(map[string]any{"crypt": []any{samplePair(nil)}}), 0o600)
	var copies [][]string
	spawn := func(_ string, args []string, _ ErrSink) (SpawnRet, error) {
		if len(args) > 2 && args[2] == "obscure" {
			return SpawnRet{Stdout: "obscured\n"}, nil
		}
		copies = append(copies, append([]string{}, args...))
		return SpawnRet{Stdout: "[]\n"}, nil
	}
	got := runCLI(t, home, spawn, "download", "photograph-123", "album/missing.JPG")
	if got.code != 1 || !strings.Contains(got.stdout, "remote file not found") {
		t.Fatal(got)
	}
	for _, item := range copies {
		if item[2] == "copyto" {
			t.Fatal(copies)
		}
	}
}

func TestGetRequiresPath(t *testing.T) {
	home := withHome(t)
	writeCfg(t, home, sampleCfg(nil), 0o600)
	got := runCLI(t, home, stubOf(0, ""), "get", "photograph-123")
	if got.code != 2 || !strings.Contains(got.stderr, "get requires a path") {
		t.Fatal(got)
	}
}

func TestGetDecryptsLocally(t *testing.T) {
	home := withHome(t)
	writeCfg(t, home, sampleCfg(map[string]any{"crypt": []any{samplePair(nil)}}), 0o600)
	var copies [][]string
	spawn := func(_ string, args []string, _ ErrSink) (SpawnRet, error) {
		if len(args) > 2 && args[2] == "obscure" {
			return SpawnRet{Stdout: "obscured\n"}, nil
		}
		copies = append(copies, append([]string{}, args...))
		if args[2] == "lsjson" {
			return SpawnRet{Stdout: `[{"Path":"DSC_0001.NEF.bin","Size":1,"IsDir":false}]`}, nil
		}
		if args[2] == "copyto" {
			writeCopyDest(t, args)
		}
		return SpawnRet{Stderr: "Transferred:            1 / 1, 100%\n"}, nil
	}
	dest := filepath.Join(home, "DSC_0001.NEF")
	got := runCLI(t, home, spawn, "get", "photograph-123", "album/DSC_0001.NEF", dest)
	if got.code != 0 {
		t.Fatal(got)
	}
	if copies[0][2] != "lsjson" || !strings.HasPrefix(copies[0][3], ":webdav") {
		t.Fatal(copies[0])
	}
	if copies[0][indexOf(copies[0], "--retries")+1] != "1" {
		t.Fatal(copies[0])
	}
	var copytos [][]string
	for _, item := range copies {
		if item[2] == "copyto" {
			copytos = append(copytos, item)
		}
	}
	if len(copytos) != 2 || !strings.HasPrefix(copytos[0][3], ":webdav") || !strings.Contains(copytos[0][3], "DSC_0001.NEF.bin") || !containsStr(copytos[0], "-q") {
		t.Fatal(copytos)
	}
	if !strings.HasPrefix(copytos[1][3], ":crypt") || !strings.Contains(strings.Join(copytos[1], "\n"), "suffix='none'") {
		t.Fatal(copytos[1])
	}
	if _, err := os.Stat(dest); err != nil {
		t.Fatal(err)
	}
}

func TestGetRetriesNextPair(t *testing.T) {
	home := withHome(t)
	writeCfg(t, home, sampleCfg(map[string]any{"crypt": []any{
		samplePair(nil),
		samplePair(map[string]any{"name": "legacy", "key": "old", "salt": "s", "default": false}),
	}}), 0o600)
	var copies [][]string
	spawn := func(_ string, args []string, sink ErrSink) (SpawnRet, error) {
		if len(args) > 2 && args[2] == "obscure" {
			return SpawnRet{Stdout: "obscured\n"}, nil
		}
		copies = append(copies, append([]string{}, args...))
		if args[2] == "lsjson" {
			return SpawnRet{Stdout: `[{"Path":"DSC_0001.NEF","Size":1,"IsDir":false}]`}, nil
		}
		src := ""
		if len(args) > 3 {
			src = args[3]
		}
		if args[2] == "copyto" && strings.HasPrefix(src, ":crypt") {
			n := 0
			for _, item := range copies {
				if item[2] == "copyto" && strings.HasPrefix(item[3], ":crypt") {
					n++
				}
			}
			if n == 1 {
				sink.WriteErr("NOTICE: Failed to copyto: failed to authenticate decrypted block - bad password?\n")
				return SpawnRet{Code: 1, Stderr: "secretbox authentication failed\n"}, nil
			}
		}
		if args[2] == "copyto" {
			writeCopyDest(t, args)
		}
		return SpawnRet{Stderr: "Transferred:            1 / 1, 100%\n"}, nil
	}
	dest := filepath.Join(home, "DSC_0001.NEF")
	got := runCLI(t, home, spawn, "get", "photograph-123", "album/DSC_0001.NEF", dest)
	if got.code != 0 {
		t.Fatal(got)
	}
	web, crypts := 0, 0
	var first []string
	for _, item := range copies {
		if item[2] != "copyto" {
			continue
		}
		if first == nil {
			first = item
		}
		if strings.HasPrefix(item[3], ":webdav") {
			web++
		}
		if strings.HasPrefix(item[3], ":crypt") {
			crypts++
		}
	}
	if web != 1 || crypts != 2 || first[indexOf(first, "--retries")+1] != "1" {
		t.Fatal(web, crypts, first)
	}
	if _, err := os.Stat(dest); err != nil {
		t.Fatal(err)
	}
}

func TestCheckBinAndLsLimit(t *testing.T) {
	home := withHome(t)
	writeCfg(t, home, sampleCfg(map[string]any{"crypt": []any{samplePair(nil)}}), 0o600)
	spawn := func(_ string, args []string, _ ErrSink) (SpawnRet, error) {
		if len(args) > 2 && args[2] == "obscure" {
			return SpawnRet{Stdout: "obscured\n"}, nil
		}
		root := ""
		if len(args) > 3 {
			root = args[3]
		}
		if strings.HasPrefix(root, "/") {
			return SpawnRet{Stdout: `[{"Path":"a.NEF","Size":10,"IsDir":false},{"Path":"b.NEF","Size":20,"IsDir":false}]`}, nil
		}
		return SpawnRet{Stdout: `[{"Path":"a.NEF","Size":10,"IsDir":false},{"Path":"b.NEF.bin","Size":20,"IsDir":false},{"Path":"cloud-only.JPG","Size":3,"IsDir":false}]`}, nil
	}
	got := runCLI(t, home, spawn, "check", "photograph-123")
	if got.code != 0 || !strings.Contains(got.stdout, "ok-bin") || !strings.Contains(got.stdout, "extra") || !strings.Contains(got.stdout, "cloud-only.JPG") {
		t.Fatal(got)
	}
	listed := runCLI(t, home, spawn, "ls", "photograph-123")
	if listed.code != 0 || !strings.Contains(listed.stdout, "a.NEF") || !strings.Contains(listed.stdout, "b.NEF") || !strings.Contains(listed.stdout, "limit=10") {
		t.Fatal(listed.stdout)
	}
	one := runCLI(t, home, spawn, "ls", "photograph-123", "-n", "1")
	if !strings.Contains(one.stdout, "showing 1/3") || !strings.Contains(one.stdout, "a.NEF") || strings.Contains(one.stdout, "b.NEF") {
		t.Fatal(one.stdout)
	}
}

func TestPullFillsBin(t *testing.T) {
	home := withHome(t)
	writeCfg(t, home, sampleCfg(map[string]any{"crypt": []any{samplePair(nil)}}), 0o600)
	spawn, copies := stubSpawn(0, "Transferred:            1 / 1, 100%\n")
	got := runCLI(t, home, spawn, "pull", "photograph-123")
	if got.code != 0 || len(*copies) != 2 {
		t.Fatal(got, *copies)
	}
	if !strings.Contains(strings.Join((*copies)[0], "\n"), "suffix='none'") || !containsStr((*copies)[0], "*.bin") {
		t.Fatal((*copies)[0])
	}
	if !strings.Contains(strings.Join((*copies)[1], "\n"), "suffix='.bin'") || !containsStr((*copies)[1], "--ignore-existing") {
		t.Fatal((*copies)[1])
	}
}

func TestDownloadVerbose(t *testing.T) {
	home := withHome(t)
	writeCfg(t, home, sampleCfg(map[string]any{"crypt": []any{samplePair(nil)}}), 0o600)
	deps := testDeps(home, nil)
	deps.Cwd = home
	out := &nopBuf{}
	errBuf := &nopBuf{}
	deps.Stdout = out
	deps.Stderr = errBuf
	deps.Spawn = func(_ string, args []string, sink ErrSink) (SpawnRet, error) {
		if len(args) > 2 && args[2] == "obscure" {
			return SpawnRet{Stdout: "obscured\n"}, nil
		}
		if args[2] == "lsjson" {
			return SpawnRet{Stdout: `[{"Path":"DSC_0746.JPG.bin","Size":1,"IsDir":false}]`}, nil
		}
		if args[2] == "copyto" {
			writeCopyDest(t, args)
			sink.WriteErr("NOTICE: copied\n")
		}
		return SpawnRet{}, nil
	}
	code := Execute([]string{"download", "photograph-123", "album/DSC_0746.JPG.bin", "-v"}, deps)
	if code != 0 {
		t.Fatal(code, out.String(), errBuf.String())
	}
	text := errBuf.String()
	if !strings.Contains(text, "fetch ") || !strings.Contains(text, "decrypt v1") || !strings.Contains(text, "NOTICE: copied") {
		t.Fatal(text)
	}
}

func TestCompleteAndFish(t *testing.T) {
	home := withHome(t)
	writeCfg(t, home, sampleCfg(nil), 0o600)
	root := runCLI(t, home, stubOf(0, ""), "__complete", "")
	if !strings.Contains(root.stdout, "get") {
		t.Fatal(root.stdout)
	}
	tasks := runCLI(t, home, stubOf(0, ""), "__complete", "get", "")
	if !strings.Contains(tasks.stdout, "photograph-123") {
		t.Fatal(tasks.stdout)
	}
	fish := runCLI(t, home, stubOf(0, ""), "completion", "fish")
	if !strings.Contains(fish.stdout, "push pull check get download ls") {
		t.Fatal(fish.stdout)
	}
}

func TestMissingConfigUsage(t *testing.T) {
	home := withHome(t)
	got := runCLI(t, home, stubOf(0, ""), "push")
	if got.code != 2 || !strings.Contains(got.stderr, "config missing") {
		t.Fatal(got)
	}
}

func TestVersionDoesNotReadConfig(t *testing.T) {
	home := withHome(t)
	before := dirSnap(t, filepath.Join(home, ".config"))
	got := runCLI(t, home, stubOf(0, ""), "version")
	if got.code != 0 || strings.TrimSpace(got.stdout) != "rclone-cli 0.1.0" {
		t.Fatal(got)
	}
	if dirSnap(t, filepath.Join(home, ".config")) != before {
		t.Fatal("version touched config")
	}
}

func TestDoctorMissingAndInvalid(t *testing.T) {
	home := withHome(t)
	cfgRoot := filepath.Join(home, ".config")
	before := dirSnap(t, cfgRoot)
	got := runCLI(t, home, stubOf(0, ""), "doctor")
	if got.code == 0 || got.code > 128 || !strings.Contains(got.stdout, "fail config:") || got.stderr != "" {
		t.Fatalf("%+v", got)
	}
	if dirSnap(t, cfgRoot) != before {
		t.Fatal("doctor created config")
	}
	writeCfg(t, home, sampleCfg(nil), 0o644)
	got = runCLI(t, home, stubOf(0, ""), "doctor")
	if got.code == 0 || !strings.Contains(got.stdout, "mode must be 600") {
		t.Fatal(got)
	}
	writeCfg(t, home, sampleCfg(nil), 0o600)
	info, err := os.Stat(filepath.Join(home, ".config", "rclone-cli", "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	got = runCLI(t, home, stubOf(0, ""), "doctor")
	if got.code != 0 || !strings.Contains(got.stdout, "pass config") {
		t.Fatal(got)
	}
	after, err := os.Stat(filepath.Join(home, ".config", "rclone-cli", "config.json"))
	if err != nil || after.Mode().Perm() != info.Mode().Perm() {
		t.Fatal("doctor chmod config", err)
	}
}

func TestConfigFlagAndStateGuard(t *testing.T) {
	home := withHome(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "custom.json")
	if err := os.WriteFile(path, []byte(mustJSON(t, sampleCfg(nil))), 0o600); err != nil {
		t.Fatal(err)
	}
	got := runCLI(t, home, stubOf(0, ""), "-c", path, "ls")
	if got.code != 0 || !strings.Contains(got.stdout, "photograph-123") || strings.Contains(got.stdout, plainPass) {
		t.Fatal(got)
	}
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, ".config", "state"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	got = runCLI(t, home, stubOf(0, ""), "push", "photograph-123")
	if got.code == 0 || !strings.Contains(got.stderr, "config") {
		t.Fatal(got)
	}
}

func TestHelp(t *testing.T) {
	home := withHome(t)
	got := runCLI(t, home, stubOf(0, ""), "--help")
	if got.code != 0 || !strings.Contains(got.stdout, "no default task") || !strings.Contains(got.stdout, "completion fish") {
		t.Fatal(got)
	}
}

func stubOf(code int, stderr string) SpawnFn {
	spawn, _ := stubSpawn(code, stderr)
	return spawn
}

func writeCopyDest(t *testing.T, args []string) {
	t.Helper()
	if len(args) < 5 || args[4] == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(args[4]), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(args[4], []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func dirSnap(t *testing.T, root string) string {
	t.Helper()
	var names []string
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		names = append(names, rel)
		return nil
	})
	return strings.Join(names, "\n")
}
