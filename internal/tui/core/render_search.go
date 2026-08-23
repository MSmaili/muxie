package core

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/MSmaili/hetki/internal/terminal"
)

type SearchBarProps struct {
	Width            int
	Filter           string
	Right            string
	Active           bool
	Compact          bool
	Style            lipgloss.Style
	PromptStyle      lipgloss.Style
	PlaceholderStyle lipgloss.Style
	MetaStyle        lipgloss.Style
}

func RenderSearchBar(props SearchBarProps) string {
	if props.Width <= 0 {
		return ""
	}
	search := strings.TrimSpace(terminal.Sanitize(props.Filter))
	text := " "
	placeholder := false
	switch {
	case search != "":
		text += search
		if props.Active {
			text += "_"
		}
	case props.Active:
		text += "_"
	case props.Compact:
		text += "search"
		placeholder = true
	default:
		text += "search destinations"
		placeholder = true
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
	prompt := props.PromptStyle.Render(" / ")
	left := terminal.Cut(prompt, 0, leftW)
	if promptW := terminal.Width(prompt); leftW > promptW {
		text = truncateWidth(text, leftW-promptW)
		style := props.Style
		if placeholder {
			style = props.PlaceholderStyle
		}
		left = prompt + style.Render(text)
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
