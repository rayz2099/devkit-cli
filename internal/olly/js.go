package olly

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"strconv"
	"strings"
)

// form 按插入顺序编码。Go 的 url.Values.Encode 会按 key 排序，对不上 URLSearchParams。
type form struct {
	pairs [][2]string
}

func (f *form) set(key, val string) {
	for i := range f.pairs {
		if f.pairs[i][0] == key {
			f.pairs[i][1] = val
			return
		}
	}
	f.add(key, val)
}

func (f *form) add(key, val string) {
	f.pairs = append(f.pairs, [2]string{key, val})
}

func (f form) get(key string) (string, bool) {
	for _, pair := range f.pairs {
		if pair[0] == key {
			return pair[1], true
		}
	}
	return "", false
}

func (f form) getAll(key string) []string {
	var out []string
	for _, pair := range f.pairs {
		if pair[0] == key {
			out = append(out, pair[1])
		}
	}
	return out
}

func (f form) encode() string {
	var buf strings.Builder
	for i, pair := range f.pairs {
		if i > 0 {
			buf.WriteByte('&')
		}
		buf.WriteString(formEscape(pair[0]))
		buf.WriteByte('=')
		buf.WriteString(formEscape(pair[1]))
	}
	return buf.String()
}

// formEscape 对齐 URLSearchParams：空格变 +，星号保留。QueryEscape 会把 * 编成 %2A。
func formEscape(val string) string {
	return strings.ReplaceAll(url.QueryEscape(val), "%2A", "*")
}

func trimRightSlash(val string) string {
	return strings.TrimRight(val, "/")
}

func withScheme(val string) string {
	if strings.HasPrefix(val, "http://") || strings.HasPrefix(val, "https://") {
		return val
	}
	return "http://" + val
}

// jsNum 把 JSON 数字印成 JS String(number)。整数值不能带小数点。
func jsNum(val float64) string {
	if math.IsNaN(val) || math.IsInf(val, 0) {
		return ""
	}
	return strconv.FormatFloat(val, 'f', -1, 64)
}

func jsFmt(val any) string {
	switch typed := val.(type) {
	case nil:
		return ""
	case string:
		return typed
	case float64:
		return jsNum(typed)
	case json.Number:
		return typed.String()
	case bool:
		return strconv.FormatBool(typed)
	default:
		return fmt.Sprint(typed)
	}
}

func asFloat(val any) (float64, bool) {
	switch typed := val.(type) {
	case float64:
		return typed, true
	case json.Number:
		num, err := typed.Float64()
		return num, err == nil
	case int:
		return float64(typed), true
	default:
		return 0, false
	}
}

func asString(val any) (string, bool) {
	text, ok := val.(string)
	return text, ok
}

// pretty 用两空格缩进并保留字符串里的 &，对齐 JSON.stringify(value, null, 2)。
func pretty(val any) (string, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(val); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func prettyRaw(raw []byte) (string, error) {
	var buf bytes.Buffer
	if err := json.Indent(&buf, raw, "", "  "); err != nil {
		return "", err
	}
	buf.WriteByte('\n')
	return buf.String(), nil
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

func isRecord(val any) (map[string]any, bool) {
	rec, ok := val.(map[string]any)
	return rec, ok
}
