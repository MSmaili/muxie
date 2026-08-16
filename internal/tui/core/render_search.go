package core

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/MSmaili/hetki/internal/terminal"
)

type SearchBarProps struct {
	Width       int
	Filter      string
	Right       string
	Active      bool
	Compact     bool
	Style       lipgloss.Style
	PromptStyle lipgloss.Style
	MetaStyle   lipgloss.Style
}

func RenderSearchBar(props SearchBarProps) string {
	search := strings.TrimSpace(terminal.Sanitize(props.Filter))
	prompt := "\uf002 "
	if props.Compact && search == "" && !props.Active {
		prompt = "\uf002 search"
	}
	content := prompt + search
	if props.Active {
		content += "_"
	} else if search == "" {
		content = prompt
	}
	if props.Width <= 0 {
		return ""
	}

	rightText := terminal.Sanitize(props.Right)
	right := props.MetaStyle.Render(rightText)
	rightW := terminal.Width(right)
	if rightW >= props.Width {
		return props.MetaStyle.Render(truncateWidth(rightText, props.Width))
	}
	leftW := props.Width
	if rightW > 0 {
		leftW -= rightW + 1
	}
	content = truncateWidth(content, leftW)
	left := props.Style.Render(content)
	if strings.HasPrefix(content, prompt) {
		left = props.PromptStyle.Render(prompt) + props.Style.Render(strings.TrimPrefix(content, prompt))
	}
	if rightW == 0 {
		return left
	}
	gap := props.Width - terminal.Width(left) - rightW
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}
