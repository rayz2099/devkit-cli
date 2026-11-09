package issue

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// APIError 保留服务端错误码和追踪号，冲突直接上报而不重放写入。
type APIError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Status    int    `json:"status,omitempty"`
	Details   any    `json:"details,omitempty"`
	RequestID string `json:"requestId,omitempty"`
}

func (e *APIError) Error() string { return fmt.Sprintf("%s: %s", e.Code, e.Message) }

// Client 可注入 HTTP transport；始终禁止重定向，防止凭据泄露和隐式重发。
type Client struct {
	cfg  Config
	http *http.Client
}

func NewClient(cfg Config, hc *http.Client) (*Client, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	client := http.Client{Timeout: 30 * time.Second}
	if hc != nil {
		client = *hc
	}
	if client.Timeout == 0 {
		client.Timeout = 30 * time.Second
	}
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	cfg.URL = strings.TrimRight(cfg.URL, "/")
	return &Client{cfg: cfg, http: &client}, nil
}

// Do 只执行一次；网络失败不含原始 URL，避免 transport 将凭据带入错误。
func (c *Client) Do(ctx context.Context, method, path string, q url.Values, body map[string]any) (any, error) {
	var buf []byte
	var err error
	if body != nil {
		buf, err = json.Marshal(body)
		if err != nil {
			return nil, errors.New("请求内容不能编码为 JSON")
		}
	}
	target := c.cfg.URL + "/api/" + path
	if len(q) > 0 {
		target += "?" + q.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, target, bytes.NewReader(buf))
	if err != nil {
		return nil, errors.New("无法构造 HTTP 请求")
	}
	req.Header.Set("Authorization", "Bearer "+c.cfg.Token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, &APIError{Code: "TRANSPORT_ERROR", Message: "HTTP 请求失败；写入结果可能未知，请先查询，不要盲目重试"}
	}
	defer resp.Body.Close()
	const cap = 32 << 20
	raw, err := io.ReadAll(io.LimitReader(resp.Body, cap+1))
	if err != nil || len(raw) > cap {
		return nil, errors.New("读取响应失败或超过 32MiB 限制；写入结果可能未知")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		apiErr := &APIError{Code: "HTTP_ERROR", Message: http.StatusText(resp.StatusCode), Status: resp.StatusCode}
		var remote struct {
			Code      string `json:"code"`
			Message   string `json:"message"`
			Details   any    `json:"details"`
			RequestID string `json:"requestId"`
		}
		if json.Unmarshal(raw, &remote) == nil && remote.Code != "" {
			apiErr.Code = strings.ReplaceAll(remote.Code, c.cfg.Token, "[REDACTED]")
			apiErr.Message = strings.ReplaceAll(remote.Message, c.cfg.Token, "[REDACTED]")
			apiErr.RequestID = strings.ReplaceAll(remote.RequestID, c.cfg.Token, "[REDACTED]")
			apiErr.Details = redact(remote.Details, c.cfg.Token)
		}
		return nil, apiErr
	}
	var out any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, errors.New("服务端未返回有效 JSON；写入结果可能未知")
	}
	return redact(out, c.cfg.Token), nil
}
