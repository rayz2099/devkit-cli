package har

import (
	"errors"
	"net/url"
	"strings"
)

// Prefix 把路径收成前 n 段。
// 权限白名单看的是接口分组，不是每一条带 query 的完整路径。
func Prefix(path string, depth int) string {
	segs := splitPath(path)
	if len(segs) == 0 {
		return "/*"
	}
	if depth < 1 {
		depth = 1
	}
	if depth > len(segs) {
		depth = len(segs)
	}
	return "/" + strings.Join(segs[:depth], "/") + "/*"
}

func splitPath(path string) []string {
	raw := strings.Split(path, "/")
	out := make([]string, 0, len(raw))
	for _, seg := range raw {
		if seg != "" {
			out = append(out, seg)
		}
	}
	return out
}

// splitURL 取出 pathname 和带 query 的路径。
// 输出要跟浏览器 URL 的 pathname/search 一致，不能用 Go 解码后的 Path。
func splitURL(raw string) (string, string, error) {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", "", errors.New("Invalid URL")
	}
	path := parsed.EscapedPath()
	if path == "" {
		path = "/"
	}
	return path, parsed.RequestURI(), nil
}
