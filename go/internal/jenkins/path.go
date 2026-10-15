package jenkins

import (
	"errors"
	"strings"
	"unicode/utf8"
)

// JobURL 把 JobPath 编成 Jenkins 的 /job 段。编码集中在一处，避免某一段漏编或被编两次。
func JobURL(jobPath string) (string, error) {
	trimmed := strings.Trim(jobPath, "/")
	if trimmed == "" {
		return "", errors.New("job path is required")
	}
	parts := strings.Split(trimmed, "/")
	var b strings.Builder
	for _, seg := range parts {
		b.WriteString("/job/")
		b.WriteString(encodeSeg(seg))
	}
	return b.String(), nil
}

// IsMainline 判断是不是主分支任务。用户禁止打主分支包，只看路径最后一段，不靠描述猜测。
func IsMainline(jobPath string) bool {
	last := ""
	for _, seg := range strings.Split(jobPath, "/") {
		if seg != "" {
			last = seg
		}
	}
	return last == "master" || last == "main"
}

// RunResult 把 Jenkins 的 building/result 收成一个状态。构建中优先，避免把半成品标成 SUCCESS。
func RunResult(building bool, result *string) string {
	if building {
		return "IN_PROGRESS"
	}
	if result == nil {
		return "UNKNOWN"
	}
	return *result
}

// encodeSeg 对齐 JS encodeURIComponent。url.PathEscape 会把 !*'() 也编掉，Job 名对不上。
func encodeSeg(s string) string {
	var b strings.Builder
	for _, r := range s {
		if isURIUnescaped(r) {
			b.WriteRune(r)
			continue
		}
		var tmp [utf8.UTFMax]byte
		n := utf8.EncodeRune(tmp[:], r)
		for i := 0; i < n; i++ {
			hex := "0123456789ABCDEF"
			b.WriteByte('%')
			b.WriteByte(hex[tmp[i]>>4])
			b.WriteByte(hex[tmp[i]&0x0F])
		}
	}
	return b.String()
}

func isURIUnescaped(r rune) bool {
	if r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
		return true
	}
	switch r {
	case '-', '_', '.', '!', '~', '*', '\'', '(', ')':
		return true
	default:
		return false
	}
}
