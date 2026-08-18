package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"git.internal.linran.top/linran/tl/internal/domain/markdown"
	"git.internal.linran.top/linran/tl/internal/domain/translation"
)

func TestRunReadsFromStdinAndWritesMarkdownOnlyToStdout(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	app := New(AppDependencies{
		Stdin:  bytes.NewBufferString("# hello\n"),
		Stdout: &stdout,
		Stderr: &stderr,
		Translator: stubTranslator{
			results: map[string]string{"# hello\n": "# 你好\n"},
		},
		ProgressEnabled: true,
	})

	err := app.Run(context.Background(), []string{"md", "en2zh"})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if got := stdout.String(); got != "# 你好\n" {
		t.Fatalf("stdout = %q, want %q", got, "# 你好\n")
	}

	if got := stderr.String(); got == "" {
		t.Fatal("stderr should contain progress output")
	}
}

func TestRunReadsFromFileWhenPathProvided(t *testing.T) {
	t.Parallel()

	file := writeTempFile(t, "# hello\n")
	var stdout bytes.Buffer

	app := New(AppDependencies{
		Stdout: &stdout,
		Stderr: &bytes.Buffer{},
		Translator: stubTranslator{
			results: map[string]string{"# hello\n": "# 你好\n"},
		},
	})

	err := app.Run(context.Background(), []string{"md", "en2zh", file})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if got := stdout.String(); got != "# 你好\n" {
		t.Fatalf("stdout = %q, want %q", got, "# 你好\n")
	}
}

func TestRunTranslatesPlainTextArgument(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer

	app := New(AppDependencies{
		Stdout: &stdout,
		Stderr: &bytes.Buffer{},
		Translator: stubTranslator{
			results: map[string]string{"hello": "你好"},
		},
	})

	err := app.Run(context.Background(), []string{"en2zh", "hello"})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if got := stdout.String(); got != "你好" {
		t.Fatalf("stdout = %q, want %q", got, "你好")
	}
}

func TestRunReadsPlainTextFromStdinWhenArgumentMissing(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer

	app := New(AppDependencies{
		Stdin:  bytes.NewBufferString("你好"),
		Stdout: &stdout,
		Stderr: &bytes.Buffer{},
		Translator: stubTranslator{
			results: map[string]string{"你好": "hello"},
		},
	})

	err := app.Run(context.Background(), []string{"zh2en"})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if got := stdout.String(); got != "hello" {
		t.Fatalf("stdout = %q, want %q", got, "hello")
	}
}

func TestRunRejectsMultiplePlainTextArguments(t *testing.T) {
	t.Parallel()

	app := New(AppDependencies{
		Stdout:     &bytes.Buffer{},
		Stderr:     &bytes.Buffer{},
		Translator: stubTranslator{results: map[string]string{}},
	})

	err := app.Run(context.Background(), []string{"en2zh", "hello", "world"})
	if err == nil {
		t.Fatal("Run() error = nil, want usage error")
	}
}

func TestRunRejectsUnknownDirection(t *testing.T) {
	t.Parallel()

	app := New(AppDependencies{
		Stdin:      bytes.NewBufferString("# hello\n"),
		Stdout:     &bytes.Buffer{},
		Stderr:     &bytes.Buffer{},
		Translator: stubTranslator{results: map[string]string{"# hello\n": "# 你好\n"}},
	})

	err := app.Run(context.Background(), []string{"md", "oops"})
	if err == nil {
		t.Fatal("Run() error = nil, want invalid direction error")
	}
}

type stubTranslator struct {
	results map[string]string
}

func (s stubTranslator) Translate(_ context.Context, _ translation.Direction, text string) (string, error) {
	result, ok := s.results[text]
	if !ok {
		return "", context.Canceled
	}
	return result, nil
}

func TestRunFastFlagPacksMarkdownUnits(t *testing.T) {
	t.Parallel()

	source := "# hello\n\n## next\n\nworld\n"
	_, units, err := markdown.Parse([]byte(source))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	texts := make([]string, 0, len(units))
	translated := make([]string, 0, len(units))
	for _, unit := range units {
		texts = append(texts, unit.Text)
		item := strings.ReplaceAll(unit.Text, "hello", "你好")
		item = strings.ReplaceAll(item, "next", "下一节")
		item = strings.ReplaceAll(item, "world", "世界")
		translated = append(translated, item)
	}

	var stdout bytes.Buffer
	app := New(AppDependencies{
		Stdin:  bytes.NewBufferString(source),
		Stdout: &stdout,
		Stderr: &bytes.Buffer{},
		Translator: stubTranslator{
			results: map[string]string{
				markdown.EncodePack(texts): markdown.EncodePack(translated),
			},
		},
	})

	err = app.Run(context.Background(), []string{"md", "en2zh", "--fast"})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	got := stdout.String()
	if !strings.Contains(got, "# 你好") || !strings.Contains(got, "## 下一节") {
		t.Fatalf("stdout = %q, want packed fast translation", got)
	}
}
