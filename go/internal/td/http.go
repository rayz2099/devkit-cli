package td

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const maxBody = 64 << 20

// Session 只活在这一次进程里. 配置改完不用再单独保存令牌.
type Session struct {
	Host  string
	Token string
	BFF   bool
}

type httpSend struct {
	URL    string
	Method string
	Token  string
	JSON   []byte
	Form   string
	FormOK bool
}

func apiSend(in httpSend) (json.RawMessage, error) {
	var body io.Reader
	req, err := http.NewRequest(in.Method, in.URL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Accept-Language", "zh-CN")
	if in.Token != "" {
		req.Header.Set("Authorization", "bearer "+in.Token)
	}
	if in.FormOK {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		body = strings.NewReader(in.Form)
	} else if in.JSON != nil {
		req.Header.Set("Content-Type", "application/json")
		body = bytes.NewReader(in.JSON)
	}
	if body != nil {
		req, err = http.NewRequest(in.Method, in.URL, body)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("X-Requested-With", "XMLHttpRequest")
		req.Header.Set("Accept-Language", "zh-CN")
		if in.Token != "" {
			req.Header.Set("Authorization", "bearer "+in.Token)
		}
		if in.FormOK {
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		} else if in.JSON != nil {
			req.Header.Set("Content-Type", "application/json")
		}
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	buf, err := readLimited(res.Body, maxBody)
	if err != nil {
		return nil, err
	}
	return unwrap(res.StatusCode, buf)
}

// unwrap 把 return_code 和 HTTP 401 分开. 前者是业务失败, 后者是令牌无效, 不能当成空结果.
func unwrap(status int, body []byte) (json.RawMessage, error) {
	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		return nil, fmt.Errorf("unauthorized")
	}
	rawBody := bytes.TrimSpace(body)
	if len(rawBody) == 0 {
		rawBody = []byte("{}")
	}
	if !json.Valid(rawBody) {
		msg := strings.TrimSpace(string(body))
		if status < 200 || status >= 300 {
			if msg == "" {
				return nil, fmt.Errorf("http %d", status)
			}
			return nil, fmt.Errorf("%s", msg)
		}
		return marshalCompact(map[string]string{"return_message": string(body)})
	}
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("%s", httpMsg(status, rawBody))
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(rawBody, &obj); err != nil {
		return json.RawMessage(rawBody), nil
	}
	code, hasCode := obj["return_code"]
	if !hasCode {
		return json.RawMessage(rawBody), nil
	}
	if !codeOK(code) {
		message := rawText(obj["return_message"])
		if message == "" {
			message = fmt.Sprintf("return_code %s", rawText(code))
		}
		return nil, fmt.Errorf("%s", message)
	}
	if data, ok := obj["data"]; ok {
		return data, nil
	}
	return json.RawMessage(rawBody), nil
}

func codeOK(raw json.RawMessage) bool {
	var n json.Number
	if err := json.Unmarshal(raw, &n); err == nil {
		return n.String() == "0"
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s == "0"
	}
	return false
}

func rawText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	v, err := decodeJSON(raw)
	if err != nil {
		return ""
	}
	return asText(v)
}

func httpMsg(status int, body []byte) string {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(body, &obj); err == nil {
		for _, key := range []string{"return_message", "error", "message"} {
			message := rawText(obj[key])
			if message != "" {
				return message
			}
		}
	}
	return fmt.Sprintf("http %d", status)
}

func taRoot(host string, bff bool) (string, error) {
	origin, err := originOf(host)
	if err != nil {
		return "", err
	}
	if bff {
		return origin + "/bff/v1/ta", nil
	}
	return origin + "/v1/ta", nil
}

func withQuery(raw string, query [][2]string, stamp bool, now time.Time) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString(u.Scheme)
	b.WriteString("://")
	b.WriteString(u.Host)
	b.WriteString(u.EscapedPath())
	b.WriteByte('?')
	first := true
	write := func(k, v string) {
		if !first {
			b.WriteByte('&')
		}
		first = false
		b.WriteString(url.QueryEscape(k))
		b.WriteByte('=')
		b.WriteString(url.QueryEscape(v))
	}
	for _, pair := range query {
		write(pair[0], pair[1])
	}
	if stamp {
		write("@t", fmt.Sprintf("%d", now.UnixMilli()))
	}
	return b.String(), nil
}

func formBody(pairs [][2]string) string {
	var b strings.Builder
	for i, pair := range pairs {
		if i > 0 {
			b.WriteByte('&')
		}
		b.WriteString(url.QueryEscape(pair[0]))
		b.WriteByte('=')
		b.WriteString(url.QueryEscape(pair[1]))
	}
	return b.String()
}

func readLimited(r io.Reader, max int64) ([]byte, error) {
	buf, err := io.ReadAll(io.LimitReader(r, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(buf)) > max {
		return nil, fmt.Errorf("response too large")
	}
	return buf, nil
}
