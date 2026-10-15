package td

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"git.internal.linran.top/linran/devkit-cli/internal/cfgfile"
	"git.internal.linran.top/linran/devkit-cli/internal/doctor"
)

// quietExit 表示 stdout 上已经写过原因, 调用方不要再补一行错误.
type quietExit struct{}

func (quietExit) Error() string { return "doctor failed" }

const versionText = "td-cli 0.1.0"

func dispatch(args []string, stdout, stderr io.Writer, now time.Time) error {
	cmd, err := parseArgs(args)
	if err != nil {
		return err
	}
	if cmd.Kind == kindDoctor {
		return runDoctor(cmd, stdout)
	}
	var buf bytes.Buffer
	if err := runCmd(cmd, &buf, stderr, now); err != nil {
		return err
	}
	_, err = io.Copy(stdout, &buf)
	return err
}

func runCmd(cmd Cmd, stdout, stderr io.Writer, now time.Time) error {
	switch cmd.Kind {
	case kindFish:
		_, err := io.WriteString(stdout, fishScript())
		return err
	case kindComplete:
		_, err := io.WriteString(stdout, completeLines(cmd.Tokens, cmd.Current))
		return err
	case kindHelp:
		text := helpText(cmd.Topic)
		out, err := emitValue(cmd.Audience, usageBody{Usage: text}, text)
		if err != nil {
			return err
		}
		_, err = io.WriteString(stdout, out)
		return err
	case kindVersion:
		_, err := io.WriteString(stdout, versionText+"\n")
		return err
	case kindLogin:
		return doLogin(cmd, stdout)
	case kindStatus:
		return doStatus(cmd, stdout)
	case kindProjects:
		return doProjects(cmd, stdout)
	case kindEvents:
		return doEvents(cmd, stdout, now)
	case kindProps:
		return doProps(cmd, stdout, now)
	case kindReportLs:
		return doReportLs(cmd, stdout, now)
	case kindReportGet:
		return doReportGet(cmd, stdout, now)
	case kindQueryEvent, kindQueryProp, kindQueryRun, kindQueryReport:
		return doQuery(cmd, stdout, stderr, now)
	default:
		return fmt.Errorf("unknown command: %s", cmd.Kind)
	}
}

func doLogin(cmd Cmd, stdout io.Writer) error {
	profile, err := loadProfile(cmd)
	if err != nil {
		return err
	}
	session, err := connect(profile)
	if err != nil {
		return err
	}
	view := loginView{Profile: profile.Name, URL: session.Host, Username: profile.Username, OK: true}
	text := fmt.Sprintf("profile\t%s\nurl\t%s\nusername\t%s\nok\ttrue\n", view.Profile, view.URL, view.Username)
	out, err := emitValue(cmd.Audience, view, text)
	if err != nil {
		return err
	}
	_, err = io.WriteString(stdout, out)
	return err
}

func doStatus(cmd Cmd, stdout io.Writer) error {
	profile, err := loadProfile(cmd)
	if err != nil {
		return err
	}
	view := statusView{Profile: profile.Name, URL: profile.URL, Username: profile.Username}
	text := fmt.Sprintf("profile\t%s\nurl\t%s\nusername\t%s\n", view.Profile, view.URL, view.Username)
	out, err := emitValue(cmd.Audience, view, text)
	if err != nil {
		return err
	}
	_, err = io.WriteString(stdout, out)
	return err
}

func doProjects(cmd Cmd, stdout io.Writer) error {
	session, err := openSession(cmd)
	if err != nil {
		return err
	}
	data, err := listProjects(session)
	if err != nil {
		return err
	}
	return emitRows(cmd, stdout, []string{"ID", "NAME"}, data, projectRows)
}

func doEvents(cmd Cmd, stdout io.Writer, now time.Time) error {
	session, err := openSession(cmd)
	if err != nil {
		return err
	}
	data, err := listEvents(session, cmd.Project, now)
	if err != nil {
		return err
	}
	return emitRows(cmd, stdout, []string{"EVENT", "DESC"}, data, eventRows)
}

func doProps(cmd Cmd, stdout io.Writer, now time.Time) error {
	session, err := openSession(cmd)
	if err != nil {
		return err
	}
	data, err := listProps(session, cmd.Project, cmd.Event, now)
	if err != nil {
		return err
	}
	return emitRows(cmd, stdout, []string{"PROP", "DESC", "TABLE"}, data, propRows)
}

func doReportLs(cmd Cmd, stdout io.Writer, now time.Time) error {
	session, err := openSession(cmd)
	if err != nil {
		return err
	}
	data, err := listReports(session, cmd.Project, now)
	if err != nil {
		return err
	}
	return emitRows(cmd, stdout, []string{"ID", "NAME", "MODEL"}, data, reportRows)
}

func doReportGet(cmd Cmd, stdout io.Writer, now time.Time) error {
	session, err := openSession(cmd)
	if err != nil {
		return err
	}
	data, err := getReport(session, cmd.Project, cmd.ReportID, now)
	if err != nil {
		return err
	}
	text, err := jsonText(data)
	if err != nil {
		return err
	}
	out, err := emitRaw(cmd.Audience, data, text)
	if err != nil {
		return err
	}
	_, err = io.WriteString(stdout, out)
	return err
}

func emitRows(cmd Cmd, stdout io.Writer, headers []string, data json.RawMessage, rowsOf func(json.RawMessage) ([][]string, error)) error {
	rows, err := rowsOf(data)
	if err != nil {
		return err
	}
	text, err := rowsOrJSON(headers, rows, data)
	if err != nil {
		return err
	}
	out, err := emitRaw(cmd.Audience, data, text)
	if err != nil {
		return err
	}
	_, err = io.WriteString(stdout, out)
	return err
}

func doQuery(cmd Cmd, stdout, stderr io.Writer, now time.Time) error {
	session, err := openSession(cmd)
	if err != nil {
		return err
	}
	projectID, err := asInt(cmd.Project, "--project")
	if err != nil {
		return err
	}
	model, qp, err := buildQuery(session, cmd, now)
	if err != nil {
		return err
	}
	if err := assertReadModel(model); err != nil {
		return err
	}
	prefix, err := modelPrefix(model)
	if err != nil {
		return err
	}
	var progress func(string) error
	if cmd.Audience == "human" {
		progress = func(line string) error {
			_, err := fmt.Fprintf(stderr, "%s\n", line)
			return err
		}
	}
	result, err := runQuery(queryIn{
		Host:       session.Host,
		Token:      session.Token,
		BFF:        session.BFF,
		Prefix:     prefix,
		ProjectID:  projectID,
		EventModel: model,
		QP:         qp,
		Timeout:    cmd.Timeout,
		OnProgress: progress,
	})
	if err != nil {
		return err
	}
	text, err := queryText(result)
	if err != nil {
		return err
	}
	out, err := emitRaw(cmd.Audience, result, text)
	if err != nil {
		return err
	}
	_, err = io.WriteString(stdout, out)
	return err
}

func buildQuery(session Session, cmd Cmd, now time.Time) (int, json.RawMessage, error) {
	switch cmd.Kind {
	case kindQueryRun:
		if err := assertReadModel(cmd.Model); err != nil {
			return 0, nil, err
		}
		qp, err := readQP(*cmd.QP)
		if err != nil {
			return 0, nil, err
		}
		return cmd.Model, qp, nil
	case kindQueryReport:
		data, err := getReport(session, cmd.Project, cmd.ReportID, now)
		if err != nil {
			return 0, nil, err
		}
		var model *int
		if cmd.ModelSet {
			m := cmd.Model
			model = &m
		}
		return takeReport(data, model)
	}
	if cmd.QP != nil {
		model := 3
		if cmd.Kind == kindQueryEvent {
			model = 0
		}
		qp, err := readQP(*cmd.QP)
		if err != nil {
			return 0, nil, err
		}
		return model, qp, nil
	}
	if cmd.Kind == kindQueryEvent {
		span, err := resolveSpan(cmd.Recent, cmd.From, cmd.To, true, now)
		if err != nil {
			return 0, nil, err
		}
		if span == nil {
			return 0, nil, fmt.Errorf("event query needs --recent or --from/--to")
		}
		qp, err := eventQP(eventIn{
			EventName: cmd.EventName,
			Analysis:  cmd.Analysis,
			Span:      *span,
			Particle:  cmd.Particle,
			Group:     cmd.Group,
		})
		if err != nil {
			return 0, nil, err
		}
		return 0, qp, nil
	}
	span, err := resolveSpan(cmd.Recent, cmd.From, cmd.To, false, now)
	if err != nil {
		return 0, nil, err
	}
	qp, err := propertyQP(propIn{
		Analysis: cmd.Analysis,
		Quota:    cmd.Quota,
		Group:    cmd.Group,
		Span:     span,
	})
	if err != nil {
		return 0, nil, err
	}
	return 3, qp, nil
}

func readQP(path string) (json.RawMessage, error) {
	buf, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	out, err := compactJSON(buf)
	if err != nil {
		return nil, fmt.Errorf("invalid qp file %s", path)
	}
	return out, nil
}

func compactJSON(buf []byte) (json.RawMessage, error) {
	var out bytes.Buffer
	if err := json.Compact(&out, buf); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func openSession(cmd Cmd) (Session, error) {
	profile, err := loadProfile(cmd)
	if err != nil {
		return Session{}, err
	}
	return connect(profile)
}

func loadProfile(cmd Cmd) (Profile, error) {
	path, err := resolveCfg(cmd.Config)
	if err != nil {
		return Profile{}, err
	}
	file, err := loadFileCfg(path)
	if err != nil {
		return Profile{}, err
	}
	return pickProfile(file, cmd.Profile)
}

// connect 每次现登. 令牌只留在这次进程, 不写回配置.
func connect(profile Profile) (Session, error) {
	if profile.Username == "" || profile.Password == "" {
		return Session{}, fmt.Errorf("profile %s needs username and password", profile.Name)
	}
	host, err := originOf(profile.URL)
	if err != nil {
		return Session{}, err
	}
	token, err := login(host, profile.Username, profile.Password)
	if err != nil {
		return Session{}, err
	}
	return Session{Host: host, Token: token, BFF: false}, nil
}

// runDoctor 只检查已有配置能否解析. 不联网, 也不创建配置目录.
func runDoctor(cmd Cmd, stdout io.Writer) error {
	path, err := resolveCfg(cmd.Config)
	items := []doctor.Item{{Name: "config", Err: err}}
	if err == nil {
		buf, readErr := cfgfile.Read(path, cfgExample)
		if readErr != nil {
			items = []doctor.Item{{Name: "config", Err: readErr}}
		} else {
			_, parseErr := parseFileCfg(buf, path)
			items = []doctor.Item{
				{Name: "config", Err: nil},
				{Name: "fields", Err: parseErr},
			}
		}
	}
	if err := doctor.Write(stdout, items); err != nil {
		if strings.Contains(err.Error(), "doctor failed") {
			return quietExit{}
		}
		return err
	}
	return nil
}
