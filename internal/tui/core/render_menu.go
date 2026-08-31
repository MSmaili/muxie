package core

import (
	"fmt"

	"charm.land/lipgloss/v2"
	"github.com/MSmaili/hetki/internal/terminal"
)

type MenuProps struct {
	LineWidth     int
	MaxHeight     int
	Title         string
	Entries       []MenuEntry
	Selected      int
	Hint          string
	ModalStyle    lipgloss.Style
	TitleStyle    lipgloss.Style
	EntryStyle    lipgloss.Style
	KeyStyle      lipgloss.Style
	SelectedStyle lipgloss.Style
	HintStyle     lipgloss.Style
}

func RenderMenu(props MenuProps) string {
	style := props.ModalStyle
	entries, selected := props.Entries, props.Selected
	showTitle, showHint := true, true
	if props.MaxHeight > 0 {
		if props.MaxHeight <= style.GetVerticalFrameSize() {
			style = lipgloss.NewStyle()
		}
		contentRows := max(1, props.MaxHeight-style.GetVerticalFrameSize())
		showTitle = contentRows >= 2
		showHint = contentRows >= 3
		entryRows := contentRows
		if showTitle {
			entryRows--
		}
		if showHint {
			entryRows--
		}
		entries, selected = visibleMenuEntries(entries, selected, entryRows)
	}

	lines := make([]string, 0, len(entries)+2)
	if showTitle {
		lines = append(lines, props.TitleStyle.Render(terminal.Sanitize(props.Title)))
	}
	for i, entry := range entries {
		key := fmt.Sprintf("‹%c›", entry.Activation)
		label := terminal.Sanitize(entry.Label)
		if i == selected {
			lines = append(lines, props.SelectedStyle.Render(key+" "+label))
			continue
		}
		lines = append(lines, props.KeyStyle.Render(key)+" "+props.EntryStyle.Render(label))
	}
	if showHint {
		lines = append(lines, props.HintStyle.Render(terminal.Sanitize(props.Hint)))
	}
	return renderBox(lines, props.LineWidth, 48, style)
}

func visibleMenuEntries(entries []MenuEntry, selected, limit int) ([]MenuEntry, int) {
	limit = max(1, limit)
	if len(entries) <= limit {
		return entries, selected
	}
	selected = max(0, min(selected, len(entries)-1))
	start := min(max(0, selected-limit+1), len(entries)-limit)
	return entries[start : start+limit], selected - start
}
