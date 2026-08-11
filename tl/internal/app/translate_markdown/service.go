package translate_markdown

import (
	"context"
	"sync"
	"sync/atomic"

	"git.internal.linran.top/linran/tl/internal/domain/markdown"
	"git.internal.linran.top/linran/tl/internal/domain/translation"
)

type Progress struct {
	DoneUnits  int
	TotalUnits int
}

type ProgressSink func(Progress)

type Runner interface {
	Translate(ctx context.Context, direction translation.Direction, source []byte) ([]byte, error)
}

type ServiceDependencies struct {
	Translator   translation.Translator
	Concurrency  int
	ProgressSink ProgressSink
}

type Service struct {
	translator   translation.Translator
	concurrency  int
	progressSink ProgressSink
}

func New(deps ServiceDependencies) *Service {
	concurrency := deps.Concurrency
	if concurrency <= 0 {
		concurrency = 8
	}
	return &Service{
		translator:   deps.Translator,
		concurrency:  concurrency,
		progressSink: deps.ProgressSink,
	}
}

func (s *Service) Translate(ctx context.Context, direction translation.Direction, source []byte) ([]byte, error) {
	document, units, err := markdown.Parse(source)
	if err != nil {
		return nil, err
	}
	if len(units) == 0 {
		return source, nil
	}

	translations := make([]string, len(units))
	type job struct {
		index int
		text  string
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	jobs := make(chan job)
	errCh := make(chan error, 1)
	var doneUnits atomic.Int64
	var once sync.Once
	var workers sync.WaitGroup

	for worker := 0; worker < s.concurrency; worker++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for current := range jobs {
				translated, err := s.translator.Translate(ctx, direction, current.text)
				if err != nil {
					once.Do(func() {
						errCh <- err
						cancel()
					})
					return
				}

				translations[current.index] = translated
				if s.progressSink != nil {
					done := int(doneUnits.Add(1))
					s.progressSink(Progress{
						DoneUnits:  done,
						TotalUnits: len(units),
					})
				}
			}
		}()
	}

	for index, unit := range units {
		select {
		case <-ctx.Done():
			break
		case jobs <- job{index: index, text: unit.Text}:
		}
		if ctx.Err() != nil {
			break
		}
	}
	close(jobs)
	workers.Wait()

	select {
	case err := <-errCh:
		return nil, err
	default:
	}

	return document.Render(translations)
}
