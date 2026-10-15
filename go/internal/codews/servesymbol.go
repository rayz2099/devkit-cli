package codews

import (
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// Hit 是符号的一处定义或引用。定义在前，前端据此决定直跳还是浮层。
type Hit struct {
	Path string `json:"path"`
	Line int    `json:"line"`
	Kind string `json:"kind"`
	Text string `json:"text"`
}

// SymbolResult 是一次符号查询。Truncated 表示预算或条数到顶，不是搜完了。
type SymbolResult struct {
	Query     string `json:"query"`
	Hits      []Hit  `json:"hits"`
	Truncated bool   `json:"truncated"`
}

var symbolRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{1,79}$`)

const (
	fileMax    = 1024 * 1024
	byteBudget = 32 * 1024 * 1024
	defCap     = 20
	refCap     = 30
)

// SearchSymbol 在已索引文件里找定义和引用。不启语言服务，serve 要能秒开。
func SearchSymbol(root string, files []string, query string) SymbolResult {
	empty := SymbolResult{Query: query, Hits: []Hit{}}
	if !symbolRe.MatchString(query) {
		return empty
	}
	ordered := append([]string{}, files...)
	slices.SortStableFunc(ordered, func(a, b string) int {
		return rankFile(a, query) - rankFile(b, query)
	})
	defs := defRes(query)
	word := regexp.MustCompile(`\b` + regexp.QuoteMeta(query) + `\b`)
	var hits []Hit
	seen := map[string]bool{}
	bytes := 0
	defN := 0
	refN := 0
	truncated := false
	stop := false
	for _, rel := range ordered {
		if stop {
			break
		}
		if bytes >= byteBudget {
			truncated = true
			break
		}
		text, ok := readSafe(root, rel)
		if !ok {
			continue
		}
		bytes += len(text)
		lines := strings.Split(text, "\n")
		for i, line := range lines {
			if defN >= defCap && refN >= refCap {
				truncated = true
				stop = true
				break
			}
			if !word.MatchString(line) {
				continue
			}
			isDef := false
			for _, re := range defs {
				if re.MatchString(line) {
					isDef = true
					break
				}
			}
			if isDef && defN >= defCap {
				continue
			}
			if !isDef && refN >= refCap {
				continue
			}
			key := rel + ":" + itoa(i+1)
			if seen[key] {
				continue
			}
			seen[key] = true
			kind := "ref"
			if isDef {
				defN++
				kind = "def"
			} else {
				refN++
			}
			hits = append(hits, Hit{Path: rel, Line: i + 1, Kind: kind, Text: clipLine(line)})
		}
	}
	slices.SortStableFunc(hits, compareHit)
	if hits == nil {
		hits = []Hit{}
	}
	return SymbolResult{Query: query, Hits: hits, Truncated: truncated}
}

func readSafe(root, rel string) (string, bool) {
	defer func() { _ = recover() }()
	return ReadIndexText(root, rel, fileMax)
}

func rankFile(file, query string) int {
	base := strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
	if base == query {
		return 0
	}
	return 1
}

func clipLine(line string) string {
	trimmed := strings.TrimSpace(line)
	runes := []rune(trimmed)
	if len(runes) <= 180 {
		return trimmed
	}
	return string(runes[:180])
}

func compareHit(a, b Hit) int {
	if a.Kind != b.Kind {
		if a.Kind == "def" {
			return -1
		}
		return 1
	}
	if a.Path != b.Path {
		if a.Path < b.Path {
			return -1
		}
		return 1
	}
	return a.Line - b.Line
}

func defRes(name string) []*regexp.Regexp {
	n := regexp.QuoteMeta(name)
	srcs := []string{
		`\bclass\s+` + n + `\b`,
		`\binterface\s+` + n + `\b`,
		`\benum\s+(?:class\s+)?` + n + `\b`,
		`\btype\s+` + n + `\b`,
		`\b(?:fun|func|fn|def)\s+(?:\([^)\n]*\)\s*)?` + n + `\b`,
		`\b(?:const|let|var|val)\s+` + n + `\b`,
		`\b(?:struct|trait|mod|object|typealias|namespace)\s+` + n + `\b`,
		`\b(?:async\s+)?function\s+` + n + `\b`,
		`#define\s+` + n + `\b`,
		`\b(?:public|private|protected|internal)\s+(?:static\s+|final\s+|open\s+|override\s+|abstract\s+|suspend\s+)*[\w.<>,?\[\]]+\s+` + n + `\s*\(`,
	}
	out := make([]*regexp.Regexp, 0, len(srcs))
	for _, src := range srcs {
		out = append(out, regexp.MustCompile(src))
	}
	return out
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [16]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
