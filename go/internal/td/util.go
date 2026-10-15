package td

import (
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

var intRe = regexp.MustCompile(`^-?\d+$`)

// 为什么: JS 的 Number.isSafeInteger 比 Go 的 int 窄, 超过这个范围服务端和页面都对不齐.
const maxSafeInt = 9007199254740991

func isObj(v any) bool {
	_, ok := v.(map[string]any)
	return ok
}

func asMap(v any) (map[string]any, bool) {
	m, ok := v.(map[string]any)
	return m, ok
}

// asText 对齐页面把数字和布尔塞进表格时的字符串, 对象不能被打成 [object Object].
func asText(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case json.Number:
		return t.String()
	case bool:
		return strconv.FormatBool(t)
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case int:
		return strconv.Itoa(t)
	case int64:
		return strconv.FormatInt(t, 10)
	default:
		return ""
	}
}

func decodeJSON(raw []byte) (any, error) {
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	return v, nil
}

// originOf 丢掉页面 hash 和路径. 接口和 websocket 只认 origin.
func originOf(host string) (string, error) {
	raw := host
	if !strings.Contains(host, "://") {
		raw = "http://" + host
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("invalid url")
	}
	name := u.Hostname()
	port := u.Port()
	if (u.Scheme == "http" && port == "80") || (u.Scheme == "https" && port == "443") {
		port = ""
	}
	var hostport string
	if port == "" {
		if strings.Contains(name, ":") {
			hostport = "[" + name + "]"
		} else {
			hostport = name
		}
	} else {
		hostport = net.JoinHostPort(name, port)
	}
	return u.Scheme + "://" + hostport, nil
}

// asInt 拦在查询体拼装之前. 模型号和项目号带小数时页面和接口对不上.
func asInt(raw, label string) (int, error) {
	if !intRe.MatchString(raw) {
		return 0, fmt.Errorf("%s must be an integer: %s", label, raw)
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || n > maxSafeInt || n < -maxSafeInt {
		return 0, fmt.Errorf("%s must be an integer: %s", label, raw)
	}
	return int(n), nil
}

// encodeURIComponent 对齐浏览器. websocket 路径上的令牌必须按这个集合编码, 不能用 PathEscape.
func encodeURIComponent(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if uriUnreserved(c) {
			b.WriteByte(c)
			continue
		}
		fmt.Fprintf(&b, "%%%02X", c)
	}
	return b.String()
}

func uriUnreserved(c byte) bool {
	if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') {
		return true
	}
	switch c {
	case '-', '_', '.', '!', '~', '*', '\'', '(', ')':
		return true
	default:
		return false
	}
}

func padEnd(s string, width int) string {
	n := utf8.RuneCountInString(s)
	if n >= width {
		return s
	}
	return s + strings.Repeat(" ", width-n)
}

func jsNumber(raw string) json.RawMessage {
	f, err := strconv.ParseFloat(raw, 64)
	if err != nil || strings.ContainsAny(raw, " \t") {
		return json.RawMessage("null")
	}
	b, err := json.Marshal(f)
	if err != nil {
		return json.RawMessage("null")
	}
	return b
}
