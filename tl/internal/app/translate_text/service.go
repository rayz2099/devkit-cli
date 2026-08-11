package translate_text

import (
	"context"
	"errors"

	"git.internal.linran.top/linran/tl/internal/domain/translation"
)

type Runner interface {
	Translate(ctx context.Context, direction translation.Direction, text string) (string, error)
}

type ServiceDependencies struct {
	Translator translation.Translator
}

type Service struct {
	translator translation.Translator
}

func New(deps ServiceDependencies) *Service {
	return &Service{translator: deps.Translator}
}

func (s *Service) Translate(ctx context.Context, direction translation.Direction, text string) (string, error) {
	if s.translator == nil {
		return "", errors.New("translator is required")
	}
	return s.translator.Translate(ctx, direction, text)
}
