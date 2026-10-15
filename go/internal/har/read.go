package har

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strings"
)

// entry 是一条归一化后的 HAR 记录。
// 过滤、明文和字段投影都读这一份，避免各处再解释一遍原始 JSON。
type entry struct {
	source      string
	method      string
	url         string
	path        string
	pathQuery   string
	status      float64
	statusText  *string
	mime        *string
	body        jVal
	reqVer      *string
	respVer     *string
	reqHeaders  []header
	respHeaders []header
	reqBody     *string
	respBody    *string
}

type header struct {
	name  string
	value string
}

func readFiles(files []string) ([]entry, error) {
	var out []entry
	for _, file := range files {
		buf, err := os.ReadFile(file)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil, fmt.Errorf("ENOENT: no such file or directory, open '%s'", file)
			}
			return nil, err
		}
		root, ok := parseJSONVal(string(buf))
		if !ok || root.k == kindNull {
			return nil, fmt.Errorf("invalid HAR: %s", file)
		}
		got, err := entriesOf(root, file)
		if err != nil {
			return nil, err
		}
		out = append(out, got...)
	}
	return out, nil
}

func entriesOf(root jVal, file string) ([]entry, error) {
	if root.k != kindObj {
		return nil, nil
	}
	log := root.child("log")
	if log.k != kindObj {
		return nil, nil
	}
	raw := log.child("entries")
	if raw.k == kindUndef || raw.k == kindNull {
		return nil, nil
	}
	if raw.k != kindArr {
		return nil, errors.New("invalid HAR entries")
	}
	out := make([]entry, 0, len(raw.a))
	for _, item := range raw.a {
		if item.k != kindObj {
			return nil, errors.New("invalid HAR entry")
		}
		got, ok, err := oneEntry(item, file)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, got)
		}
	}
	return out, nil
}

func oneEntry(item jVal, file string) (entry, bool, error) {
	req := item.child("request")
	resp := item.child("response")
	if req.k != kindObj || resp.k != kindObj {
		return entry{}, false, nil
	}
	rawURL := req.child("url")
	method := req.child("method")
	status := resp.child("status")
	if missingReq(rawURL) || missingReq(method) || status.k == kindUndef {
		return entry{}, false, nil
	}
	if rawURL.k != kindStr || method.k != kindStr || status.k != kindNum {
		return entry{}, false, errors.New("invalid HAR entry")
	}
	path, pathQuery, err := splitURL(rawURL.s)
	if err != nil {
		return entry{}, false, err
	}
	reqHeaders, err := readHeaders(req.child("headers"))
	if err != nil {
		return entry{}, false, err
	}
	respHeaders, err := readHeaders(resp.child("headers"))
	if err != nil {
		return entry{}, false, err
	}
	reqBody, err := readText(req.child("postData"))
	if err != nil {
		return entry{}, false, err
	}
	content := resp.child("content")
	respBody, err := readText(content)
	if err != nil {
		return entry{}, false, err
	}
	got := entry{
		source:      file,
		method:      method.s,
		url:         rawURL.s,
		path:        path,
		pathQuery:   pathQuery,
		status:      status.n,
		statusText:  strPtr(resp, "statusText"),
		mime:        mimeOf(content),
		body:        parseBody(respBody),
		reqVer:      strPtr(req, "httpVersion"),
		respVer:     strPtr(resp, "httpVersion"),
		reqHeaders:  reqHeaders,
		respHeaders: respHeaders,
		reqBody:     reqBody,
		respBody:    respBody,
	}
	return got, true, nil
}

func missingReq(v jVal) bool {
	if v.k == kindUndef || v.k == kindNull {
		return true
	}
	return v.k == kindStr && v.s == ""
}

func strPtr(obj jVal, key string) *string {
	val := obj.child(key)
	if val.k != kindStr {
		return nil
	}
	s := val.s
	return &s
}

func mimeOf(content jVal) *string {
	if content.k != kindObj {
		return nil
	}
	return strPtr(content, "mimeType")
}

func readHeaders(v jVal) ([]header, error) {
	if v.k == kindUndef || v.k == kindNull {
		return []header{}, nil
	}
	if v.k != kindArr {
		return nil, errors.New("invalid HAR headers")
	}
	out := make([]header, 0, len(v.a))
	for _, item := range v.a {
		if item.k != kindObj {
			return nil, errors.New("invalid HAR header")
		}
		name := ""
		if got := item.child("name"); got.k == kindStr {
			name = got.s
		}
		val := ""
		if got := item.child("value"); got.k == kindStr {
			val = got.s
		}
		out = append(out, header{name: name, value: val})
	}
	return out, nil
}

func readText(obj jVal) (*string, error) {
	if obj.k == kindUndef || obj.k == kindNull {
		return nil, nil
	}
	if obj.k != kindObj {
		return nil, errors.New("invalid HAR body")
	}
	text := obj.child("text")
	if text.k == kindUndef || text.k == kindNull {
		return nil, nil
	}
	if text.k != kindStr {
		return nil, nil
	}
	enc := obj.child("encoding")
	if enc.k == kindStr && enc.s == "base64" {
		got, err := decodeB64(text.s)
		if err != nil {
			return nil, err
		}
		return &got, nil
	}
	s := text.s
	return &s, nil
}

func parseBody(text *string) jVal {
	if text == nil {
		return undef()
	}
	val, ok := parseJSONVal(*text)
	if !ok {
		return undef()
	}
	return val
}

// decodeB64 按 Node Buffer 的 base64 习惯解开 HAR 正文。
// Chrome 导出的正文常带换行和省略的填充，严格解码会把本来能看的响应丢掉。
func decodeB64(raw string) (string, error) {
	var kept strings.Builder
	for _, r := range raw {
		if isB64(r) {
			kept.WriteRune(r)
		}
	}
	s := kept.String()
	// Node 会丢掉凑不齐一组的尾巴，而不是报错。
	if len(s)%4 == 1 {
		s = s[:len(s)-1]
	}
	if s == "" {
		return "", nil
	}
	if pad := len(s) % 4; pad != 0 {
		s += strings.Repeat("=", 4-pad)
	}
	buf, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return "", fmt.Errorf("decode base64 body: %w", err)
	}
	return string(buf), nil
}

func isB64(r rune) bool {
	return r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '+' || r == '/'
}
