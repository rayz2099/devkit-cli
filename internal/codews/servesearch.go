package codews

import (
	"strings"
)

// Query 是 Cmd+K 的解析结果。词要全部命中，扩展名是过滤而不是模糊词。
type Query struct {
	Terms []string
	Exts  []string
}

// Ranked 是一条带分的路径。
type Ranked struct {
	Path  string
	Score int
}

var typeAlias = map[string][]string{
	"md": {".md", ".markdown"}, "markdown": {".md", ".markdown"},
	"kt": {".kt", ".kts"}, "kotlin": {".kt", ".kts"},
	"ts": {".ts", ".tsx"},
	"js": {".js", ".jsx", ".mjs", ".cjs"},
	"py": {".py"}, "go": {".go"}, "rs": {".rs", ".rust"},
	"java": {".java"}, "json": {".json"},
	"yml": {".yml", ".yaml"}, "yaml": {".yml", ".yaml"},
}

func pushUnique(arr []string, val string) []string {
	for _, item := range arr {
		if item == val {
			return arr
		}
	}
	return append(arr, val)
}

// ParseQuery 认 `readme .md`、`ext:kt` 和 `type:markdown` 这几种人类输入。
func ParseQuery(raw string) Query {
	var terms, exts []string
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Query{}
	}
	for _, token := range strings.Fields(raw) {
		lower := strings.ToLower(token)
		if strings.HasPrefix(lower, "ext:") || strings.HasPrefix(lower, "type:") {
			val := lower[strings.Index(lower, ":")+1:]
			val = strings.TrimPrefix(val, ".")
			if val == "" {
				continue
			}
			if mapped, ok := typeAlias[val]; ok {
				for _, ext := range mapped {
					exts = pushUnique(exts, ext)
				}
			} else {
				exts = pushUnique(exts, "."+val)
			}
			continue
		}
		if strings.HasPrefix(lower, ".") && len(lower) > 1 && !strings.Contains(lower[1:], "/") {
			exts = pushUnique(exts, lower)
			continue
		}
		terms = append(terms, token)
	}
	return Query{Terms: terms, Exts: exts}
}

func baseOf(path string) string {
	idx := strings.LastIndex(path, "/")
	if idx >= 0 {
		return path[idx+1:]
	}
	return path
}

func extOf(path string) string {
	base := strings.ToLower(baseOf(path))
	dot := strings.LastIndex(base, ".")
	if dot <= 0 {
		return ""
	}
	return base[dot:]
}

// FuzzyScore 是 fzf 风格的子序列分。散得太开的命中直接丢掉，避免 readme 误伤 metadata。
func FuzzyScore(text, query string) int {
	if query == "" {
		return 0
	}
	hay := strings.ToLower(text)
	needle := strings.ToLower(query)
	if hay == needle {
		return 10000
	}
	exact := strings.Index(hay, needle)
	if exact >= 0 {
		score := 5000 - exact*2 - max(0, len(hay)-len(needle))
		if exact == 0 {
			score += 800
		} else {
			prev := hay[exact-1]
			if prev == '/' || prev == '-' || prev == '_' || prev == '.' {
				score += 400
			}
		}
		return score
	}
	qi := 0
	score := 0
	prevMatch := -2
	gaps := 0
	for i := 0; i < len(hay) && qi < len(needle); i++ {
		if hay[i] != needle[qi] {
			continue
		}
		gain := 12
		if i == prevMatch+1 {
			gain += 48
		} else if prevMatch >= 0 {
			gap := i - prevMatch - 1
			gaps += gap
			gain -= min(30, gap*4)
		}
		if i == 0 {
			gain += 24
		} else {
			prev := hay[i-1]
			if prev == '/' || prev == '-' || prev == '_' || prev == '.' {
				gain += 20
			}
		}
		score += gain
		prevMatch = i
		qi++
	}
	if qi < len(needle) {
		return -1
	}
	if gaps > max(4, len(needle)) {
		return -1
	}
	score -= gaps * 6
	score -= max(0, len(hay)-len(needle))
	if score > 0 {
		return score
	}
	return -1
}

// ScorePath 综合文件名和全路径。带点的查询必须命中文件名，不然路径字符会拼出假阳性。
func ScorePath(path string, query Query) int {
	if len(query.Exts) > 0 {
		ext := extOf(path)
		ok := false
		for _, item := range query.Exts {
			if item == ext {
				ok = true
			}
		}
		if !ok {
			return -1
		}
	}
	if len(query.Terms) == 0 {
		if len(query.Exts) > 0 {
			return 1
		}
		return 0
	}
	base := baseOf(path)
	total := 0
	for _, term := range query.Terms {
		baseScore := FuzzyScore(base, term)
		pathScore := FuzzyScore(path, term)
		fileLike := strings.Contains(term, ".")
		if fileLike && baseScore < 0 {
			return -1
		}
		best := -1
		if baseScore >= 0 {
			best = baseScore + 1200
		}
		if !fileLike && pathScore > best {
			best = pathScore
		}
		if best < 0 {
			return -1
		}
		total += best
	}
	segs := strings.Count(path, "/") + 1
	if path == "" {
		segs = 0
	}
	total -= min(80, segs*4)
	return total
}

// RankPaths 排序检索结果。空查询时 recent 是主列表，有查询时只做轻量加权。
func RankPaths(files []string, raw string, recent []string, limit int) []Ranked {
	if limit <= 0 {
		limit = 50
	}
	query := ParseQuery(raw)
	empty := len(query.Terms) == 0 && len(query.Exts) == 0
	if empty {
		seen := map[string]bool{}
		var out []Ranked
		for _, path := range recent {
			if seen[path] || !contains(files, path) {
				continue
			}
			seen[path] = true
			out = append(out, Ranked{Path: path, Score: 1000000 - len(out)})
			if len(out) >= limit {
				return out
			}
		}
		for _, path := range files {
			if seen[path] {
				continue
			}
			out = append(out, Ranked{Path: path, Score: 0})
			if len(out) >= limit {
				break
			}
		}
		return out
	}
	boost := map[string]int{}
	for idx, path := range recent {
		gain := 200 - idx*8
		if gain < 0 {
			gain = 0
		}
		boost[path] = gain
	}
	var ranked []Ranked
	for _, path := range files {
		base := ScorePath(path, query)
		if base < 0 {
			continue
		}
		ranked = append(ranked, Ranked{Path: path, Score: base + boost[path]})
	}
	sortRanked(ranked)
	if len(ranked) > limit {
		ranked = ranked[:limit]
	}
	return ranked
}

func contains(files []string, path string) bool {
	for _, item := range files {
		if item == path {
			return true
		}
	}
	return false
}

func sortRanked(items []Ranked) {
	for i := 1; i < len(items); i++ {
		cur := items[i]
		j := i
		for j > 0 {
			prev := items[j-1]
			if prev.Score > cur.Score || (prev.Score == cur.Score && prev.Path <= cur.Path) {
				break
			}
			items[j] = prev
			j--
		}
		items[j] = cur
	}
}
