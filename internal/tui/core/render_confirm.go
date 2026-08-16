package core

import (
	"charm.land/lipgloss/v2"
	"github.com/MSmaili/hetki/internal/terminal"
)

type ConfirmModalProps struct {
	LineWidth  int
	Title      string
	Body       string
	Hint       string
	ModalStyle lipgloss.Style
	TitleStyle lipgloss.Style
	HintStyle  lipgloss.Style
}

func RenderConfirmModal(props ConfirmModalProps) string {
	lines := []string{
		props.TitleStyle.Render(terminal.Sanitize(props.Title)),
		terminal.Sanitize(props.Body),
		props.HintStyle.Render(terminal.Sanitize(props.Hint)),
	}
	return renderBox(lines, props.LineWidth, 72, props.ModalStyle)
}
