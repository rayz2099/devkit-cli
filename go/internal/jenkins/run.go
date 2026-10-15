package jenkins

import (
	"fmt"
	"io"

	"git.internal.linran.top/linran/devkit-cli/internal/doctor"
)

func dispatch(w io.Writer, args []string) error {
	parsed, err := parseArgs(args)
	if err != nil {
		return err
	}
	switch parsed.kind {
	case "help":
		_, err := io.WriteString(w, renderText(helpText(parsed.topic)))
		return err
	case "completion-fish":
		_, err := io.WriteString(w, fishScript())
		return err
	case "version":
		_, err := io.WriteString(w, versionLine)
		return err
	case "doctor":
		return runDoctor(w, parsed.config)
	case "complete":
		_, err := io.WriteString(w, completeLines(parsed.tokens, parsed.current))
		return err
	default:
		return runRemote(w, parsed)
	}
}

// runDoctor 只检查配置文件能不能解析、必填字段是否有效。不联网，失败行写到 stdout。
func runDoctor(w io.Writer, flag string) error {
	err := checkCfg(flag)
	return doctor.Write(w, []doctor.Item{{Name: "config", Err: err}})
}

func checkCfg(flag string) error {
	cfg, err := LoadFileCfg(flag)
	if err != nil {
		return err
	}
	if _, err := PickProfile(cfg, ""); err != nil {
		return err
	}
	return nil
}

func runRemote(w io.Writer, parsed cmd) error {
	cfg, err := LoadFileCfg(parsed.config)
	if err != nil {
		return err
	}
	rt, err := resolveRT(cfg, parsed.audience, parsed.profile)
	if err != nil {
		return err
	}
	client := newClient(rt)
	text, err := renderCmd(parsed, rt, client)
	if err != nil {
		return err
	}
	_, err = io.WriteString(w, text)
	return err
}

func renderCmd(parsed cmd, rt runtime, client *Client) (string, error) {
	switch parsed.kind {
	case "job-ls":
		jobs, err := client.listJobs(parsed.folder)
		if err != nil {
			return "", err
		}
		return pickBody(rt.aud, jobLsText(jobs), jobs)
	case "job-view":
		job, err := client.viewJob(parsed.job)
		if err != nil {
			return "", err
		}
		return pickBody(rt.aud, jobViewText(job), job)
	case "run-ls":
		items, err := client.listRuns(parsed.job, parsed.limit)
		if err != nil {
			return "", err
		}
		return pickBody(rt.aud, runLsText(items), items)
	case "run-view":
		info, err := client.viewRun(parsed.job, parsed.build)
		if err != nil {
			return "", err
		}
		if parsed.slim {
			if rt.aud == "agent" {
				return renderJSON(info.RunStatus)
			}
			line := fmt.Sprintf("%s #%d %s", info.Job, info.Build, info.Result)
			return renderText(line), nil
		}
		if rt.aud == "agent" {
			return renderJSON(info)
		}
		return renderText(humanRun(info)), nil
	case "run-start":
		out, err := client.startRun(parsed.job)
		if err != nil {
			return "", err
		}
		return pickBody(rt.aud, triggerText("triggered "+out.Job, out.QueueLocation), out)
	case "run-rerun":
		out, err := client.rerun(parsed.job, parsed.build)
		if err != nil {
			return "", err
		}
		head := fmt.Sprintf("reran %s from #%s", out.Job, parsed.build)
		return pickBody(rt.aud, triggerText(head, out.QueueLocation), out)
	case "run-cancel":
		if err := client.cancelRun(parsed.job, parsed.build); err != nil {
			return "", err
		}
		body := CancelOut{JobPath: parsed.job, Build: parsed.build, Action: "stop", Status: "requested"}
		text := fmt.Sprintf("cancel requested %s #%s\n", parsed.job, parsed.build)
		return pickBody(rt.aud, text, body)
	case "log":
		text, err := client.readLog(parsed.job, parsed.build, parsed.tail)
		if err != nil {
			return "", err
		}
		return renderText(text), nil
	case "queue-ls":
		items, err := client.listQueue()
		if err != nil {
			return "", err
		}
		return pickBody(rt.aud, queueLsText(items), items)
	default:
		return "", fmt.Errorf("unhandled command: %s", parsed.kind)
	}
}

func humanRun(info RunInfo) string {
	lines := []string{fmt.Sprintf("%s #%d %s", info.Job, info.Build, info.Result)}
	if info.URL != "" {
		lines = append(lines, info.URL)
	}
	if joined := joinCauses(info.Causes); joined != "" {
		lines = append(lines, joined)
	}
	return joinLines(lines)
}

func joinCauses(causes []string) string {
	out := ""
	for i, cause := range causes {
		if i > 0 {
			out += "; "
		}
		out += cause
	}
	return out
}

func joinLines(lines []string) string {
	out := ""
	for i, line := range lines {
		if i > 0 {
			out += "\n"
		}
		out += line
	}
	return out
}

func triggerText(head string, loc *string) string {
	if loc != nil {
		return head + "\n" + *loc + "\n"
	}
	return head + "\n"
}
