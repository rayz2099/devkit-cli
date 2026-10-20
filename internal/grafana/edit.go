package grafana

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// Input 是 edit 的一次调用。Write 为假时 Body 被忽略，避免查看请求把空正文写成看板。
type Input struct {
	Aud       string
	Source    string
	Body      []byte
	Write     bool
	Overwrite bool
}

// Edit 读取或更新一块看板。已有 uid 默认不写，--overwrite 还要 version 与线上一致。
func Edit(ctx context.Context, client *http.Client, cfg Config, in Input) (string, error) {
	ref, err := ParseDashRef(in.Source)
	if err != nil {
		return "", err
	}
	if in.Write {
		return editWrite(ctx, client, cfg, ref, in)
	}
	return editRead(ctx, client, cfg, ref, in)
}

func editRead(ctx context.Context, client *http.Client, cfg Config, ref DashRef, in Input) (string, error) {
	dash, err := loadDash(ctx, client, cfg, ref)
	if err != nil {
		return "", editErr(in.Aud, ref, err)
	}
	if in.Aud == "agent" {
		return prettyRaw(dash.Raw)
	}
	link, err := readLink(cfg, in.Source, dash, ref.OrgID)
	if err != nil {
		return "", err
	}
	return humanRead(dash, ref.OrgID, link), nil
}

func readLink(cfg Config, source string, dash dashHead, orgID *int) (string, error) {
	if u := strings.TrimSpace(source); strings.Contains(u, "://") {
		return u, nil
	}
	return PublicURL(cfg, "", dash.MetaURL, dash.UID, orgID)
}

func editWrite(ctx context.Context, client *http.Client, cfg Config, ref DashRef, in Input) (string, error) {
	body, err := parseDash(in.Body)
	if err != nil {
		return "", err
	}
	if body.UID != ref.UID {
		return "", &Fail{
			Aud:     in.Aud,
			Type:    "uid",
			Message: fmt.Sprintf("dashboard uid %s does not match url uid %s", body.UID, ref.UID),
			UID:     ref.UID,
		}
	}
	live, err := loadDash(ctx, client, cfg, ref)
	if err != nil {
		return "", editErr(in.Aud, ref, err)
	}
	if !in.Overwrite {
		link, err := PublicURL(cfg, in.Source, live.MetaURL, live.UID, ref.OrgID)
		if err != nil {
			return "", err
		}
		return "", &Fail{
			Aud:     in.Aud,
			Type:    "exists",
			Message: fmt.Sprintf("dashboard uid %s already exists (version %d): %s", live.UID, live.Version, live.Title),
			UID:     live.UID,
			Title:   live.Title,
			Version: live.Version,
			URL:     link,
			Hint:    true,
		}
	}
	if body.Version != live.Version {
		link, err := PublicURL(cfg, in.Source, live.MetaURL, live.UID, ref.OrgID)
		if err != nil {
			return "", err
		}
		return "", &Fail{
			Aud:     in.Aud,
			Type:    "version",
			Message: fmt.Sprintf("dashboard uid %s version %d does not match server version %d", live.UID, body.Version, live.Version),
			UID:     live.UID,
			Title:   live.Title,
			Version: live.Version,
			URL:     link,
		}
	}
	saved, err := Save(ctx, client, cfg, ref.OrgID, live.FolderUID, body.Raw)
	if err != nil {
		return "", err
	}
	link, err := PublicURL(cfg, in.Source, saved.URL, saved.UID, ref.OrgID)
	if err != nil {
		return "", err
	}
	if in.Aud == "agent" {
		return agentWrite(saved.UID, saved.Version, link)
	}
	return fmt.Sprintf("updated %s\nversion: %d → %d\nurl: %s\n", saved.UID, live.Version, saved.Version, link), nil
}

func editErr(aud string, ref DashRef, err error) error {
	if errors.Is(err, ErrNotFound) {
		return &Fail{
			Aud:     aud,
			Type:    "missing",
			Message: fmt.Sprintf("dashboard uid %s not found", ref.UID),
			UID:     ref.UID,
		}
	}
	return err
}

// Fail 是 edit 拒绝写入时的结果。Agent 要机器可读的 type，人要能直接看的句子。
type Fail struct {
	Aud     string
	Type    string
	Message string
	UID     string
	Title   string
	Version int
	URL     string
	Hint    bool
}

// Error 按受众渲染。version 是线上版本，Agent 靠它改完再提交。
func (f *Fail) Error() string {
	if f.Aud == "agent" {
		text, err := agentFail(f)
		if err != nil {
			return f.Message
		}
		return text
	}
	var buf strings.Builder
	buf.WriteString(f.Message)
	if f.URL != "" {
		buf.WriteByte('\n')
		buf.WriteString(f.URL)
	}
	if f.Hint {
		buf.WriteString("\nre-run with --overwrite to update")
	}
	return buf.String()
}

type agentFailBody struct {
	OK    bool         `json:"ok"`
	Error agentFailErr `json:"error"`
}

type agentFailErr struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	UID     string `json:"uid,omitempty"`
	Title   string `json:"title,omitempty"`
	Version *int   `json:"version,omitempty"`
	URL     string `json:"url,omitempty"`
}

func agentFail(f *Fail) (string, error) {
	body := agentFailBody{Error: agentFailErr{
		Type:    f.Type,
		Message: f.Message,
		UID:     f.UID,
		Title:   f.Title,
		URL:     f.URL,
	}}
	if f.Type == "exists" || f.Type == "version" {
		ver := f.Version
		body.Error.Version = &ver
	}
	buf, err := json.MarshalIndent(body, "", "  ")
	if err != nil {
		return "", err
	}
	return string(buf), nil
}

type dashHead struct {
	Raw       json.RawMessage
	UID       string
	Title     string
	Version   int
	FolderUID string
	MetaURL   string
	Panels    int
}

func loadDash(ctx context.Context, client *http.Client, cfg Config, ref DashRef) (dashHead, error) {
	raw, _, err := Get(ctx, client, cfg, ref.UID, ref.OrgID)
	if err != nil {
		return dashHead{}, err
	}
	var envelope struct {
		Meta      json.RawMessage `json:"meta"`
		Dashboard json.RawMessage `json:"dashboard"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil || len(envelope.Dashboard) == 0 {
		return dashHead{}, errors.New("grafana dashboard missing uid")
	}
	head, err := parseDash(envelope.Dashboard)
	if err != nil {
		return dashHead{}, err
	}
	var meta struct {
		URL       string `json:"url"`
		FolderUID string `json:"folderUid"`
	}
	if len(envelope.Meta) > 0 {
		if err := json.Unmarshal(envelope.Meta, &meta); err != nil {
			return dashHead{}, errors.New("grafana dashboard meta is invalid")
		}
	}
	head.FolderUID = meta.FolderUID
	head.MetaURL = meta.URL
	return head, nil
}

func parseDash(raw []byte) (dashHead, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return dashHead{}, errors.New("dashboard json is empty")
	}
	var obj struct {
		UID     string          `json:"uid"`
		Title   string          `json:"title"`
		Version *int            `json:"version"`
		Panels  json.RawMessage `json:"panels"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil || obj.UID == "" || obj.Version == nil {
		return dashHead{}, errors.New("dashboard json is invalid")
	}
	n, err := countPanels(obj.Panels)
	if err != nil {
		return dashHead{}, err
	}
	return dashHead{Raw: json.RawMessage(raw), UID: obj.UID, Title: obj.Title, Version: *obj.Version, Panels: n}, nil
}

type panelNode struct {
	Type   string      `json:"type"`
	Panels []panelNode `json:"panels"`
}

func countPanels(raw json.RawMessage) (int, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return 0, nil
	}
	var nodes []panelNode
	if err := json.Unmarshal(raw, &nodes); err != nil {
		return 0, errors.New("dashboard panels are invalid")
	}
	return walkPanels(nodes), nil
}

func walkPanels(nodes []panelNode) int {
	n := 0
	for _, node := range nodes {
		if node.Type != "row" {
			n++
		}
		n += walkPanels(node.Panels)
	}
	return n
}

func humanRead(dash dashHead, orgID *int, link string) string {
	var buf strings.Builder
	fmt.Fprintf(&buf, "title: %s\nuid: %s\nversion: %d\n", dash.Title, dash.UID, dash.Version)
	if orgID != nil {
		fmt.Fprintf(&buf, "org: %d\n", *orgID)
	}
	fmt.Fprintf(&buf, "url: %s\npanels: %d\n", link, dash.Panels)
	return buf.String()
}

func prettyRaw(raw json.RawMessage) (string, error) {
	var buf bytes.Buffer
	if err := json.Indent(&buf, raw, "", "  "); err != nil {
		return "", err
	}
	buf.WriteByte('\n')
	return buf.String(), nil
}

type agentWriteBody struct {
	UID     string `json:"uid"`
	Version int    `json:"version"`
	URL     string `json:"url"`
	Status  string `json:"status"`
}

func agentWrite(uid string, version int, link string) (string, error) {
	buf, err := json.MarshalIndent(agentWriteBody{UID: uid, Version: version, URL: link, Status: "updated"}, "", "  ")
	if err != nil {
		return "", err
	}
	return string(buf) + "\n", nil
}
