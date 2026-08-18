package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"git.internal.linran.top/linran/tl/internal/app/translate_markdown"
	"git.internal.linran.top/linran/tl/internal/app/translate_text"
	"git.internal.linran.top/linran/tl/internal/domain/translation"
	"git.internal.linran.top/linran/tl/internal/infra/cliui"
)

type AppDependencies struct {
	Stdin              io.Reader
	Stdout             io.Writer
	Stderr             io.Writer
	MarkdownTranslator translate_markdown.Runner
	TextTranslator     translate_text.Runner
	Translator         translation.Translator
	Concurrency        int
	ProgressEnabled    bool
}

type App struct {
	stdin              io.Reader
	stdout             io.Writer
	stderr             io.Writer
	markdownTranslator translate_markdown.Runner
	textTranslator     translate_text.Runner
	translator         translation.Translator
	concurrency        int
	progressEnabled    bool
}

func New(deps AppDependencies) *App {
	return &App{
		stdin:              valueOrDefaultReader(deps.Stdin, os.Stdin),
		stdout:             valueOrDefaultWriter(deps.Stdout, os.Stdout),
		stderr:             valueOrDefaultWriter(deps.Stderr, os.Stderr),
		markdownTranslator: deps.MarkdownTranslator,
		textTranslator:     deps.TextTranslator,
		translator:         deps.Translator,
		concurrency:        deps.Concurrency,
		progressEnabled:    deps.ProgressEnabled,
	}
}

func (a *App) Run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New(rootUsage)
	}

	if args[0] == "md" {
		return a.runMarkdown(ctx, args[1:])
	}

	return a.runText(ctx, args)
}

func (a *App) runMarkdown(ctx context.Context, args []string) error {
	direction, paths, fast, err := parseMarkdownArgs(args)
	if err != nil {
		return err
	}
	input, err := a.readMarkdownInput(paths)
	if err != nil {
		return err
	}

	translator, err := a.resolveMarkdownTranslator(fast)
	if err != nil {
		return err
	}

	output, err := translator.Translate(ctx, direction, input)
	if err != nil {
		return err
	}

	_, err = a.stdout.Write(output)
	return err
}

func (a *App) runText(ctx context.Context, args []string) error {
	direction, err := parseDirection(args[0])
	if err != nil {
		return fmt.Errorf("unsupported command: %s", args[0])
	}

	input, err := a.readTextInput(args[1:])
	if err != nil {
		return err
	}

	translator, err := a.resolveTextTranslator()
	if err != nil {
		return err
	}

	output, err := translator.Translate(ctx, direction, string(input))
	if err != nil {
		return err
	}

	_, err = a.stdout.Write([]byte(output))
	return err
}

func (a *App) readMarkdownInput(paths []string) ([]byte, error) {
	if len(paths) == 0 {
		return io.ReadAll(a.stdin)
	}
	return os.ReadFile(paths[0])
}

func (a *App) readTextInput(args []string) ([]byte, error) {
	switch len(args) {
	case 0:
		return io.ReadAll(a.stdin)
	case 1:
		return []byte(args[0]), nil
	default:
		return nil, errors.New(rootUsage)
	}
}

func valueOrDefaultReader(current io.Reader, alt io.Reader) io.Reader {
	if current != nil {
		return current
	}
	return alt
}

func valueOrDefaultWriter(current io.Writer, alt io.Writer) io.Writer {
	if current != nil {
		return current
	}
	return alt
}

func parseDirection(raw string) (translation.Direction, error) {
	direction := translation.Direction(raw)
	switch direction {
	case translation.DirectionEnToZh, translation.DirectionZhToEn:
		return direction, nil
	default:
		return "", fmt.Errorf("unsupported direction: %s", raw)
	}
}

func (a *App) resolveMarkdownTranslator(fast bool) (translate_markdown.Runner, error) {
	if a.markdownTranslator != nil {
		return a.markdownTranslator, nil
	}
	if a.translator == nil {
		return nil, errors.New("translator is required")
	}
	var progressSink translate_markdown.ProgressSink
	if a.progressEnabled {
		progressBar := cliui.NewProgressBar(a.stderr, true)
		progressSink = progressBar.Report
	}
	return translate_markdown.New(translate_markdown.ServiceDependencies{
		Translator:   a.translator,
		Concurrency:  a.concurrency,
		ProgressSink: progressSink,
		Fast:         fast,
	}), nil
}

// parseMarkdownArgs 只剥离 --fast, 其余位置语义与旧命令兼容.
func parseMarkdownArgs(args []string) (translation.Direction, []string, bool, error) {
	rest := make([]string, 0, len(args))
	fast := false
	for _, arg := range args {
		if arg == "--fast" {
			fast = true
			continue
		}
		rest = append(rest, arg)
	}
	if len(rest) == 0 {
		return "", nil, fast, errors.New(markdownUsage)
	}
	direction, err := parseDirection(rest[0])
	if err != nil {
		return "", nil, fast, err
	}
	return direction, rest[1:], fast, nil
}

func (a *App) resolveTextTranslator() (translate_text.Runner, error) {
	if a.textTranslator != nil {
		return a.textTranslator, nil
	}
	if a.translator == nil {
		return nil, errors.New("translator is required")
	}
	return translate_text.New(translate_text.ServiceDependencies{
		Translator: a.translator,
	}), nil
}

const (
	rootUsage     = "usage: tl <en2zh|zh2en> [text]"
	markdownUsage = "usage: tl md <en2zh|zh2en> [--fast] [file]"
)
