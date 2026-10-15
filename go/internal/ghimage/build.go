package ghimage

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode"
)

type buildResult struct {
	status string
	runURL string
}

type buildIn struct {
	local   repo
	img     image
	alias   string
	version string
	cfg     config
}

type runInfo struct {
	DatabaseID   int64   `json:"databaseId"`
	Status       string  `json:"status"`
	Conclusion   *string `json:"conclusion"`
	URL          string  `json:"url"`
	DisplayTitle string  `json:"displayTitle"`
}

func workflowName(kind string) (string, bool) {
	switch kind {
	case "mirror":
		return "sync-image.yml", true
	case "dockerfile":
		return "build-dockerfile.yml", true
	default:
		return "", false
	}
}

// runBuild 触发 workflow_dispatch 并等到结束。
// 请求号放进参数，是为了从最近的 run 列表里认出这一次，而不是误绑并发的同名 tag。
func runBuild(rt runtime, in buildIn, stdout, stderr io.Writer) (buildResult, error) {
	workflow, ok := workflowName(in.img.kind)
	if !ok {
		return buildResult{}, fmt.Errorf("invalid image record: %s", in.alias)
	}
	reqID, err := rt.id()
	if err != nil {
		return buildResult{}, err
	}
	_, err = mustRun(rt, "gh", triggerArgs(in, workflow, reqID), in.local.root)
	if err != nil {
		return buildResult{}, err
	}
	deadline := rt.now().Add(time.Duration(in.cfg.timeoutSeconds) * time.Second)
	var found *runInfo
	last := ""
	for rt.now().Before(deadline) {
		run, err := findRun(rt, in.local, workflow, reqID)
		if err != nil {
			return buildResult{}, err
		}
		if run == nil {
			rt.sleep(2 * time.Second)
			continue
		}
		found = run
		if run.Status != last {
			fmt.Fprintf(stdout, "action: %s\n", run.Status)
			fmt.Fprintf(stdout, "run: %s\n", run.URL)
			last = run.Status
		}
		if run.Status == "completed" {
			if run.Conclusion != nil && *run.Conclusion == "success" {
				return buildResult{status: "done", runURL: run.URL}, nil
			}
			if err := printFailedLog(rt, in.local, run.DatabaseID, stderr); err != nil {
				return buildResult{}, err
			}
			return buildResult{status: "failed", runURL: run.URL}, nil
		}
		rt.sleep(3 * time.Second)
	}
	url := fmt.Sprintf("https://github.com/%s/actions/workflows/%s", in.local.repository, workflow)
	if found != nil {
		url = found.URL
	}
	return buildResult{status: "timeout", runURL: url}, nil
}

func triggerArgs(in buildIn, workflow, reqID string) []string {
	args := []string{
		"workflow", "run", workflow,
		"--repo", in.local.repository,
		"--ref", in.local.defaultBranch,
	}
	if in.img.kind == "mirror" {
		args = append(args, "-f", "source="+in.img.source)
	} else {
		args = append(args, "-f", "script="+in.img.script)
	}
	args = append(args,
		"-f", "alias="+in.alias,
		"-f", "version="+in.version,
		"-f", "request_id="+reqID,
		"-f", "registry="+in.cfg.registry,
		"-f", "namespace="+in.cfg.namespace,
	)
	return args
}

func findRun(rt runtime, local repo, workflow, reqID string) (*runInfo, error) {
	raw, err := mustRun(rt, "gh", []string{
		"run", "list",
		"--repo", local.repository,
		"--workflow", workflow,
		"--event", "workflow_dispatch",
		"--limit", "30",
		"--json", "databaseId,status,conclusion,url,displayTitle",
	}, local.root)
	if err != nil {
		return nil, err
	}
	var runs []runInfo
	if err := json.Unmarshal([]byte(raw), &runs); err != nil {
		return nil, fmt.Errorf("invalid gh run list: %w", err)
	}
	for i := range runs {
		if strings.Contains(runs[i].DisplayTitle, reqID) {
			return &runs[i], nil
		}
	}
	return nil, nil
}

func printFailedLog(rt runtime, local repo, id int64, stderr io.Writer) error {
	out, err := rt.run("gh", []string{
		"run", "view", fmt.Sprint(id),
		"--repo", local.repository,
		"--log-failed",
	}, local.root)
	if err != nil {
		return err
	}
	detail := out.stdout
	if detail == "" {
		detail = out.stderr
	}
	detail = strings.TrimRightFunc(detail, unicode.IsSpace)
	if detail != "" {
		fmt.Fprintln(stderr, detail)
	}
	return nil
}
