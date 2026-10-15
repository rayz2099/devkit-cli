package olly

import (
	"fmt"
	"math"
	"strings"
)

// AgentQuery 是用户查这条 trace 时带的服务、环境和路由，写进上下文避免模型再猜。
type AgentQuery struct {
	Service string
	Env     string
	URI     string
	HasSvc  bool
	HasEnv  bool
	HasURI  bool
}

// AgentContext 是给 LLM 的稳定输入，trace 结构已经展开。
type AgentContext struct {
	ProjectID float64
	TraceID   string
	TraceURL  string
	Query     AgentQuery
	Tree      TraceTree
	Hints     []string
}

// BuildContext 把 span 列表收成可直接交给模型的上下文。
func BuildContext(cfg UptraceConfig, traceInput string, spans []map[string]any, query AgentQuery) (AgentContext, error) {
	parsed, err := ParseTraceInput(traceInput)
	if err != nil {
		return AgentContext{}, err
	}
	project := cfg.ProjectID
	if parsed.ProjectID != nil {
		project = *parsed.ProjectID
	}
	web := cfg.BaseURL
	if cfg.HasWeb {
		web = cfg.WebBaseURL
	}
	return AgentContext{
		ProjectID: project,
		TraceID:   parsed.TraceID,
		TraceURL:  fmt.Sprintf("%s/traces/%s/%s", trimRightSlash(web), jsNum(project), parsed.TraceID),
		Query:     query,
		Tree:      BuildTraceTree(spans),
		Hints:     sourceHints(spans),
	}, nil
}

// FormatContextAgent 去掉装饰，只留调用链、耗时、错误和源码线索。
func FormatContextAgent(ctx AgentContext) string {
	lines := []string{
		"trace_id: " + ctx.TraceID,
		"project_id: " + jsNum(ctx.ProjectID),
		"trace_url: " + ctx.TraceURL,
	}
	if ctx.Query.Service != "" || ctx.Query.Env != "" || ctx.Query.URI != "" {
		lines = append(lines, "query:")
		addOpt(&lines, "service", ctx.Query.Service, 2)
		addOpt(&lines, "env", ctx.Query.Env, 2)
		addOpt(&lines, "uri", ctx.Query.URI, 2)
	}
	lines = append(lines,
		"summary:",
		"  total_duration_us: "+jsNum(math.Round(ctx.Tree.TotalDur)),
		"  span_count: "+jsNum(float64(len(ctx.Tree.All))),
		"  error_span_count: "+jsNum(float64(len(ctx.Tree.Errors))),
		"top_slow_spans:",
	)
	for _, node := range ctx.Tree.TopSlow {
		lines = append(lines, "  - "+spanLine(node))
	}
	lines = append(lines, "call_tree:")
	for _, root := range ctx.Tree.Roots {
		appendNode(&lines, root)
	}
	if len(ctx.Hints) > 0 {
		lines = append(lines, "source_hints:")
		for _, hint := range ctx.Hints {
			lines = append(lines, "  - "+hint)
		}
	}
	return strings.Join(lines, "\n") + "\n"
}

// FormatContextHuman 让人先看到入口、慢点和链接。
func FormatContextHuman(ctx AgentContext) string {
	lines := []string{
		"Trace: " + ctx.TraceID,
		"Project: " + jsNum(ctx.ProjectID),
		"URL: " + ctx.TraceURL,
		fmt.Sprintf("Spans: %s, Errors: %s, Duration(us): %s", jsNum(float64(len(ctx.Tree.All))), jsNum(float64(len(ctx.Tree.Errors))), jsNum(math.Round(ctx.Tree.TotalDur))),
		"",
		"Top slow spans:",
	}
	for _, node := range ctx.Tree.TopSlow {
		lines = append(lines, "- "+spanLine(node))
	}
	lines = append(lines, "", "Call tree:")
	for _, root := range ctx.Tree.Roots {
		appendNode(&lines, root)
	}
	return strings.Join(lines, "\n") + "\n"
}

func appendNode(lines *[]string, node *TraceNode) {
	pad := strings.Repeat("  ", node.Depth)
	*lines = append(*lines, pad+"- "+spanLine(node))
	for _, child := range node.Children {
		appendNode(lines, child)
	}
}

func spanLine(node *TraceNode) string {
	span := node.Span
	attrs := spanAttrs(span)
	service := hintText(first(attrs["service_name"], attrs["service.name"]))
	route := hintText(first(attrs["http_route"], attrs["http.route"], attrs["http_target"], attrs["http.target"]))
	name := "<unnamed>"
	if text, ok := span["name"].(string); ok && text != "" {
		name = text
	} else if span["name"] != nil {
		if text := hintText(span["name"]); text != "" {
			name = text
		}
	}
	kind := "unknown"
	if text := hintText(span["kind"]); text != "" {
		kind = text
	}
	suffix := ""
	if service != "" {
		suffix += " service=" + service
	}
	if route != "" {
		suffix += " route=" + route
	}
	if node.IsError {
		suffix += " error=true"
	}
	if node.IsSlow {
		suffix += " slow=true"
	}
	return fmt.Sprintf("id=%s name=\"%s\" kind=%s duration_us=%s%s", jsFmt(span["id"]), name, kind, jsNum(math.Round(spanDur(span))), suffix)
}

func sourceHints(spans []map[string]any) []string {
	seen := map[string]struct{}{}
	var hints []string
	add := func(key string, val any) {
		text := hintText(val)
		if text == "" {
			return
		}
		line := key + ": " + text
		if _, ok := seen[line]; ok {
			return
		}
		seen[line] = struct{}{}
		hints = append(hints, line)
	}
	for _, span := range spans {
		attrs := spanAttrs(span)
		add("service_name", first(attrs["service_name"], attrs["service.name"]))
		add("http_route", first(attrs["http_route"], attrs["http.route"]))
		add("http_target", first(attrs["http_target"], attrs["http.target"]))
		add("rpc_method", first(attrs["rpc_method"], attrs["rpc.method"]))
		add("peer_service", first(attrs["peer_service"], attrs["peer.service"]))
		add("db_statement", first(attrs["db_statement"], attrs["db.statement"]))
	}
	return hints
}

func addOpt(lines *[]string, key, val string, indent int) {
	if val == "" {
		return
	}
	*lines = append(*lines, strings.Repeat(" ", indent)+key+": "+val)
}

func hintText(val any) string {
	switch typed := val.(type) {
	case string:
		if typed == "" {
			return ""
		}
		return typed
	case float64:
		return jsNum(typed)
	case bool:
		return jsFmt(typed)
	default:
		return ""
	}
}
