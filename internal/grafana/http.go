package grafana

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
)

// ErrNotFound 表示这块 uid 在当前 org 里没有看板。编辑不会因此新建。
var ErrNotFound = errors.New("grafana dashboard not found")

// BasicAuth 是 Grafana basic 头。浏览器 btoa 对 ASCII 口令结果相同。
func BasicAuth(user, pass string) string {
	raw := user + ":" + pass
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(raw))
}

// Do 发 Grafana HTTP API。org 放头里，不放进 URL。client 为空时用默认传输，抽出前的调用方就是这样测的。
func Do(ctx context.Context, client *http.Client, cfg Config, method, specURL string, orgID *int, body []byte) (json.RawMessage, map[string]any, error) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, specURL, reader)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", BasicAuth(cfg.Username, cfg.Password))
	if orgID != nil {
		req.Header.Set("X-Grafana-Org-Id", strconv.Itoa(*orgID))
	}
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/json")
	}
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	buf, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, nil, &httpStatus{code: resp.StatusCode, body: string(buf)}
	}
	val, err := decodeJSON(buf)
	if err != nil {
		return nil, nil, fmt.Errorf("grafana response is not json: %s", string(buf))
	}
	rec, _ := val.(map[string]any)
	return json.RawMessage(buf), rec, nil
}

// Get 读看板定义。返回的 raw 是整段响应，供调用方自己切 dashboard。
func Get(ctx context.Context, client *http.Client, cfg Config, uid string, orgID *int) (json.RawMessage, map[string]any, error) {
	spec := cfg.BaseURL + "/api/dashboards/uid/" + url.PathEscape(uid)
	return Do(ctx, client, cfg, http.MethodGet, spec, orgID, nil)
}

type saveReq struct {
	Dashboard json.RawMessage `json:"dashboard"`
	FolderUID string          `json:"folderUid,omitempty"`
	Overwrite bool            `json:"overwrite"`
	Message   string          `json:"message"`
}

// SaveResult 是保存接口里编辑方需要的那几项。
type SaveResult struct {
	UID     string
	Version int
	URL     string
}

// Save 更新已有看板。folderUid 原样带回，避免 Grafana 把看板挪进 General。
func Save(ctx context.Context, client *http.Client, cfg Config, orgID *int, folderUID string, dash json.RawMessage) (SaveResult, error) {
	payload := saveReq{
		Dashboard: dash,
		FolderUID: folderUID,
		Overwrite: true,
		Message:   "grafana-cli edit",
	}
	buf, err := json.Marshal(payload)
	if err != nil {
		return SaveResult{}, err
	}
	spec := cfg.BaseURL + "/api/dashboards/db"
	raw, _, err := Do(ctx, client, cfg, http.MethodPost, spec, orgID, buf)
	if err != nil {
		return SaveResult{}, err
	}
	var resp struct {
		UID     string `json:"uid"`
		URL     string `json:"url"`
		Version *int   `json:"version"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return SaveResult{}, errors.New("grafana save response is invalid")
	}
	if resp.UID == "" || resp.Version == nil {
		return SaveResult{}, errors.New("grafana save response missing version")
	}
	return SaveResult{UID: resp.UID, Version: *resp.Version, URL: resp.URL}, nil
}

func decodeJSON(raw []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var val any
	if err := dec.Decode(&val); err != nil {
		return nil, err
	}
	return val, nil
}

type httpStatus struct {
	code int
	body string
}

func (e *httpStatus) Error() string {
	return fmt.Sprintf("grafana http %d: %s", e.code, e.body)
}

func (e *httpStatus) Unwrap() error {
	if e.code == http.StatusNotFound {
		return ErrNotFound
	}
	return nil
}
