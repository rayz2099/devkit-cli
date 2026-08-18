package markdown

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

const (
	packMarkerPrefix = "<<<TL "
	packMarkerSuffix = ">>>"
)

// FastPackBudget 按文档体积选单请求字符预算.
// 为什么看整篇大小而不是固定 1000: heading 碎片才是 RTT 来源, 小文一次打完, 大文用更大桶合并.
func FastPackBudget(docBytes int) int {
	switch {
	case docBytes <= 2<<10:
		return 16 << 10
	case docBytes <= 8<<10:
		return 4 << 10
	default:
		return 8 << 10
	}
}

// PackUnitIndexes 按字符预算把相邻 unit 收成一组.
// 为什么只合并索引不改 TextUnit: Render 仍按原偏移回填, 代码块间隙不会被送进模型.
func PackUnitIndexes(units []TextUnit, budget int) [][]int {
	if len(units) == 0 {
		return nil
	}

	groups := make([][]int, 0, len(units))
	current := []int{0}
	size := utf8.RuneCountInString(units[0].Text)
	for index := 1; index < len(units); index++ {
		next := utf8.RuneCountInString(units[index].Text)
		if size > 0 && size+next > budget {
			groups = append(groups, current)
			current = []int{index}
			size = next
			continue
		}
		current = append(current, index)
		size += next
	}
	return append(groups, current)
}

// EncodePack 把多段原文塞进一次请求.
// 为什么用行级 marker 而不是 JSON: 模型更容易原样保留分隔符, 回填才能拆回 N 段.
func EncodePack(texts []string) string {
	var builder strings.Builder
	builder.WriteString("Keep each <<<TL n>>> marker exactly. Translate only the text inside.\n")
	for index, text := range texts {
		fmt.Fprintf(&builder, "%s\n", packMarker(index))
		builder.WriteString(text)
		if !strings.HasSuffix(text, "\n") {
			builder.WriteByte('\n')
		}
	}
	return builder.String()
}

// DecodePack 按 marker 拆回与 originals 等长的译文.
// 为什么对照 originals 修换行: 回填偏移按原文切片, 缺/多一个尾换行会把后续 AST 拼歪.
func DecodePack(translated string, originals []string) ([]string, error) {
	count := len(originals)
	if count == 0 {
		return nil, fmt.Errorf("fast pack is empty")
	}

	starts := make([]int, count)
	for index := 0; index < count; index++ {
		marker := packMarker(index)
		at := strings.Index(translated, marker)
		if at < 0 {
			return nil, fmt.Errorf("fast pack missing marker %s", marker)
		}
		if index > 0 && at <= starts[index-1] {
			return nil, fmt.Errorf("fast pack marker order invalid at %s", marker)
		}
		starts[index] = at
	}

	out := make([]string, count)
	for index := 0; index < count; index++ {
		start := starts[index] + len(packMarker(index))
		if start < len(translated) && translated[start] == '\n' {
			start++
		}
		stop := len(translated)
		if index+1 < count {
			stop = starts[index+1]
		}
		text := strings.TrimSuffix(translated[start:stop], "\n")
		if strings.HasSuffix(originals[index], "\n") && !strings.HasSuffix(text, "\n") {
			text += "\n"
		}
		out[index] = text
	}
	return out, nil
}

func packMarker(index int) string {
	return fmt.Sprintf("%s%d%s", packMarkerPrefix, index, packMarkerSuffix)
}
