package core

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/MSmaili/hetki/internal/terminal"
)

type SearchBarProps struct {
	Width       int
	Filter      string
	Hint        string
	Right       string
	Prompt      string
	Active      bool
	Style       lipgloss.Style
	PromptStyle lipgloss.Style
	HintStyle   lipgloss.Style
	MetaStyle   lipgloss.Style
}

func RenderSearchBar(props SearchBarProps) string {
	if props.Width <= 0 {
		return ""
	}
	search := strings.TrimSpace(terminal.Sanitize(props.Filter))
	text := ""
	if search != "" {
		text = " " + search
	}
	if props.Active {
		text += "█"
		if search == "" {
			text = " " + text
		}
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
	prompt := props.PromptStyle.Render(terminal.Sanitize(props.Prompt))
	left := terminal.Cut(prompt, 0, leftW)
	if promptW := terminal.Width(prompt); leftW > promptW {
		text = truncateWidth(text, leftW-promptW)
		left = prompt + props.Style.Render(text)
	}
	leftW = terminal.Width(left)
	rightX := props.Width - rightW
	hintText := strings.TrimSpace(terminal.Sanitize(props.Hint))
	hint := props.HintStyle.Render(hintText)
	hintW := terminal.Width(hint)
	hintX := (props.Width - hintW) / 2
	if hintW > 0 && hintX > leftW && hintX+hintW < rightX {
		return left + strings.Repeat(" ", hintX-leftW) + hint +
			strings.Repeat(" ", rightX-hintX-hintW) + right
	}
	if rightW == 0 {
		return left
	}
	gap := props.Width - leftW - rightW
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}
