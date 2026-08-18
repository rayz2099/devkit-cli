package translate_markdown

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"git.internal.linran.top/linran/tl/internal/domain/markdown"
	"git.internal.linran.top/linran/tl/internal/domain/translation"
)

func TestNewDefaultsConcurrencyToEight(t *testing.T) {
	t.Parallel()

	service := New(ServiceDependencies{
		Translator: &recordingTranslator{},
	})

	if service.concurrency != 8 {
		t.Fatalf("default concurrency = %d, want 8", service.concurrency)
	}
}

func TestServiceTranslatesEachUnitIndividuallyAndReportsProgress(t *testing.T) {
	t.Parallel()

	translator := &recordingTranslator{
		results: map[string]string{
			"# hello\n\nworld\n\n[site](https://example.com)\n": "# 你好\n\n世界\n\n[站点](https://example.com)\n",
		},
	}
	var events []Progress

	service := New(ServiceDependencies{
		Translator:   translator,
		Concurrency:  2,
		ProgressSink: func(progress Progress) { events = append(events, progress) },
	})

	output, err := service.Translate(
		context.Background(),
		translation.DirectionEnToZh,
		[]byte("# hello\n\nworld\n\n[site](https://example.com)\n"),
	)
	if err != nil {
		t.Fatalf("Translate() error = %v", err)
	}

	gotCalls := strings.Join(translator.calls, "|")
	wantCalls := strings.Join([]string{"# hello\n\nworld\n\n[site](https://example.com)\n"}, "|")
	if gotCalls != wantCalls {
		t.Fatalf("calls = %q, want %q", gotCalls, wantCalls)
	}

	if len(events) != 1 {
		t.Fatalf("progress event count = %d, want %d", len(events), 1)
	}
	if events[0].DoneUnits != 1 || events[0].TotalUnits != 1 {
		t.Fatalf("progress = %#v, want done=1 total=1", events[0])
	}

	gotOutput := string(output)
	if !strings.Contains(gotOutput, "# 你好") {
		t.Fatalf("output = %q, want translated heading", gotOutput)
	}
	if !strings.Contains(gotOutput, "世界") {
		t.Fatalf("output = %q, want translated paragraph", gotOutput)
	}
	if !strings.Contains(gotOutput, "[站点](https://example.com)") {
		t.Fatalf("output = %q, want translated link text with preserved url", gotOutput)
	}
}

func TestServiceReturnsErrorWhenWorkerFails(t *testing.T) {
	t.Parallel()

	service := New(ServiceDependencies{
		Translator: &recordingTranslator{
			results: map[string]string{
				"# hello\n": "# 你好\n",
			},
			errors: map[string]error{
				"## next\n\nworld\n": errors.New("boom"),
			},
		},
	})

	_, err := service.Translate(
		context.Background(),
		translation.DirectionEnToZh,
		[]byte("# hello\n\n## next\n\nworld\n"),
	)
	if err == nil {
		t.Fatal("Translate() error = nil, want worker error")
	}
}

type recordingTranslator struct {
	mu      sync.Mutex
	results map[string]string
	errors  map[string]error
	delays  map[string]time.Duration
	calls   []string
}

func (r *recordingTranslator) Translate(_ context.Context, _ translation.Direction, text string) (string, error) {
	r.mu.Lock()
	r.calls = append(r.calls, text)
	delay := r.delays[text]
	err := r.errors[text]
	result := r.results[text]
	r.mu.Unlock()

	if delay > 0 {
		time.Sleep(delay)
	}
	if err != nil {
		return "", err
	}
	return result, nil
}

func TestServiceFastModePacksUnitsIntoFewerRequests(t *testing.T) {
	t.Parallel()

	source := []byte("# hello\n\n## next\n\nworld\n")
	_, units, err := markdown.Parse(source)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if len(units) < 2 {
		t.Fatalf("unit count = %d, want at least 2", len(units))
	}

	texts := make([]string, 0, len(units))
	for _, unit := range units {
		texts = append(texts, unit.Text)
	}
	translated := make([]string, len(texts))
	for i, text := range texts {
		translated[i] = strings.ReplaceAll(text, "hello", "你好")
		translated[i] = strings.ReplaceAll(translated[i], "next", "下一节")
		translated[i] = strings.ReplaceAll(translated[i], "world", "世界")
	}

	translator := &recordingTranslator{
		results: map[string]string{
			markdown.EncodePack(texts): markdown.EncodePack(translated),
		},
	}
	var events []Progress
	service := New(ServiceDependencies{
		Translator:   translator,
		Fast:         true,
		ProgressSink: func(progress Progress) { events = append(events, progress) },
	})

	output, err := service.Translate(context.Background(), translation.DirectionEnToZh, source)
	if err != nil {
		t.Fatalf("Translate() error = %v", err)
	}
	if len(translator.calls) != 1 {
		t.Fatalf("call count = %d, want 1 packed request; calls=%q", len(translator.calls), translator.calls)
	}
	if len(events) != 1 || events[0].DoneUnits != len(units) || events[0].TotalUnits != len(units) {
		t.Fatalf("progress = %#v, want one event covering %d units", events, len(units))
	}
	got := string(output)
	if !strings.Contains(got, "# 你好") || !strings.Contains(got, "## 下一节") || !strings.Contains(got, "世界") {
		t.Fatalf("output = %q, want packed translations spliced back", got)
	}
}

func TestServiceDefaultModeKeepsOneRequestPerUnit(t *testing.T) {
	t.Parallel()

	source := []byte("# hello\n\n## next\n\nworld\n")
	translator := &recordingTranslator{
		results: map[string]string{
			"# hello\n":          "# 你好\n",
			"## next\n\nworld\n": "## 下一节\n\n世界\n",
		},
	}
	service := New(ServiceDependencies{Translator: translator})
	output, err := service.Translate(context.Background(), translation.DirectionEnToZh, source)
	if err != nil {
		t.Fatalf("Translate() error = %v", err)
	}
	if len(translator.calls) != 2 {
		t.Fatalf("call count = %d, want 2; calls=%q", len(translator.calls), translator.calls)
	}
	got := string(output)
	if !strings.Contains(got, "# 你好") || !strings.Contains(got, "## 下一节") {
		t.Fatalf("output = %q, want per-unit translations", got)
	}
}
