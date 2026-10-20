package grafana

import (
	"fmt"
	"math"
	"net/url"
	"strconv"
	"strings"
)

// DashRef 是从公网 URL 或裸 uid 抽出的看板定位。真正请求不走这个 origin。
type DashRef struct {
	UID   string
	OrgID *int
	From  *string
	To    *string
}

// ParseDashRef 只从粘贴的 URL 里拿 uid、orgId 和时间窗。
// 公网域名只负责定位，API 必须打 profile 里的内网 base_url。
func ParseDashRef(source string) (DashRef, error) {
	trimmed := strings.TrimSpace(source)
	if trimmed == "" {
		return DashRef{}, fmt.Errorf("grafana dashboard url or uid is required")
	}
	u, err := url.Parse(trimmed)
	if err != nil || u.Scheme == "" || u.Host == "" {
		if strings.Contains(trimmed, "/") || strings.ContainsAny(trimmed, " \t\n\r") {
			return DashRef{}, fmt.Errorf("grafana dashboard ref is invalid: %s", trimmed)
		}
		return DashRef{UID: trimmed}, nil
	}
	uid := dashUID(u.Path)
	if uid == "" {
		return DashRef{}, fmt.Errorf("grafana url missing dashboard uid: %s", trimmed)
	}
	decoded, err := url.PathUnescape(uid)
	if err != nil {
		return DashRef{}, err
	}
	ref := DashRef{UID: decoded}
	if text := u.Query().Get("orgId"); text != "" {
		org, err := parseOrg(text)
		if err != nil {
			return DashRef{}, err
		}
		ref.OrgID = &org
	}
	if text := u.Query().Get("from"); text != "" {
		ref.From = &text
	}
	if text := u.Query().Get("to"); text != "" {
		ref.To = &text
	}
	return ref, nil
}

func dashUID(path string) string {
	parts := strings.Split(path, "/")
	for i, part := range parts {
		if part == "d" && i+1 < len(parts) && parts[i+1] != "" {
			return parts[i+1]
		}
	}
	return ""
}

func parseOrg(val string) (int, error) {
	num, err := strconv.ParseFloat(val, 64)
	if err != nil || num != math.Trunc(num) || math.IsInf(num, 0) {
		return 0, fmt.Errorf("grafana orgId is invalid: %s", val)
	}
	return int(num), nil
}

// BuildDashURL 给人点的链接必须用粘贴 URL 的 origin，不能把内网 API 地址暴露成入口。
func BuildDashURL(source, webBase, path, uid string, orgID *int, from, to string) (string, error) {
	origin := trimRight(webBase)
	if u, err := url.Parse(source); err == nil && u.Scheme != "" && u.Host != "" {
		origin = u.Scheme + "://" + u.Host
	}
	dashPath := path
	if dashPath == "" {
		dashPath = "/d/" + uid
	}
	base, err := url.Parse(origin + "/")
	if err != nil {
		return "", err
	}
	ref, err := url.Parse(dashPath)
	if err != nil {
		return "", err
	}
	resolved := base.ResolveReference(ref)
	var parts []string
	if orgID != nil {
		parts = append(parts, "orgId="+url.QueryEscape(strconv.Itoa(*orgID)))
	}
	parts = append(parts, "from="+url.QueryEscape(from), "to="+url.QueryEscape(to))
	resolved.RawQuery = strings.Join(parts, "&")
	resolved.Fragment = ""
	return resolved.String(), nil
}

// PublicURL 用公网 origin 拼保存后的路径。标题一变 slug 会变，不能沿用粘贴链接里的旧路径。
func PublicURL(cfg Config, source, metaPath, uid string, orgID *int) (string, error) {
	origin := cfg.BaseURL
	if cfg.HasWeb {
		origin = cfg.WebBaseURL
	}
	if u, err := url.Parse(strings.TrimSpace(source)); err == nil && u.Scheme != "" && u.Host != "" {
		origin = u.Scheme + "://" + u.Host
	}
	path := metaPath
	if path == "" {
		path = "/d/" + url.PathEscape(uid)
	}
	base, err := url.Parse(trimRight(origin) + "/")
	if err != nil {
		return "", err
	}
	ref, err := url.Parse(path)
	if err != nil {
		return "", err
	}
	resolved := base.ResolveReference(ref)
	if orgID != nil {
		q := resolved.Query()
		q.Set("orgId", strconv.Itoa(*orgID))
		resolved.RawQuery = q.Encode()
	}
	resolved.Fragment = ""
	return resolved.String(), nil
}

func trimRight(val string) string {
	return strings.TrimRight(val, "/")
}
