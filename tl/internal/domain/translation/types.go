package translation

import "context"

type Direction string

const (
	DirectionEnToZh Direction = "en2zh"
	DirectionZhToEn Direction = "zh2en"
)

type Translator interface {
	Translate(ctx context.Context, direction Direction, text string) (string, error)
}
