package codeup

import (
	"net/url"
	"regexp"
	"strings"
	"unicode/utf8"
)

const (
	// APIBase 是云效 OpenAPI 根。组织站点和 API 主机不是同一个。
	APIBase = "https://openapi-rdc.aliyuncs.com"
	// CodeupHost 锁死组织 URL 和 git remote，避免把别的托管当成云效。
	CodeupHost = "codeup.aliyun.com"
)

var (
	digitRE    = regexp.MustCompile(`^[0-9]+$`)
	sshRemote  = regexp.MustCompile(`^[^@]+@([^:]+):(.+?)(?:\.git)?$`)
	httpRemote = regexp.MustCompile(`^https?://(?:[^@]+@)?([^/]+)/(.+?)(?:\.git)?$`)
)

// GitRemote 是 origin URL 拆开后的主机和仓库路径。
type GitRemote struct {
	Host string
	Path string
}

// ParseOrgID 只接受组织根。多一段路径会把仓库名当成组织 ID。
func ParseOrgID(rawURL string) (string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" {
		return "", failf(1, "invalid organization url: %s", rawURL)
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return "", failf(1, "invalid organization url: %s", rawURL)
	}
	if parsed.Hostname() != CodeupHost {
		return "", failf(1, "organization url host must be %s", CodeupHost)
	}
	segs := splitPath(parsed.EscapedPath())
	if len(segs) != 1 {
		return "", fail(1, "organization url must be https://codeup.aliyun.com/<organizationId>")
	}
	orgID, err := url.PathUnescape(segs[0])
	if err != nil || orgID == "" || orgID == "<organizationId>" {
		return "", fail(1, "organization id is missing from profile url")
	}
	return orgID, nil
}

// EncodeRepoID 把 group/project 编成云效 repositoryId。数字 ID 保持原样，避免二次编码。
func EncodeRepoID(raw, orgID string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fail(1, "repository is required")
	}
	if digitRE.MatchString(trimmed) {
		return trimmed, nil
	}
	if strings.Contains(trimmed, "%2F") || strings.Contains(trimmed, "%2f") {
		return trimmed, nil
	}
	parts := splitPath(trimmed)
	if len(parts) > 0 && parts[0] == orgID {
		parts = parts[1:]
	}
	if len(parts) == 0 {
		return "", fail(1, "repository must be group/project or numeric id")
	}
	withOrg := make([]string, 0, len(parts)+1)
	withOrg = append(withOrg, orgID)
	withOrg = append(withOrg, parts...)
	enc := make([]string, len(withOrg))
	for i, seg := range withOrg {
		enc[i] = strings.ReplaceAll(encURI(seg), "+", "%20")
	}
	return strings.Join(enc, "%2F"), nil
}

// ParseGitRemote 同时认 ssh 和 https。解析失败必须马上报，不能猜主机。
func ParseGitRemote(raw string) (GitRemote, error) {
	if m := sshRemote.FindStringSubmatch(raw); m != nil {
		return GitRemote{Host: m[1], Path: m[2]}, nil
	}
	if m := httpRemote.FindStringSubmatch(raw); m != nil {
		return GitRemote{Host: m[1], Path: m[2]}, nil
	}
	return GitRemote{}, failf(1, "cannot parse git remote: %s", raw)
}

// RepoFromRemotePath 只留 group/project。组织 ID 以配置为准，不把 remote 里的租户写进命令参数。
func RepoFromRemotePath(path, orgID string) (string, error) {
	parts := splitPath(path)
	if len(parts) > 0 && parts[0] == orgID {
		parts = parts[1:]
	}
	if len(parts) < 2 {
		return "", failf(1, "git remote path is not group/project: %s", path)
	}
	return parts[0] + "/" + parts[1], nil
}

func splitPath(raw string) []string {
	chunks := strings.Split(raw, "/")
	out := make([]string, 0, len(chunks))
	for _, seg := range chunks {
		if seg != "" {
			out = append(out, seg)
		}
	}
	return out
}

func encURI(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 1 {
			b.WriteString(hexByte(s[i]))
			i++
			continue
		}
		if uriSafe(r) {
			b.WriteRune(r)
			i += size
			continue
		}
		var buf [utf8.UTFMax]byte
		n := utf8.EncodeRune(buf[:], r)
		for _, c := range buf[:n] {
			b.WriteString(hexByte(c))
		}
		i += size
	}
	return b.String()
}

func uriSafe(r rune) bool {
	switch {
	case r >= 'a' && r <= 'z':
	case r >= 'A' && r <= 'Z':
	case r >= '0' && r <= '9':
	default:
		return strings.ContainsRune("-_.!~*'()", r)
	}
	return true
}

func hexByte(c byte) string {
	const hex = "0123456789ABCDEF"
	return "%" + string([]byte{hex[c>>4], hex[c&0x0f]})
}
