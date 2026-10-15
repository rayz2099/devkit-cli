package codews

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"strings"
)

//go:embed assets/serve.html
var servePage string

const rootSentinel = "__CW_ROOT_NAME__"

// RenderPage 把目录名填进从 TypeScript 原样导出的页面。不改样式和脚本。
func RenderPage(rootName string) string {
	js := jsonString(rootName)
	page := strings.Replace(servePage, `rootName: "`+rootSentinel+`"`, "rootName: "+js, 1)
	return strings.ReplaceAll(page, rootSentinel, htmlEscape(rootName))
}

func htmlEscape(s string) string {
	r := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
	)
	return r.Replace(s)
}

func jsonString(s string) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(s)
	return strings.TrimRight(buf.String(), "\n")
}

func encodeJSON(dst *bytes.Buffer, v any) {
	enc := json.NewEncoder(dst)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}
