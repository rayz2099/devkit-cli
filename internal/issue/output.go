package issue

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"
)

// output human 展示字段层级，agent 原样保留后端分页及子树结构，避免有损转换。
func (r *runner) output(value any) error {
	value = redact(value, r.token)
	if r.audience == "agent" {
		return json.NewEncoder(r.opts.Out).Encode(value)
	}
	var text strings.Builder
	human(&text, value, 0)
	_, err := fmt.Fprint(r.opts.Out, text.String())
	return err
}

// personalOutput 只压缩 human 展示，避免自动化调用丢失原始字段和分组上下文。
func (r *runner) personalOutput(value map[string]any) error {
	if r.audience == "agent" {
		return r.output(value)
	}
	value = redact(value, r.token).(map[string]any)
	var text strings.Builder
	if groups, ok := value["groups"].([]any); ok {
		for _, v := range groups {
			group := v.(map[string]any)
			root := group["root"].(map[string]any)
			fmt.Fprintf(&text, "事项 %s  %s\n", tableCell(root["id"]), tableCell(root["title"]))
			writePersonalTable(&text, group["items"].([]any))
			text.WriteByte('\n')
		}
		if len(groups) == 0 {
			text.WriteString("（空）\n")
		}
	} else {
		writePersonalTable(&text, value["items"].([]any))
	}
	fmt.Fprintf(&text, "共 %v 项\n", value["count"])
	_, err := fmt.Fprint(r.opts.Out, text.String())
	return err
}

// writePersonalTable 固定摘要列，长描述与派生诊断仍由 agent JSON 提供。
func writePersonalTable(b *strings.Builder, items []any) {
	w := tabwriter.NewWriter(b, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "ID\t类型\t状态\t标题\t截止")
	for _, v := range items {
		item := v.(map[string]any)
		kind := tableCell(item["type"])
		if label, ok := map[string]string{"ITEM": "事项", "WORKGROUP": "分工", "TASK": "任务"}[kind]; ok {
			kind = label
		}
		status := tableCell(item["status"])
		if label, ok := map[string]string{"TODO": "待办", "IN_PROGRESS": "进行中", "COMPLETED": "已完成", "CANCELLED": "已取消"}[status]; ok {
			status = label
		}
		deadline := tableCell(item["deadlineDate"])
		if clock, ok := item["deadlineTime"].(string); ok && clock != "" {
			deadline += " " + tableCell(clock)
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", tableCell(item["id"]), kind, status, tableCell(item["title"]), deadline)
	}
	_ = w.Flush()
	if len(items) == 0 {
		b.WriteString("（空）\n")
	}
}

// tableCell 防止标题中的换行和控制符破坏表格边界。
func tableCell(value any) string {
	if value == nil || value == "" {
		return "—"
	}
	return strings.Map(func(c rune) rune {
		if c < 32 || c == 127 {
			return ' '
		}
		return c
	}, fmt.Sprint(value))
}

func human(b *strings.Builder, v any, depth int) {
	pad := strings.Repeat("  ", depth)
	switch x := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(x))
		for key := range x {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			switch x[key].(type) {
			case map[string]any, []any:
				fmt.Fprintf(b, "%s%s:\n", pad, key)
				human(b, x[key], depth+1)
			default:
				fmt.Fprintf(b, "%s%s: %v\n", pad, key, x[key])
			}
		}
	case []any:
		if len(x) == 0 {
			fmt.Fprintf(b, "%s（空）\n", pad)
		}
		for _, item := range x {
			fmt.Fprintf(b, "%s-\n", pad)
			human(b, item, depth+1)
		}
	default:
		fmt.Fprintf(b, "%s%v\n", pad, x)
	}
}

// redact 递归检查解码后的文本，因此转义后的 Token 回显也不会穿过输出边界。
func redact(value any, token string) any {
	if token == "" {
		return value
	}
	switch x := value.(type) {
	case string:
		return strings.ReplaceAll(x, token, "[REDACTED]")
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, v := range x {
			key := strings.ReplaceAll(k, token, "[REDACTED]")
			out[key] = redact(v, token)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, v := range x {
			out[i] = redact(v, token)
		}
		return out
	default:
		return value
	}
}
