package core

import (
	"charm.land/lipgloss/v2"
	"github.com/MSmaili/hetki/internal/terminal"
)

type InputModalProps struct {
	LineWidth  int
	Title      string
	Prompt     string
	Value      string
	Hint       string
	ModalStyle lipgloss.Style
	TitleStyle lipgloss.Style
	HintStyle  lipgloss.Style
}

func RenderInputModal(props InputModalProps) string {
	lines := []string{
		props.TitleStyle.Render(terminal.Sanitize(props.Title)),
		terminal.Sanitize(props.Prompt),
		"> " + terminal.Sanitize(props.Value) + "_",
		props.HintStyle.Render(terminal.Sanitize(props.Hint)),
	}
	return renderBox(lines, props.LineWidth, 72, props.ModalStyle)
}
