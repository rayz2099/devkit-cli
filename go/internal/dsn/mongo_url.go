package dsn

import (
	"net/url"
	"regexp"
	"strings"
)

// Go 的 url.Parse 不认副本集逗号主机，mongo 的 seed list 必须单独认。
var mongoURLRe = regexp.MustCompile(`^(mongodb(?:\+srv)?)://(?:([^:@]*)(?::([^@]*))?@)?([^/?@]+)(.*)$`)

type mongoParts struct {
	hosts []string
	db    string
}

func parseMongoURL(raw string) (mongoParts, bool) {
	m := mongoURLRe.FindStringSubmatch(raw)
	if m == nil {
		return mongoParts{}, false
	}
	hostList := m[4]
	if hostList == "" || strings.HasPrefix(hostList, ":") {
		return mongoParts{}, false
	}
	hosts := strings.Split(hostList, ",")
	if len(hosts) == 0 {
		return mongoParts{}, false
	}
	for _, host := range hosts {
		if host == "" {
			return mongoParts{}, false
		}
	}
	rest := m[5]
	if hash := strings.IndexByte(rest, '#'); hash >= 0 {
		rest = rest[:hash]
	}
	path := rest
	if q := strings.IndexByte(path, '?'); q >= 0 {
		path = path[:q]
	}
	db, err := url.PathUnescape(strings.TrimPrefix(path, "/"))
	if err != nil {
		return mongoParts{}, false
	}
	return mongoParts{hosts: hosts, db: db}, true
}

func mongoURLOk(raw string) bool {
	_, ok := parseMongoURL(raw)
	return ok
}

// mongoDBName 从 path 取库名。没有 path 时返回空，调用方再用驱动默认库。
func mongoDBName(raw string) (string, error) {
	parts, ok := parseMongoURL(raw)
	if !ok {
		return "", exitf(1, "invalid mongodb url")
	}
	return parts.db, nil
}
