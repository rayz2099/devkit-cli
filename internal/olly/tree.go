package olly

import (
	"math"
	"sort"
	"time"
)

const slowTopLimit = 10

// TraceNode 是调用链上的一个节点。depth 从根重新算，不能沿用接口里的顺序。
type TraceNode struct {
	Span     map[string]any
	Children []*TraceNode
	Depth    int
	IsError  bool
	IsSlow   bool
}

// TraceTree 是给人和 LLM 看的调用链，不只是 span 列表。
type TraceTree struct {
	Roots    []*TraceNode
	Orphans  []*TraceNode
	All      []*TraceNode
	TopSlow  []*TraceNode
	Errors   []*TraceNode
	TotalDur float64
}

// BuildTraceTree 按 parent id 把 span 列表收成树，并标出最慢的一批。
func BuildTraceTree(spans []map[string]any) TraceTree {
	total := 0.0
	for _, span := range spans {
		if dur := spanDur(span); dur > total {
			total = dur
		}
	}
	nodes := make([]*TraceNode, len(spans))
	byID := map[string]*TraceNode{}
	for i, span := range spans {
		node := &TraceNode{Span: span, IsError: spanErr(span)}
		nodes[i] = node
		byID[jsFmt(span["id"])] = node
	}
	var roots []*TraceNode
	var orphans []*TraceNode
	for _, node := range nodes {
		parentID, ok := spanParent(node.Span)
		if !ok || parentID == "0" {
			roots = append(roots, node)
			continue
		}
		parent := byID[parentID]
		if parent == nil {
			orphans = append(orphans, node)
			roots = append(roots, node)
			continue
		}
		node.Depth = parent.Depth + 1
		parent.Children = append(parent.Children, node)
	}
	for _, root := range roots {
		assignDepth(root, 0)
		sortKids(root)
	}
	sort.SliceStable(roots, func(i, j int) bool {
		return spanTime(roots[i].Span).Before(spanTime(roots[j].Span))
	})
	sorted := append([]*TraceNode(nil), nodes...)
	sort.SliceStable(sorted, func(i, j int) bool {
		return spanDur(sorted[i].Span) > spanDur(sorted[j].Span)
	})
	threshold := math.Inf(1)
	if len(sorted) > 0 {
		idx := slowTopLimit - 1
		if idx >= len(sorted) {
			idx = len(sorted) - 1
		}
		threshold = spanDur(sorted[idx].Span)
	}
	if len(nodes) > 0 {
		for _, node := range nodes {
			node.IsSlow = spanDur(node.Span) >= threshold
		}
	}
	var errs []*TraceNode
	for _, node := range nodes {
		if node.IsError {
			errs = append(errs, node)
		}
	}
	top := sorted
	if len(top) > slowTopLimit {
		top = top[:slowTopLimit]
	}
	return TraceTree{
		Roots:    roots,
		Orphans:  orphans,
		All:      nodes,
		TopSlow:  top,
		Errors:   errs,
		TotalDur: total,
	}
}

func assignDepth(node *TraceNode, depth int) {
	node.Depth = depth
	for _, child := range node.Children {
		assignDepth(child, depth+1)
	}
}

func sortKids(node *TraceNode) {
	sort.SliceStable(node.Children, func(i, j int) bool {
		return spanTime(node.Children[i].Span).Before(spanTime(node.Children[j].Span))
	})
	for _, child := range node.Children {
		sortKids(child)
	}
}

func spanDur(span map[string]any) float64 {
	if num, ok := asFloat(span["duration"]); ok {
		return num
	}
	return 0
}

func spanParent(span map[string]any) (string, bool) {
	val, ok := span["parentId"]
	if !ok {
		val, ok = span["parent_id"]
	}
	if !ok || val == nil {
		return "", false
	}
	return jsFmt(val), true
}

func spanErr(span map[string]any) bool {
	status := stringsLower(first(span["statusCode"], span["status_code"]))
	attrs := spanAttrs(span)
	attrStatus := stringsLower(first(attrs["otel.status_code"], attrs["status_code"]))
	return status == "error" || attrStatus == "error"
}

func spanAttrs(span map[string]any) map[string]any {
	if rec, ok := isRecord(span["attrs"]); ok {
		return rec
	}
	if rec, ok := isRecord(span["attributes"]); ok {
		return rec
	}
	return map[string]any{}
}

func first(vals ...any) any {
	for _, val := range vals {
		if val != nil {
			return val
		}
	}
	return nil
}

func stringsLower(val any) string {
	text, ok := val.(string)
	if !ok {
		return ""
	}
	out := make([]byte, 0, len(text))
	for i := 0; i < len(text); i++ {
		ch := text[i]
		if ch >= 'A' && ch <= 'Z' {
			ch += 'a' - 'A'
		}
		out = append(out, ch)
	}
	return string(out)
}

func spanTime(span map[string]any) time.Time {
	text, ok := span["time"].(string)
	if !ok || text == "" {
		if num, isNum := asFloat(span["time"]); isNum {
			return time.UnixMilli(int64(num)).UTC()
		}
		return time.Unix(0, 0).UTC()
	}
	if at, err := time.Parse(time.RFC3339Nano, text); err == nil {
		return at
	}
	return time.Unix(0, 0).UTC()
}
