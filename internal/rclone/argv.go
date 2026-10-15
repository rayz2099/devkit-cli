package rclone

import "strconv"

type retryBudget struct {
	retries float64
	sleep   string
	low     float64
}

// BuildArgv 只拼 copy 或 copyto。sync 会删掉另一侧多出来的文件，废片和旧密文都不能被清掉。
func BuildArgv(plan *Plan, cfgFile, remote, dest string) ([]string, error) {
	ends, err := orderedEnds(plan, remote, dest)
	if err != nil {
		return nil, err
	}
	kind, err := verb(plan)
	if err != nil {
		return nil, err
	}
	args := []string{"--config", cfgFile, kind, ends.src, ends.dst}
	// copyto 不能带 filter。单文件加上 --exclude 会直接 CRITICAL，不是 not found。
	if kind != "copyto" {
		for _, pattern := range plan.Excludes {
			args = append(args, "--exclude", pattern)
		}
	}
	if plan.Target.SkipLinks {
		args = append(args, "--skip-links")
	}
	if plan.Target.SizeOnly {
		args = append(args, "--size-only")
	}
	budget := retryOf(plan)
	args = append(args,
		"--transfers", numStr(plan.Transfers),
		"--checkers", numStr(plan.Target.Checkers),
		"--retries", numStr(budget.retries),
		"--retries-sleep", budget.sleep,
		"--low-level-retries", numStr(budget.low),
		"--timeout", plan.Timeout,
		"--contimeout", plan.Target.Contimeout,
		"--tpslimit", numStr(plan.Tpslimit),
	)
	if plan.DryRun {
		args = append(args, "--dry-run")
	}
	if plan.Verbose {
		args = append(args, "-v")
	} else if plan.Action == "get" || plan.Action == "download" {
		args = append(args, "-q")
	}
	if plan.Progress {
		args = append(args, "--progress")
	}
	return args, nil
}

func retryOf(plan *Plan) retryBudget {
	// get/download 会自己换密钥对。rclone 再跑 10 次 15s 会把一次解错钥匙拖成几分钟。
	if plan.Action == "get" || plan.Action == "download" {
		return retryBudget{retries: 1, sleep: "1s", low: 1}
	}
	return retryBudget{
		retries: plan.Target.Retries,
		sleep:   plan.Target.RetriesSleep,
		low:     plan.Target.LowLevelRetries,
	}
}

func verb(plan *Plan) (string, error) {
	if plan.Action == "check" {
		return "", fail(1, "check lists files; it does not spawn rclone check")
	}
	if plan.Action == "get" || plan.Action == "download" {
		return "copyto", nil
	}
	return "copy", nil
}

type ends struct {
	src string
	dst string
}

func orderedEnds(plan *Plan, remote, dest string) (ends, error) {
	if plan.Action == "pull" || plan.Action == "get" || plan.Action == "download" {
		return ends{src: remote, dst: dest}, nil
	}
	return ends{src: dest, dst: remote}, nil
}

func numStr(n float64) string {
	if n == float64(int64(n)) {
		return strconv.FormatInt(int64(n), 10)
	}
	return strconv.FormatFloat(n, 'f', -1, 64)
}
