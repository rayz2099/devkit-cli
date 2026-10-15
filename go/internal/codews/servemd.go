package codews

import (
	"net/url"
	"strings"
)

// ResolveMdHref 把 Markdown 链接收成站点路径。外链和锚点保持原样，图片走 /raw 才能直接显示。
func ResolveMdHref(src, href, kind string) string {
	if href == "" || strings.HasPrefix(href, "#") || absHref(href) {
		return href
	}
	path, suffix := splitHref(href)
	if path == "" {
		return href
	}
	if strings.HasPrefix(path, "/raw/") || strings.HasPrefix(path, "/api/") {
		return path + suffix
	}
	joined := path
	if !strings.HasPrefix(path, "/") {
		joined = joinSrc(src, path)
	}
	encoded := encodePath(normRel(joined))
	if kind == "image" {
		return "/raw/" + encoded + suffix
	}
	return "/" + encoded + suffix
}

func absHref(href string) bool {
	if strings.HasPrefix(href, "//") {
		return true
	}
	for i := 0; i < len(href); i++ {
		ch := href[i]
		if ch == ':' {
			return i > 0
		}
		ok := (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (i > 0 && (ch == '+' || ch == '.' || ch == '-' || (ch >= '0' && ch <= '9')))
		if !ok {
			return false
		}
	}
	return false
}

func splitHref(href string) (string, string) {
	q := strings.Index(href, "?")
	h := strings.Index(href, "#")
	cut := -1
	if q >= 0 && (h < 0 || q < h) {
		cut = q
	} else if h >= 0 {
		cut = h
	}
	if cut < 0 {
		return href, ""
	}
	return href[:cut], href[cut:]
}

func joinSrc(src, rel string) string {
	src = strings.TrimLeft(strings.ReplaceAll(src, "\\", "/"), "/")
	slash := strings.LastIndex(src, "/")
	dir := ""
	if slash >= 0 {
		dir = src[:slash]
	}
	if dir == "" {
		return rel
	}
	return dir + "/" + rel
}

func normRel(path string) string {
	var parts []string
	for _, seg := range strings.Split(strings.ReplaceAll(path, "\\", "/"), "/") {
		if seg == "" || seg == "." {
			continue
		}
		if seg == ".." {
			if len(parts) > 0 {
				parts = parts[:len(parts)-1]
			}
			continue
		}
		parts = append(parts, seg)
	}
	return strings.Join(parts, "/")
}

func encodePath(path string) string {
	if path == "" {
		return ""
	}
	segs := strings.Split(path, "/")
	for i, seg := range segs {
		segs[i] = url.PathEscape(seg)
	}
	return strings.Join(segs, "/")
}
