package components

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/MSmaili/hetki/internal/terminal"
)

type HelpEntry struct {
	Keys string
	Desc string
}

type HelpSection struct {
	Title   string
	Entries []HelpEntry
}

type HelpOverlayProps struct {
	LineWidth    int
	Title        string
	Hint         string
	Sections     []HelpSection
	OverlayStyle lipgloss.Style
	TitleStyle   lipgloss.Style
	MetaStyle    lipgloss.Style
	KeyStyle     lipgloss.Style
	HintStyle    lipgloss.Style
}

func RenderHelpOverlay(props HelpOverlayProps) string {
	keyColW := 0
	for _, s := range props.Sections {
		for _, e := range s.Entries {
			if w := terminal.Width(terminal.Sanitize(e.Keys)); w > keyColW {
				keyColW = w
			}
		}
	}

	var lines []string
	lines = append(lines, props.TitleStyle.Render(terminal.Sanitize(props.Title)))
	for _, s := range props.Sections {
		lines = append(lines, "")
		lines = append(lines, props.MetaStyle.Render(terminal.Sanitize(s.Title)))
		for _, e := range s.Entries {
			keys := terminal.Sanitize(e.Keys)
			pad := keyColW - terminal.Width(keys)
			if pad < 0 {
				pad = 0
			}
			lines = append(lines, "  "+props.KeyStyle.Render(keys)+strings.Repeat(" ", pad)+"  "+terminal.Sanitize(e.Desc))
		}
	}
	lines = append(lines, "")
	lines = append(lines, props.HintStyle.Render(terminal.Sanitize(props.Hint)))

	return renderBox(lines, props.LineWidth, 76, props.OverlayStyle)
}
