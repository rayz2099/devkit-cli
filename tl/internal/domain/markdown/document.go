package markdown

import (
	"bytes"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	gmtext "github.com/yuin/goldmark/text"
)

const defaultMaxUnitChars = 1000

type TextUnit struct {
	Start int
	Stop  int
	Text  string
}

type Document struct {
	prefix []byte
	body   []byte
	units  []TextUnit
}

type blockRange struct {
	start int
	stop  int
	kind  ast.NodeKind
}

func Parse(source []byte) (*Document, []TextUnit, error) {
	prefix, body := splitFrontMatter(source)
	parser := goldmark.New(goldmark.WithExtensions(extension.GFM))
	root := parser.Parser().Parse(gmtext.NewReader(body))

	units, err := collectTextUnits(root, body, defaultMaxUnitChars)
	if err != nil {
		return nil, nil, err
	}

	document := &Document{
		prefix: append([]byte(nil), prefix...),
		body:   append([]byte(nil), body...),
		units:  append([]TextUnit(nil), units...),
	}
	return document, append([]TextUnit(nil), units...), nil
}

func (d *Document) Render(translations []string) ([]byte, error) {
	if len(translations) != len(d.units) {
		return nil, fmt.Errorf("translation count mismatch: got %d want %d", len(translations), len(d.units))
	}

	body := append([]byte(nil), d.body...)
	for index := len(d.units) - 1; index >= 0; index-- {
		unit := d.units[index]
		body = bytes.Join([][]byte{
			body[:unit.Start],
			[]byte(translations[index]),
			body[unit.Stop:],
		}, nil)
	}
	return append(append([]byte(nil), d.prefix...), body...), nil
}

func collectTextUnits(root ast.Node, source []byte, maxChars int) ([]TextUnit, error) {
	blocks, err := collectBlocks(root, source)
	if err != nil {
		return nil, err
	}
	if len(blocks) == 0 {
		return nil, nil
	}

	sections := groupIntoSections(blocks, source)
	units := make([]TextUnit, 0, len(sections))
	for _, section := range sections {
		units = append(units, splitSection(section, source, maxChars)...)
	}
	return units, nil
}

func collectBlocks(root ast.Node, source []byte) ([]blockRange, error) {
	blocks := make([]blockRange, 0)
	err := ast.Walk(root, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering || !isTranslatableBlock(node) {
			return ast.WalkContinue, nil
		}

		start, stop, ok := rawBlockRange(node, source)
		if !ok {
			return ast.WalkContinue, nil
		}
		if strings.TrimSpace(string(source[start:stop])) == "" {
			return ast.WalkContinue, nil
		}

		blocks = append(blocks, blockRange{
			start: start,
			stop:  stop,
			kind:  node.Kind(),
		})
		return ast.WalkContinue, nil
	})
	if err != nil {
		return nil, err
	}
	return blocks, nil
}

func isTranslatableBlock(node ast.Node) bool {
	switch node.Kind() {
	case ast.KindHeading, ast.KindParagraph, ast.KindTextBlock:
		return true
	default:
		return false
	}
}

func rawBlockRange(node ast.Node, source []byte) (int, int, bool) {
	lines := node.Lines()
	if lines == nil || lines.Len() == 0 {
		return 0, 0, false
	}

	first := lines.At(0)
	last := lines.At(lines.Len() - 1)
	start := lineStart(source, first.Start)
	stop := lineStop(source, last.Stop)
	if start >= stop {
		return 0, 0, false
	}
	return start, stop, true
}

func lineStart(source []byte, offset int) int {
	for offset > 0 && source[offset-1] != '\n' {
		offset--
	}
	return offset
}

func lineStop(source []byte, offset int) int {
	for offset < len(source) && source[offset] != '\n' {
		offset++
	}
	if offset < len(source) {
		offset++
	}
	return offset
}

func groupIntoSections(blocks []blockRange, source []byte) [][]blockRange {
	sections := make([][]blockRange, 0)
	current := make([]blockRange, 0)

	for _, block := range blocks {
		if len(current) == 0 {
			current = append(current, block)
			continue
		}

		previous := current[len(current)-1]
		if block.kind == ast.KindHeading || !onlyWhitespace(source[previous.stop:block.start]) {
			sections = append(sections, current)
			current = []blockRange{block}
			continue
		}

		current = append(current, block)
	}

	if len(current) > 0 {
		sections = append(sections, current)
	}
	return sections
}

func onlyWhitespace(source []byte) bool {
	return len(strings.TrimSpace(string(source))) == 0
}

func splitSection(section []blockRange, source []byte, maxChars int) []TextUnit {
	units := make([]TextUnit, 0, len(section))
	currentStart := section[0].start
	currentStop := section[0].stop

	for index := 1; index < len(section); index++ {
		next := section[index]
		if utf8.RuneCount(source[currentStart:next.stop]) <= maxChars {
			currentStop = next.stop
			continue
		}

		units = append(units, splitRange(currentStart, currentStop, source[currentStart:currentStop], maxChars)...)
		currentStart = next.start
		currentStop = next.stop
	}

	units = append(units, splitRange(currentStart, currentStop, source[currentStart:currentStop], maxChars)...)
	return units
}

func splitRange(start int, stop int, source []byte, maxChars int) []TextUnit {
	if utf8.RuneCount(source) <= maxChars {
		return []TextUnit{{
			Start: start,
			Stop:  stop,
			Text:  string(source),
		}}
	}

	units := make([]TextUnit, 0)
	offset := 0
	for offset < len(source) {
		chunkLen := nextSplitOffset(source[offset:], maxChars)
		units = append(units, TextUnit{
			Start: start + offset,
			Stop:  start + offset + chunkLen,
			Text:  string(source[offset : offset+chunkLen]),
		})
		offset += chunkLen
	}
	return units
}

func nextSplitOffset(source []byte, maxChars int) int {
	if utf8.RuneCount(source) <= maxChars {
		return len(source)
	}

	best := -1
	count := 0
	for offset, r := range string(source) {
		count++
		if count > maxChars {
			break
		}
		if strings.ContainsRune(".!?。！？\n", r) {
			best = offset + utf8.RuneLen(r)
		}
	}
	if best > 0 {
		return best
	}

	count = 0
	for offset, r := range string(source) {
		count++
		if count > maxChars {
			break
		}
		if r == ' ' || r == '\n' || r == '\t' {
			best = offset + utf8.RuneLen(r)
		}
	}
	if best > 0 {
		return best
	}

	count = 0
	for offset, r := range string(source) {
		count++
		if count == maxChars {
			return offset + utf8.RuneLen(r)
		}
	}
	return len(source)
}

func splitFrontMatter(source []byte) ([]byte, []byte) {
	if !bytes.HasPrefix(source, []byte("---\n")) {
		return nil, source
	}

	lines := bytes.SplitAfter(source, []byte("\n"))
	if len(lines) < 3 {
		return nil, source
	}

	var offset int
	for index, line := range lines[1:] {
		offset += len(lines[index])
		trimmed := strings.TrimSpace(string(line))
		if trimmed == "---" || trimmed == "..." {
			offset += len(line)
			return source[:offset], source[offset:]
		}
	}
	return nil, source
}
