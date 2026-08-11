package markdown

import (
	"strings"
	"testing"
)

func TestParseGroupsMarkdownBySectionInsteadOfLeafText(t *testing.T) {
	t.Parallel()

	source := strings.Join([]string{
		"---",
		"title: demo",
		"---",
		"# Hello",
		"",
		"Visit [site](https://example.com).",
		"",
		"- item one",
		"- item two",
		"",
		"```go",
		`fmt.Println("hello")`,
		"```",
		"",
		"## Next",
		"",
		"More text.",
		"",
	}, "\n")

	doc, units, err := Parse([]byte(source))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	gotTexts := unitTexts(units)
	wantTexts := []string{
		"# Hello\n\nVisit [site](https://example.com).\n\n- item one\n- item two\n",
		"## Next\n\nMore text.\n",
	}
	if strings.Join(gotTexts, "||") != strings.Join(wantTexts, "||") {
		t.Fatalf("texts = %#v, want %#v", gotTexts, wantTexts)
	}

	output, err := doc.Render([]string{
		"# 你好\n\n访问 [站点](https://example.com).\n\n- 条目一\n- 条目二\n",
		"## 下一节\n\n更多内容。\n",
	})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	got := string(output)
	assertContains(t, got, "---\ntitle: demo\n---")
	assertContains(t, got, "# 你好")
	assertContains(t, got, "访问 [站点](https://example.com).")
	assertContains(t, got, "- 条目一")
	assertContains(t, got, `fmt.Println("hello")`)
	assertContains(t, got, "## 下一节")
}

func TestParseSplitsLongSectionByLength(t *testing.T) {
	t.Parallel()

	longSentence := strings.Repeat("a", 1005) + ".\n"
	source := "# Title\n\n" + longSentence

	_, units, err := Parse([]byte(source))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if len(units) < 2 {
		t.Fatalf("unit count = %d, want at least 2", len(units))
	}
}

func unitTexts(units []TextUnit) []string {
	texts := make([]string, 0, len(units))
	for _, unit := range units {
		texts = append(texts, unit.Text)
	}
	return texts
}

func assertContains(t *testing.T, got string, want string) {
	t.Helper()
	if !strings.Contains(got, want) {
		t.Fatalf("output = %q, want substring %q", got, want)
	}
}
