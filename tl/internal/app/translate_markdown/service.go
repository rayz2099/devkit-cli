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
	// Fast 只改变打包, 不改 AST 抽取和回填.
	Fast bool
}

type Service struct {
	translator   translation.Translator
	concurrency  int
	progressSink ProgressSink
	fast         bool
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
		fast:         deps.Fast,
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

	jobs, err := s.planJobs(source, units)
	if err != nil {
		return nil, err
	}

	translations, err := s.runJobs(ctx, direction, jobs, len(units))
	if err != nil {
		return nil, err
	}
	return document.Render(translations)
}

type translateJob struct {
	indexes []int
	texts   []string
	payload string
}

// planJobs 默认一 unit 一请求; fast 按文档体积把相邻 unit 收成更少请求.
func (s *Service) planJobs(source []byte, units []markdown.TextUnit) ([]translateJob, error) {
	if !s.fast {
		jobs := make([]translateJob, 0, len(units))
		for index, unit := range units {
			jobs = append(jobs, translateJob{
				indexes: []int{index},
				texts:   []string{unit.Text},
				payload: unit.Text,
			})
		}
		return jobs, nil
	}

	groups := markdown.PackUnitIndexes(units, markdown.FastPackBudget(len(source)))
	jobs := make([]translateJob, 0, len(groups))
	for _, indexes := range groups {
		texts := make([]string, 0, len(indexes))
		for _, index := range indexes {
			texts = append(texts, units[index].Text)
		}
		payload := texts[0]
		if len(texts) > 1 {
			payload = markdown.EncodePack(texts)
		}
		jobs = append(jobs, translateJob{
			indexes: indexes,
			texts:   texts,
			payload: payload,
		})
	}
	return jobs, nil
}

func (s *Service) runJobs(
	ctx context.Context,
	direction translation.Direction,
	jobs []translateJob,
	totalUnits int,
) ([]string, error) {
	translations := make([]string, totalUnits)
	type queueItem struct {
		job translateJob
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	queue := make(chan queueItem)
	errCh := make(chan error, 1)
	var doneUnits atomic.Int64
	var once sync.Once
	var workers sync.WaitGroup

	for worker := 0; worker < s.concurrency; worker++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for current := range queue {
				translated, err := s.translator.Translate(ctx, direction, current.job.payload)
				if err != nil {
					once.Do(func() {
						errCh <- err
						cancel()
					})
					return
				}

				parts, err := splitJobResult(current.job, translated)
				if err != nil {
					once.Do(func() {
						errCh <- err
						cancel()
					})
					return
				}
				for offset, index := range current.job.indexes {
					translations[index] = parts[offset]
				}

				if s.progressSink != nil {
					done := int(doneUnits.Add(int64(len(current.job.indexes))))
					s.progressSink(Progress{
						DoneUnits:  done,
						TotalUnits: totalUnits,
					})
				}
			}
		}()
	}

	for _, job := range jobs {
		select {
		case <-ctx.Done():
			break
		case queue <- queueItem{job: job}:
		}
		if ctx.Err() != nil {
			break
		}
	}
	close(queue)
	workers.Wait()

	select {
	case err := <-errCh:
		return nil, err
	default:
	}
	return translations, nil
}

func splitJobResult(job translateJob, translated string) ([]string, error) {
	if len(job.texts) == 1 {
		return []string{translated}, nil
	}
	return markdown.DecodePack(translated, job.texts)
}
