package har

import (
	"math"
	"strconv"
	"strings"
)

type selKind int

const (
	selHTTP selKind = iota
	selScalar
	selFields
)

type selection struct {
	kind   selKind
	scalar string
	fields []string
}

// Format 把命中的记录渲染成 text 或 json。
// 默认去重，HTTP 明文块之间空一行，标量和字段投影则一行一条，和原来的输出对齐。
func Format(entries []entry, selectRaw, mode string, dedupe bool, depth int) string {
	sel := parseSel(selectRaw)
	if mode == "json" {
		return formatJSON(entries, sel, dedupe, depth)
	}
	return formatText(entries, sel, dedupe, depth)
}

func parseSel(raw string) selection {
	norm := strings.TrimSpace(raw)
	if norm == "" || norm == "http" {
		return selection{kind: selHTTP}
	}
	if norm == "path" || norm == "url" || norm == "prefix" {
		return selection{kind: selScalar, scalar: norm}
	}
	parts := strings.Split(norm, ",")
	fields := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			fields = append(fields, part)
		}
	}
	if len(fields) == 0 {
		return selection{kind: selHTTP}
	}
	return selection{kind: selFields, fields: fields}
}

func formatText(entries []entry, sel selection, dedupe bool, depth int) string {
	blocks := make([]string, 0, len(entries))
	seen := map[string]struct{}{}
	for _, item := range entries {
		block := textOf(item, sel, depth)
		if dedupe {
			if _, ok := seen[block]; ok {
				continue
			}
			seen[block] = struct{}{}
		}
		blocks = append(blocks, block)
	}
	if len(blocks) == 0 {
		return ""
	}
	sep := "\n"
	if sel.kind == selHTTP {
		sep = "\n\n"
	}
	return strings.Join(blocks, sep) + "\n"
}

func formatJSON(entries []entry, sel selection, dedupe bool, depth int) string {
	vals := make([]jVal, 0, len(entries))
	seen := map[string]struct{}{}
	for _, item := range entries {
		val := jsonOf(item, sel, depth)
		if dedupe {
			key := val.compact()
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
		}
		vals = append(vals, val)
	}
	return stringify(jArr(vals), true) + "\n"
}

func textOf(item entry, sel selection, depth int) string {
	switch sel.kind {
	case selHTTP:
		return item.exchange()
	case selScalar:
		return scalarOf(item, sel.scalar, depth)
	default:
		src := projSource(item, depth)
		return project(src, sel.fields).compact()
	}
}

func jsonOf(item entry, sel selection, depth int) jVal {
	src := projSource(item, depth)
	switch sel.kind {
	case selFields:
		return project(src, sel.fields)
	case selScalar:
		m := newMap()
		m.put(sel.scalar, jStr(scalarOf(item, sel.scalar, depth)))
		return jObj(m)
	default:
		return src
	}
}

func scalarOf(item entry, which string, depth int) string {
	switch which {
	case "url":
		return item.url
	case "prefix":
		return Prefix(item.path, depth)
	default:
		return item.path
	}
}

func project(src jVal, fields []string) jVal {
	m := newMap()
	for _, field := range fields {
		val := src.path(field)
		if val.k == kindUndef {
			continue
		}
		setPath(m, field, val)
	}
	return jObj(m)
}

func projSource(item entry, depth int) jVal {
	m := newMap()
	m.put("sourceFile", jStr(item.source))
	m.put("path", jStr(item.path))
	m.put("url", jStr(item.url))
	m.put("prefix", jStr(Prefix(item.path, depth)))
	m.put("http", httpObj(item))
	m.put("request", reqObj(item))
	m.put("response", respObj(item))
	if item.body.k != kindUndef {
		m.put("body", item.body)
	}
	return jObj(m)
}

func httpObj(item entry) jVal {
	m := newMap()
	m.put("method", jStr(item.method))
	m.put("status", jNum(item.status))
	return jObj(m)
}

func reqObj(item entry) jVal {
	m := newMap()
	m.put("method", jStr(item.method))
	m.put("url", jStr(item.url))
	m.put("path", jStr(item.path))
	m.put("line", jStr(item.reqLine()))
	putPtr(m, "httpVersion", item.reqVer)
	m.put("headers", headersVal(item.reqHeaders))
	putPtr(m, "body", item.reqBody)
	m.put("raw", jStr(item.reqText()))
	return jObj(m)
}

func respObj(item entry) jVal {
	m := newMap()
	m.put("status", jNum(item.status))
	putPtr(m, "statusText", item.statusText)
	m.put("line", jStr(item.respLine()))
	putPtr(m, "httpVersion", item.respVer)
	putPtr(m, "mimeType", item.mime)
	m.put("headers", headersVal(item.respHeaders))
	putPtr(m, "body", item.respBody)
	m.put("raw", jStr(item.respText()))
	return jObj(m)
}

func putPtr(m *ordMap, key string, val *string) {
	if val == nil {
		return
	}
	m.put(key, jStr(*val))
}

func headersVal(headers []header) jVal {
	items := make([]jVal, 0, len(headers))
	for _, h := range headers {
		m := newMap()
		m.put("name", jStr(h.name))
		m.put("value", jStr(h.value))
		items = append(items, jObj(m))
	}
	return jArr(items)
}

func (e entry) exchange() string {
	return "[Request]\n" + e.reqText() + "\n\n[Response]\n" + e.respText()
}

func (e entry) reqLine() string {
	parts := []string{e.method, e.pathQuery}
	if e.reqVer != nil && *e.reqVer != "" {
		parts = append(parts, *e.reqVer)
	}
	return strings.Join(parts, " ")
}

func (e entry) respLine() string {
	var parts []string
	if e.respVer != nil && *e.respVer != "" {
		parts = append(parts, *e.respVer)
	}
	parts = append(parts, statusText(e.status))
	if e.statusText != nil && *e.statusText != "" {
		parts = append(parts, *e.statusText)
	}
	return strings.Join(parts, " ")
}

func statusText(n float64) string {
	if !math.IsInf(n, 0) && n == math.Trunc(n) && math.Abs(n) < 1e15 {
		return strconv.FormatInt(int64(n), 10)
	}
	return strconv.FormatFloat(n, 'f', -1, 64)
}

func (e entry) reqText() string {
	return httpMsg(e.reqLine(), e.reqHeaders, e.reqBody)
}

func (e entry) respText() string {
	return httpMsg(e.respLine(), e.respHeaders, e.respBody)
}

func httpMsg(start string, headers []header, body *string) string {
	lines := []string{start}
	for _, h := range headers {
		lines = append(lines, h.name+": "+h.value)
	}
	if body != nil && *body != "" {
		lines = append(lines, "", *body)
	}
	return strings.Join(lines, "\n")
}
