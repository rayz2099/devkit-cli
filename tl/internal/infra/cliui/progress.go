package cliui

import (
	"fmt"
	"io"
	"strings"

	"git.internal.linran.top/linran/tl/internal/app/translate_markdown"
)

type ProgressBar struct {
	writer      io.Writer
	interactive bool
}

func NewProgressBar(writer io.Writer, interactive bool) *ProgressBar {
	return &ProgressBar{
		writer:      writer,
		interactive: interactive,
	}
}

func (p *ProgressBar) Report(progress translate_markdown.Progress) {
	if !p.interactive || progress.TotalUnits == 0 {
		return
	}

	filled := progress.DoneUnits * 20 / progress.TotalUnits
	bar := strings.Repeat("#", filled) + strings.Repeat("-", 20-filled)
	line := fmt.Sprintf("\r[%s] %d/%d", bar, progress.DoneUnits, progress.TotalUnits)
	if progress.DoneUnits == progress.TotalUnits {
		line += "\n"
	}
	_, _ = io.WriteString(p.writer, line)
}
