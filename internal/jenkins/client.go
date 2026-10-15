package jenkins

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// defaultHTTP 不设置 Transport。nil 走 net/http 默认传输，系统证书和代理保持开启。
// 禁止跟随重定向：触发构建把队列地址放在 302 的 Location 上，跟着跳会丢掉它。
var defaultHTTP = &http.Client{
	CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

type crumb struct {
	key string
	val string
}

// Client 把 crumb、404 和 Basic 收在一处。命令层只谈 JobPath 和 Run，不拼 URL。
type Client struct {
	rt         runtime
	http       *http.Client
	crumb      *crumb
	crumbTried bool
}

func newClient(rt runtime) *Client {
	return &Client{rt: rt, http: defaultHTTP}
}

func (c *Client) listJobs(folder string) ([]JobRef, error) {
	path := "/api/json"
	if strings.TrimSpace(folder) != "" {
		job, err := JobURL(folder)
		if err != nil {
			return nil, err
		}
		path = job + "/api/json"
	}
	var data struct {
		Jobs []struct {
			Name  string `json:"name"`
			URL   string `json:"url"`
			Class string `json:"_class"`
		} `json:"jobs"`
	}
	if err := c.readJSON(path, "jobs[name,url,_class]", &data); err != nil {
		return nil, err
	}
	out := make([]JobRef, 0, len(data.Jobs))
	for _, item := range data.Jobs {
		out = append(out, JobRef{Name: item.Name, URL: item.URL, ClassName: item.Class})
	}
	return out, nil
}

func (c *Client) viewJob(jobPath string) (JobView, error) {
	job, err := JobURL(jobPath)
	if err != nil {
		return JobView{}, err
	}
	var data struct {
		Name      string    `json:"name"`
		URL       string    `json:"url"`
		Buildable *bool     `json:"buildable"`
		Last      *buildRef `json:"lastBuild"`
		LastOK    *buildRef `json:"lastSuccessfulBuild"`
		LastBad   *buildRef `json:"lastFailedBuild"`
	}
	tree := "name,url,buildable,lastBuild[number],lastSuccessfulBuild[number],lastFailedBuild[number]"
	if err := c.readJSON(job+"/api/json", tree, &data); err != nil {
		return JobView{}, err
	}
	return JobView{
		Name:       data.Name,
		URL:        data.URL,
		Buildable:  data.Buildable != nil && *data.Buildable,
		Last:       refNum(data.Last),
		LastOK:     refNum(data.LastOK),
		LastFailed: refNum(data.LastBad),
	}, nil
}

func (c *Client) listRuns(jobPath string, limit int) ([]RunItem, error) {
	job, err := JobURL(jobPath)
	if err != nil {
		return nil, err
	}
	tree := fmt.Sprintf("builds[number,result,building,timestamp,duration]{0,%d}", limit)
	var data struct {
		Builds []rawBuild `json:"builds"`
	}
	if err := c.readJSON(job+"/api/json", tree, &data); err != nil {
		return nil, err
	}
	out := make([]RunItem, 0, len(data.Builds))
	for _, raw := range data.Builds {
		out = append(out, RunItem{
			Number:     num(raw.Number),
			Result:     RunResult(boolp(raw.Building), raw.Result),
			Building:   boolp(raw.Building),
			Timestamp:  num(raw.Timestamp),
			DurationMs: num(raw.Duration),
		})
	}
	return out, nil
}

func (c *Client) viewRun(jobPath, buildNo string) (RunInfo, error) {
	job, err := JobURL(jobPath)
	if err != nil {
		return RunInfo{}, err
	}
	path := job + "/" + encodeSeg(buildNo) + "/api/json"
	var raw rawBuild
	if err := c.readJSON(path, "", &raw); err != nil {
		return RunInfo{}, err
	}
	return toRunInfo(jobPath, raw), nil
}

func (c *Client) readLog(jobPath, buildNo string, tail *int) (string, error) {
	job, err := JobURL(jobPath)
	if err != nil {
		return "", err
	}
	path := job + "/" + encodeSeg(buildNo) + "/consoleText"
	resp, err := c.request(http.MethodGet, path, false, nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if err := httpErr(resp, path); err != nil {
		return "", err
	}
	buf, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	text := string(buf)
	if tail != nil {
		text = tailText(text, *tail)
	}
	return text, nil
}

func (c *Client) startRun(jobPath string) (TriggerOut, error) {
	if IsMainline(jobPath) {
		return TriggerOut{}, codeErr(1, "refusing to start mainline job: "+jobPath)
	}
	return c.postBuild(jobPath, "/build", nil)
}

func (c *Client) rerun(jobPath, buildNo string) (TriggerOut, error) {
	if IsMainline(jobPath) {
		return TriggerOut{}, codeErr(1, "refusing to rerun mainline job: "+jobPath)
	}
	info, err := c.viewRun(jobPath, buildNo)
	if err != nil {
		return TriggerOut{}, err
	}
	if len(info.Params.keys) == 0 {
		return c.postBuild(jobPath, "/build", nil)
	}
	return c.postBuild(jobPath, "/buildWithParameters", info.Params.val)
}

func (c *Client) cancelRun(jobPath, buildNo string) error {
	job, err := JobURL(jobPath)
	if err != nil {
		return err
	}
	path := job + "/" + encodeSeg(buildNo) + "/stop"
	resp, err := c.request(http.MethodPost, path, true, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return codeErr(2, fmt.Sprintf("Build not found: %s #%s", jobPath, buildNo))
	}
	if (resp.StatusCode < 200 || resp.StatusCode >= 300) && resp.StatusCode != http.StatusFound {
		msg := fmt.Sprintf("cancel failed: %d %s", resp.StatusCode, reason(resp))
		return codeErr(3, msg)
	}
	return nil
}

func (c *Client) listQueue() ([]QueueItem, error) {
	var data struct {
		Items []struct {
			ID   *int64 `json:"id"`
			Task *struct {
				Name string `json:"name"`
				URL  string `json:"url"`
			} `json:"task"`
			Why   string `json:"why"`
			Since *int64 `json:"inQueueSince"`
		} `json:"items"`
	}
	tree := "items[id,task[name,url],why,inQueueSince]"
	if err := c.readJSON("/queue/api/json", tree, &data); err != nil {
		return nil, err
	}
	out := make([]QueueItem, 0, len(data.Items))
	for _, item := range data.Items {
		name, taskURL := "", ""
		if item.Task != nil {
			name = item.Task.Name
			taskURL = item.Task.URL
		}
		out = append(out, QueueItem{
			ID:    num(item.ID),
			Name:  name,
			URL:   taskURL,
			Why:   item.Why,
			Since: num(item.Since),
		})
	}
	return out, nil
}

func (c *Client) postBuild(jobPath, suffix string, params map[string]string) (TriggerOut, error) {
	job, err := JobURL(jobPath)
	if err != nil {
		return TriggerOut{}, err
	}
	var body url.Values
	if params != nil {
		body = url.Values{}
		for k, v := range params {
			body.Set(k, v)
		}
	}
	resp, err := c.request(http.MethodPost, job+suffix, true, body)
	if err != nil {
		return TriggerOut{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return TriggerOut{}, codeErr(2, "Job not found: "+jobPath)
	}
	code := resp.StatusCode
	if code != http.StatusOK && code != http.StatusCreated && code != http.StatusFound {
		text := readLimit(resp.Body, 500)
		msg := fmt.Sprintf("Trigger failed: %d %s", code, reason(resp))
		if text != "" {
			msg += "\n" + text
		}
		return TriggerOut{}, codeErr(3, msg)
	}
	out := TriggerOut{Job: jobPath, Triggered: true, Status: code}
	if loc := resp.Header.Get("Location"); loc != "" {
		out.QueueLocation = &loc
	}
	return out, nil
}

func (c *Client) readJSON(path, tree string, dest any) error {
	full := path
	if tree != "" {
		full += "?tree=" + tree
	}
	resp, err := c.request(http.MethodGet, full, false, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if err := httpErr(resp, full); err != nil {
		return err
	}
	if err := json.NewDecoder(resp.Body).Decode(dest); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	return nil
}

func httpErr(resp *http.Response, path string) error {
	if resp.StatusCode == http.StatusNotFound {
		return codeErr(2, "not found: "+path)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := fmt.Sprintf("Jenkins API error: %d %s", resp.StatusCode, reason(resp))
		return codeErr(3, msg)
	}
	return nil
}

// reason 用响应状态行里的原文。http.StatusText 会换成标准短语，和旧 CLI 的 statusText 不一致。
func reason(resp *http.Response) string {
	raw := resp.Status
	prefix := strconv.Itoa(resp.StatusCode)
	if strings.HasPrefix(raw, prefix) {
		return strings.TrimSpace(raw[len(prefix):])
	}
	if raw != "" {
		return raw
	}
	return http.StatusText(resp.StatusCode)
}

func (c *Client) request(method, path string, withCrumb bool, body url.Values) (*http.Response, error) {
	var rdr io.Reader
	if body != nil {
		rdr = strings.NewReader(body.Encode())
	}
	req, err := http.NewRequest(method, c.rt.prof.URL+path, rdr)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(c.rt.prof.User, c.rt.secret)
	if body != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if withCrumb {
		got, err := c.loadCrumb()
		if err != nil {
			return nil, err
		}
		if got != nil {
			req.Header.Set(got.key, got.val)
		}
	}
	return c.http.Do(req)
}

func (c *Client) loadCrumb() (*crumb, error) {
	if c.crumbTried {
		return c.crumb, nil
	}
	c.crumbTried = true
	resp, err := c.request(http.MethodGet, "/crumbIssuer/api/json", false, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := fmt.Sprintf("crumb failed: %d %s", resp.StatusCode, reason(resp))
		return nil, codeErr(3, msg)
	}
	var data struct {
		Crumb string `json:"crumb"`
		Field string `json:"crumbRequestField"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	if data.Crumb != "" && data.Field != "" {
		c.crumb = &crumb{key: data.Field, val: data.Crumb}
	}
	return c.crumb, nil
}

type buildRef struct {
	Number *int64 `json:"number"`
}

type rawBuild struct {
	Number      *int64           `json:"number"`
	Result      *string          `json:"result"`
	Building    *bool            `json:"building"`
	Duration    *int64           `json:"duration"`
	Estimated   *int64           `json:"estimatedDuration"`
	Timestamp   *int64           `json:"timestamp"`
	URL         *string          `json:"url"`
	DisplayName *string          `json:"displayName"`
	FullName    *string          `json:"fullDisplayName"`
	Description *string          `json:"description"`
	Artifacts   []rawArt         `json:"artifacts"`
	Actions     []map[string]any `json:"actions"`
}

type rawArt struct {
	FileName string `json:"fileName"`
	RelPath  string `json:"relativePath"`
}

func toRunInfo(jobPath string, raw rawBuild) RunInfo {
	params, causes := collectMeta(raw.Actions)
	arts := make([]Artifact, 0, len(raw.Artifacts))
	for _, item := range raw.Artifacts {
		arts = append(arts, Artifact{Name: item.FileName, Path: item.RelPath})
	}
	info := RunInfo{
		RunStatus: RunStatus{
			Job:         jobPath,
			Build:       num(raw.Number),
			Result:      RunResult(boolp(raw.Building), raw.Result),
			Building:    boolp(raw.Building),
			DurationMs:  num(raw.Duration),
			Timestamp:   num(raw.Timestamp),
			URL:         strp(raw.URL),
			DisplayName: displayName(raw.DisplayName, raw.FullName),
		},
		EstimatedMs: num(raw.Estimated),
		Description: raw.Description,
		Artifacts:   arts,
		Causes:      causes,
		Params:      params,
	}
	return info
}

func displayName(name, full *string) *string {
	if name != nil {
		return name
	}
	return full
}

func collectMeta(actions []map[string]any) (paramMap, []string) {
	var params paramMap
	causes := make([]string, 0)
	for _, act := range actions {
		if list, ok := act["causes"].([]any); ok {
			for _, item := range list {
				rec, ok := item.(map[string]any)
				if !ok {
					continue
				}
				if text, ok := rec["shortDescription"].(string); ok {
					causes = append(causes, text)
				}
			}
		}
		if list, ok := act["parameters"].([]any); ok {
			for _, item := range list {
				rec, ok := item.(map[string]any)
				if !ok {
					continue
				}
				name := strings.TrimSpace(readStr(rec["name"]))
				if name == "" {
					continue
				}
				val, ok := jsString(rec["value"])
				if !ok {
					continue
				}
				params.set(name, val)
			}
		}
	}
	return params, causes
}

func jsString(v any) (string, bool) {
	switch t := v.(type) {
	case nil:
		return "", false
	case string:
		return t, true
	case bool:
		return strconv.FormatBool(t), true
	case float64:
		if math.IsNaN(t) || math.IsInf(t, 0) {
			return "", false
		}
		if t == math.Trunc(t) && t >= math.MinInt64 && t <= math.MaxInt64 {
			return strconv.FormatInt(int64(t), 10), true
		}
		return strconv.FormatFloat(t, 'f', -1, 64), true
	default:
		return "", false
	}
}

func refNum(ref *buildRef) *int64 {
	if ref == nil {
		return nil
	}
	return ref.Number
}

func num(v *int64) int64 {
	if v == nil {
		return 0
	}
	return *v
}

func boolp(v *bool) bool {
	return v != nil && *v
}

func strp(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func readLimit(r io.Reader, n int) string {
	buf, err := io.ReadAll(r)
	if err != nil || len(buf) == 0 {
		return ""
	}
	text := string(buf)
	runes := []rune(text)
	if len(runes) > n {
		text = string(runes[:n])
	}
	return text
}
