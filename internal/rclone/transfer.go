package rclone

import (
	"io"
	"os"
	"path/filepath"
	"strings"
)

type attemptOut struct {
	args      []string
	code      int
	stderr    string
	bytes     *int64
	checks    *int64
	transfers *int64
}

type xferEnv struct {
	bin   string
	cfg   string
	plan  *Plan
	sec   Secrets
	spawn SpawnFn
	sink  ErrSink
}

func runAttempts(env xferEnv) (attemptOut, error) {
	if (env.plan.Action == "get" || env.plan.Action == "download") && usesLegacyBin(env.plan) {
		return runGet(env)
	}
	if env.plan.Action == "pull" && usesLegacyBin(env.plan) {
		return runPull(env)
	}
	return spawnOnce(env, "none", nil, "")
}

func usesLegacyBin(plan *Plan) bool {
	return plan.Target.HasCrypt && plan.NameEnc == "off"
}

func runGet(env xferEnv) (attemptOut, error) {
	names := CryptOrder(env.plan.Target)
	if len(names) == 0 {
		return attemptOut{}, fail(1, "targets.%s.crypt requires a default key pair", env.plan.Target.Name)
	}
	suffix, err := probeSuffix(env)
	if err != nil {
		return attemptOut{}, err
	}
	if suffix == "" {
		return attemptOut{code: 1, stderr: "remote file not found: " + env.plan.Rel, args: []string{}}, nil
	}
	blobRel := env.plan.Rel
	if suffix == ".bin" {
		blobRel = env.plan.Rel + ".bin"
	}
	blob := filepath.Base(blobRel)
	tmpDir, err := os.MkdirTemp("", "rclone-cli-")
	if err != nil {
		return attemptOut{}, fail(1, "temp dir: %s", err.Error())
	}
	defer os.RemoveAll(tmpDir)
	tmpIn := filepath.Join(tmpDir, blob)
	remote := BuildPlainRemote(env.plan.Target, env.sec.Pass, blobRel)
	if env.plan.Verbose {
		env.sink.WriteErr("fetch " + blobRel + "\n")
	}
	fetched, err := spawnCopy(env, remote, tmpIn, nil)
	if err != nil {
		return attemptOut{}, err
	}
	if fetched.code != 0 || env.plan.DryRun {
		return fetched, nil
	}
	return unlockLocal(env, names, tmpDir, blob, fetched)
}

func unlockLocal(env xferEnv, names []string, tmpDir, blob string, fetched attemptOut) (attemptOut, error) {
	last := fetched
	for _, pair := range names {
		wire, ok := findWire(env.sec, pair)
		if !ok {
			return attemptOut{}, fail(1, "targets.%s.crypt.%s requires key and salt", env.plan.Target.Name, pair)
		}
		tmpOut := filepath.Join(tmpDir, "out-"+pair)
		remote := BuildLocalCrypt(tmpDir, wire, blob)
		if env.plan.Verbose {
			env.sink.WriteErr("decrypt " + pair + "\n")
		}
		step, err := spawnCopy(env, remote, tmpOut, nil)
		if err != nil {
			return attemptOut{}, err
		}
		last = step
		if step.code != 0 {
			continue
		}
		// 临时目录和目标可能不在同一卷，rename 会 EXDEV。复制再删才是跨卷落地。
		if err := os.MkdirAll(filepath.Dir(env.plan.Local), 0o755); err != nil {
			return attemptOut{}, fail(1, "dest dir: %s", err.Error())
		}
		if err := copyFile(tmpOut, env.plan.Local); err != nil {
			return attemptOut{}, err
		}
		if err := os.Remove(tmpOut); err != nil {
			return attemptOut{}, fail(1, "temp file: %s", err.Error())
		}
		return mergeAttempts(fetched, last), nil
	}
	return last, nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fail(1, "copy: %s", err.Error())
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return fail(1, "copy: %s", err.Error())
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode().Perm())
	if err != nil {
		return fail(1, "copy: %s", err.Error())
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return fail(1, "copy: %s", err.Error())
	}
	return nil
}

func probeSuffix(env xferEnv) (string, error) {
	name := env.plan.Rel
	parent := ""
	if i := strings.LastIndex(env.plan.Rel, "/"); i >= 0 {
		name = env.plan.Rel[i+1:]
		parent = env.plan.Rel[:i]
	}
	remote := BuildPlainRemote(env.plan.Target, env.sec.Pass, parent)
	args := buildListArgv(env.plan, env.cfg, remote, true)
	found, err := listFiles(env.bin, args, env.spawn)
	if err != nil {
		return "", err
	}
	names := map[string]bool{}
	for _, item := range found {
		base := item.Path
		if i := strings.LastIndex(item.Path, "/"); i >= 0 {
			base = item.Path[i+1:]
		}
		names[base] = true
	}
	if names[name] {
		return "none", nil
	}
	if names[name+".bin"] {
		return ".bin", nil
	}
	return "", nil
}

func runPull(env xferEnv) (attemptOut, error) {
	names := CryptOrder(env.plan.Target)
	if len(names) == 0 {
		return attemptOut{}, fail(1, "targets.%s.crypt requires a default key pair", env.plan.Target.Name)
	}
	var acc *attemptOut
	for index, pair := range names {
		extra := []string{"--exclude", "*.bin"}
		if index != 0 {
			extra = []string{"--ignore-existing", "--exclude", "*.bin"}
		}
		first, err := spawnOnce(env, "none", extra, pair)
		if err != nil {
			return attemptOut{}, err
		}
		step, err := fillBin(env, first, pair)
		if err != nil {
			return attemptOut{}, err
		}
		if acc == nil {
			acc = &step
			continue
		}
		merged := mergeAttempts(*acc, step)
		acc = &merged
	}
	if acc == nil {
		return attemptOut{}, fail(1, "targets.%s.crypt requires a default key pair", env.plan.Target.Name)
	}
	return *acc, nil
}

func fillBin(env xferEnv, first attemptOut, pair string) (attemptOut, error) {
	if first.code != 0 {
		if isMissingRemote(first.stderr, first.code) {
			return spawnOnce(env, ".bin", nil, pair)
		}
		return first, nil
	}
	second, err := spawnOnce(env, ".bin", []string{"--ignore-existing"}, pair)
	if err != nil {
		return attemptOut{}, err
	}
	if second.code != 0 && isMissingRemote(second.stderr, second.code) {
		return first, nil
	}
	if second.code != 0 {
		return second, nil
	}
	return mergeAttempts(first, second), nil
}

func spawnOnce(env xferEnv, suffix string, extra []string, pair string) (attemptOut, error) {
	remote, err := BuildRemote(env.plan.Target, env.sec, env.plan.Rel, env.plan.NameEnc, suffix, pair)
	if err != nil {
		return attemptOut{}, err
	}
	return spawnCopy(env, remote, env.plan.Local, extra)
}

func spawnCopy(env xferEnv, remote, dest string, extra []string) (attemptOut, error) {
	args, err := BuildArgv(env.plan, env.cfg, remote, dest)
	if err != nil {
		return attemptOut{}, err
	}
	args = append(args, extra...)
	ret, err := env.spawn(env.bin, args, env.sink)
	if err != nil {
		return attemptOut{}, err
	}
	parsed := parseStats(ret.Stderr)
	return attemptOut{
		args: args, code: ret.Code, stderr: ret.Stderr,
		bytes: parsed.bytes, checks: parsed.checks, transfers: parsed.transfers,
	}, nil
}

func mergeAttempts(first, second attemptOut) attemptOut {
	code := second.code
	if first.code == 0 || second.code == 0 {
		code = 0
	}
	stderr := first.stderr
	if second.stderr != "" {
		if stderr != "" {
			stderr += "\n"
		}
		stderr += second.stderr
	}
	out := attemptOut{args: first.args, code: code, stderr: stderr}
	out.bytes = sumOpt(first.bytes, second.bytes)
	out.checks = sumOpt(first.checks, second.checks)
	out.transfers = sumOpt(first.transfers, second.transfers)
	return out
}

func sumOpt(left, right *int64) *int64 {
	if left == nil && right == nil {
		return nil
	}
	n := int64(0)
	if left != nil {
		n += *left
	}
	if right != nil {
		n += *right
	}
	return &n
}

func isMissingRemote(stderr string, code int) bool {
	if code == 0 {
		return false
	}
	text := strings.ToLower(stderr)
	return strings.Contains(text, "not found") || strings.Contains(text, "directory not found") || strings.Contains(text, "object not found")
}
