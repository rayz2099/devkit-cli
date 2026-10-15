package td

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Span 是页面 recentDay 展开后的本地起止时间. 只传代号时服务端结果和页面不一致.
type Span struct {
	RecentDay string
	StartTime string
	EndTime   string
}

func resolveRecent(recent string, now time.Time) (Span, error) {
	parts := strings.Split(recent, "-")
	if len(parts) != 2 || !digits(parts[0]) || !digits(parts[1]) {
		return Span{}, fmt.Errorf("unsupported recentDay: %s", recent)
	}
	head, err := strconv.Atoi(parts[0])
	if err != nil {
		return Span{}, fmt.Errorf("unsupported recentDay: %s", recent)
	}
	tail, err := strconv.Atoi(parts[1])
	if err != nil {
		return Span{}, fmt.Errorf("unsupported recentDay: %s", recent)
	}
	older := tail
	if parts[0] == "0" {
		older = tail - 1
	}
	newer := head
	if older < 0 || newer < 0 {
		return Span{}, fmt.Errorf("unsupported recentDay: %s", recent)
	}
	y, m, d := now.Date()
	loc := now.Location()
	start := time.Date(y, m, d, 0, 0, 0, 0, loc).AddDate(0, 0, -older)
	end := time.Date(y, m, d, 23, 59, 59, 0, loc).AddDate(0, 0, -newer)
	return Span{RecentDay: recent, StartTime: fmtLocal(start), EndTime: fmtLocal(end)}, nil
}

// resolveSpan 把事件分析和属性分析的时间口径分开. 事件页默认过去 7 天, 属性页不补时间.
func resolveSpan(recent, from, to *string, pageDefault bool, now time.Time) (*Span, error) {
	hasFrom := from != nil
	hasTo := to != nil
	if hasFrom != hasTo {
		return nil, fmt.Errorf("--from and --to are both required")
	}
	if (hasFrom || hasTo) && recent != nil {
		return nil, fmt.Errorf("use either --recent or --from/--to")
	}
	if from != nil && to != nil {
		return &Span{RecentDay: "", StartTime: *from, EndTime: *to}, nil
	}
	if recent != nil {
		span, err := resolveRecent(*recent, now)
		if err != nil {
			return nil, err
		}
		return &span, nil
	}
	if pageDefault {
		span, err := resolveRecent("1-7", now)
		if err != nil {
			return nil, err
		}
		return &span, nil
	}
	return nil, nil
}

func fmtLocal(t time.Time) string {
	return t.Format("2006-01-02 15:04:05")
}

func digits(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
