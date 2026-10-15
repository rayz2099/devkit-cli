package nacos

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Client 直接打 Nacos OpenAPI。
// 不走 SDK 的本地快照，读到的就是当前这台 server 的结果。
type Client struct {
	rt    runtime
	base  string
	http  *http.Client
	token string
}

type listCfgIn struct {
	search   string
	dataID   string
	group    string
	pageNo   int
	pageSize int
}

type regIn struct {
	service   string
	ip        string
	group     string
	cluster   string
	port      int
	weight    float64
	ephemeral bool
}

type deregIn struct {
	service   string
	ip        string
	group     string
	cluster   string
	port      int
	ephemeral bool
}

type instQuery struct {
	service     string
	group       string
	clusters    []string
	healthyOnly bool
}

type serverEp struct {
	scheme string
	host   string
	port   int
}

func newClient(rt runtime) (*Client, error) {
	eps, err := parseServerAddr(rt.server)
	if err != nil {
		return nil, err
	}
	ep := eps[0]
	base := fmt.Sprintf("%s://%s:%d", ep.scheme, ep.host, ep.port)
	return &Client{rt: rt, base: base, http: &http.Client{}}, nil
}

func (c *Client) getConfig(dataID, group string) (string, error) {
	query := url.Values{}
	query.Set("dataId", dataID)
	query.Set("group", group)
	query.Set("tenant", c.rt.namespace)
	return c.request(http.MethodGet, "/nacos/v1/cs/configs", query, nil)
}

func (c *Client) putConfig(dataID, group, content string) error {
	form := url.Values{}
	form.Set("dataId", dataID)
	form.Set("group", group)
	form.Set("content", content)
	form.Set("tenant", c.rt.namespace)
	body, err := c.request(http.MethodPost, "/nacos/v1/cs/configs", url.Values{}, form)
	if err != nil {
		return err
	}
	if strings.ToLower(strings.TrimSpace(body)) != "true" {
		return fmt.Errorf("publish config failed")
	}
	return nil
}

func (c *Client) deleteConfig(dataID, group string) error {
	query := url.Values{}
	query.Set("dataId", dataID)
	query.Set("group", group)
	query.Set("tenant", c.rt.namespace)
	body, err := c.request(http.MethodDelete, "/nacos/v1/cs/configs", query, nil)
	if err != nil {
		return err
	}
	if strings.ToLower(strings.TrimSpace(body)) != "true" {
		return fmt.Errorf("delete config failed")
	}
	return nil
}

func (c *Client) listConfigs(in listCfgIn) (configPage, error) {
	query := url.Values{}
	query.Set("search", in.search)
	query.Set("dataId", in.dataID)
	query.Set("group", in.group)
	query.Set("pageNo", strconv.Itoa(in.pageNo))
	query.Set("pageSize", strconv.Itoa(in.pageSize))
	query.Set("tenant", c.rt.namespace)
	body, err := c.request(http.MethodGet, "/nacos/v1/cs/configs", query, nil)
	if err != nil {
		return configPage{}, err
	}
	return parseConfigPage(body, in.pageNo)
}

func (c *Client) registerInst(in regIn) error {
	query := url.Values{}
	query.Set("serviceName", in.service)
	query.Set("ip", in.ip)
	query.Set("port", strconv.Itoa(in.port))
	query.Set("groupName", in.group)
	query.Set("clusterName", in.cluster)
	query.Set("weight", strconv.FormatFloat(in.weight, 'f', -1, 64))
	query.Set("enable", "true")
	query.Set("healthy", "true")
	query.Set("ephemeral", strconv.FormatBool(in.ephemeral))
	query.Set("namespaceId", c.rt.namespace)
	body, err := c.request(http.MethodPost, "/nacos/v1/ns/instance", query, nil)
	if err != nil {
		return err
	}
	if strings.ToLower(strings.TrimSpace(body)) != "ok" {
		return fmt.Errorf("register instance failed")
	}
	return nil
}

func (c *Client) deregisterInst(in deregIn) error {
	query := url.Values{}
	query.Set("serviceName", in.service)
	query.Set("ip", in.ip)
	query.Set("port", strconv.Itoa(in.port))
	query.Set("groupName", in.group)
	query.Set("clusterName", in.cluster)
	query.Set("ephemeral", strconv.FormatBool(in.ephemeral))
	query.Set("namespaceId", c.rt.namespace)
	body, err := c.request(http.MethodDelete, "/nacos/v1/ns/instance", query, nil)
	if err != nil {
		return err
	}
	if strings.ToLower(strings.TrimSpace(body)) != "ok" {
		return fmt.Errorf("deregister instance failed")
	}
	return nil
}

func (c *Client) listInsts(in instQuery) ([]namingInst, error) {
	query := url.Values{}
	query.Set("serviceName", in.service)
	query.Set("groupName", in.group)
	query.Set("healthyOnly", strconv.FormatBool(in.healthyOnly))
	query.Set("namespaceId", c.rt.namespace)
	if len(in.clusters) > 0 {
		query.Set("clusters", strings.Join(in.clusters, ","))
	}
	body, err := c.request(http.MethodGet, "/nacos/v1/ns/instance/list", query, nil)
	if err != nil {
		return nil, err
	}
	return parseInsts(body)
}

// listItemsCached 给补全用。fish 每次 Tab 都是新进程，只能把列表落到 XDG cache。
func (c *Client) listItemsCached() ([]configItem, error) {
	path, err := runtimeFile("cache")
	if err != nil {
		return nil, err
	}
	key := cacheKey(c.rt.server, c.rt.namespace, c.rt.user)
	if items, ok := readListCache(path, key, time.Now()); ok {
		return items, nil
	}
	page, err := c.listConfigs(listCfgIn{search: "blur", pageNo: 1, pageSize: 1000})
	if err != nil {
		return nil, err
	}
	if err := writeListCache(path, key, page.items, time.Now()); err != nil {
		return nil, err
	}
	out := make([]configItem, len(page.items))
	copy(out, page.items)
	return out, nil
}

func (c *Client) request(method, path string, query, form url.Values) (string, error) {
	if err := c.ensureToken(); err != nil {
		return "", err
	}
	u, err := url.Parse(c.base + path)
	if err != nil {
		return "", err
	}
	if c.token != "" {
		query.Set("accessToken", c.token)
	}
	u.RawQuery = query.Encode()

	var body io.Reader
	if form != nil {
		if c.token != "" {
			form.Set("accessToken", c.token)
		}
		body = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequest(method, u.String(), body)
	if err != nil {
		return "", err
	}
	if form != nil {
		req.Header.Set("content-type", "application/x-www-form-urlencoded")
	}
	started := time.Now()
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	buf, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	text := string(buf)
	if err := c.writeDevLog(fmt.Sprintf("%s %s %d %dms", method, path, resp.StatusCode, time.Since(started).Milliseconds())); err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := fmt.Sprintf("nacos %s %s failed: %d %s", method, path, resp.StatusCode, text)
		return "", fmt.Errorf("%s", strings.TrimSpace(msg))
	}
	return text, nil
}

func (c *Client) ensureToken() error {
	if c.token != "" || strings.TrimSpace(c.rt.user) == "" {
		return nil
	}
	u, err := url.Parse(c.base + "/nacos/v1/auth/login")
	if err != nil {
		return err
	}
	form := url.Values{}
	form.Set("username", c.rt.user)
	form.Set("password", c.rt.pass)
	req, err := http.NewRequest(http.MethodPost, u.String(), strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("content-type", "application/x-www-form-urlencoded")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	buf, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	text := string(buf)
	if err := c.writeDevLog(fmt.Sprintf("POST /nacos/v1/auth/login %d", resp.StatusCode)); err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := fmt.Sprintf("nacos login failed: %d %s", resp.StatusCode, text)
		return fmt.Errorf("%s", strings.TrimSpace(msg))
	}
	var parsed map[string]any
	if err := json.Unmarshal(buf, &parsed); err != nil {
		return fmt.Errorf("nacos login failed: %s", err.Error())
	}
	tok, _ := parsed["accessToken"].(string)
	if tok == "" {
		return fmt.Errorf("nacos login failed: accessToken missing")
	}
	c.token = tok
	return nil
}

func (c *Client) writeDevLog(line string) error {
	if !c.rt.dev {
		return nil
	}
	path, err := runtimeFile("log")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	stamp := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	_, err = fmt.Fprintf(file, "%s %s\n", stamp, line)
	return err
}

// parseServerAddr 兼容 host:port 和带 scheme 的 URL，只使用逗号分隔里的第一台。
func parseServerAddr(raw string) ([]serverEp, error) {
	var result []serverEp
	for _, part := range strings.Split(raw, ",") {
		item := strings.TrimSpace(part)
		if item == "" {
			continue
		}
		ep, err := parseAddress(item)
		if err != nil {
			return nil, err
		}
		result = append(result, ep)
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("server-addr is required")
	}
	return result, nil
}

func parseAddress(addr string) (serverEp, error) {
	value := strings.TrimSpace(addr)
	if value == "" {
		return serverEp{}, fmt.Errorf("invalid server address: %s", addr)
	}
	if strings.Contains(value, "://") {
		parsed, err := url.Parse(value)
		if err != nil {
			return serverEp{}, fmt.Errorf("invalid server address: %s", addr)
		}
		host := strings.TrimSpace(parsed.Hostname())
		if host == "" {
			return serverEp{}, fmt.Errorf("invalid server address: %s", addr)
		}
		scheme := "http"
		if parsed.Scheme == "https" {
			scheme = "https"
		}
		if parsed.Port() == "" {
			return serverEp{scheme: scheme, host: host, port: 8848}, nil
		}
		port, err := strconv.Atoi(parsed.Port())
		if err != nil || port <= 0 {
			return serverEp{}, fmt.Errorf("invalid server port in address %s", addr)
		}
		return serverEp{scheme: scheme, host: host, port: port}, nil
	}
	last := strings.LastIndex(value, ":")
	if last < 0 {
		return serverEp{scheme: "http", host: value, port: 8848}, nil
	}
	host := strings.TrimSpace(value[:last])
	portRaw := strings.TrimSpace(value[last+1:])
	if host == "" || portRaw == "" {
		return serverEp{}, fmt.Errorf("invalid server address: %s", addr)
	}
	num, good := jsNumber(portRaw)
	if !good || !isJSInt(num) || num <= 0 {
		return serverEp{}, fmt.Errorf("invalid server port in address %s", addr)
	}
	return serverEp{scheme: "http", host: host, port: int(num)}, nil
}

func parseConfigPage(body string, pageNo int) (configPage, error) {
	if strings.TrimSpace(body) == "" {
		return configPage{page: float64(pageNo), items: []configItem{}}, nil
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(body), &parsed); err != nil {
		return configPage{}, err
	}
	items := []configItem{}
	if raw, ok := parsed["pageItems"].([]any); ok {
		for _, item := range raw {
			row, ok := item.(map[string]any)
			if !ok {
				continue
			}
			dataID, okID := row["dataId"].(string)
			group, okGroup := row["group"].(string)
			if !okID || !okGroup {
				continue
			}
			next := configItem{dataID: dataID, group: group}
			if content, ok := row["content"].(string); ok {
				next.content = &content
			}
			items = append(items, next)
		}
	}
	return configPage{
		total: asNumber(parsed["totalCount"], 0),
		page:  asNumber(parsed["pageNumber"], float64(pageNo)),
		pages: asNumber(parsed["pagesAvailable"], 0),
		items: items,
	}, nil
}

func parseInsts(body string) ([]namingInst, error) {
	if strings.TrimSpace(body) == "" {
		return []namingInst{}, nil
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(body), &parsed); err != nil {
		return nil, err
	}
	raw, ok := parsed["hosts"].([]any)
	if !ok {
		return []namingInst{}, nil
	}
	out := make([]namingInst, 0, len(raw))
	for _, item := range raw {
		row, ok := item.(map[string]any)
		if !ok {
			continue
		}
		ip, okIP := row["ip"].(string)
		port, okPort := row["port"].(float64)
		if !okIP || !okPort {
			continue
		}
		next := namingInst{ip: ip, port: port}
		if weight, ok := row["weight"].(float64); ok {
			next.weight = &weight
		}
		if healthy, ok := row["healthy"].(bool); ok {
			next.healthy = &healthy
		}
		if enabled, ok := row["enabled"].(bool); ok {
			next.enabled = &enabled
		}
		if ephemeral, ok := row["ephemeral"].(bool); ok {
			next.ephemeral = &ephemeral
		}
		if cluster, ok := row["clusterName"].(string); ok {
			next.clusterName = &cluster
		}
		if service, ok := row["serviceName"].(string); ok {
			next.serviceName = &service
		}
		if meta, ok := row["metadata"].(map[string]any); ok {
			copied := map[string]string{}
			for key, val := range meta {
				text, ok := val.(string)
				if ok {
					copied[key] = text
				}
			}
			next.metadata = &copied
		}
		out = append(out, next)
	}
	return out, nil
}

func asNumber(value any, def float64) float64 {
	num, ok := value.(float64)
	if !ok || math.IsNaN(num) || math.IsInf(num, 0) {
		return def
	}
	return num
}
