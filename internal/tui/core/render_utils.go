package core

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/MSmaili/hetki/internal/terminal"
)

func truncateWidth(s string, width int) string { return terminal.Truncate(s, width) }

func renderBox(lines []string, lineWidth, maxContentWidth int, style lipgloss.Style) string {
	frameWidth := style.GetHorizontalFrameSize()
	if lineWidth <= frameWidth {
		for i := range lines {
			lines[i] = truncateWidth(lines[i], lineWidth)
		}
		return strings.Join(lines, "\n")
	}
	contentWidth := min(maxContentWidth, lineWidth-frameWidth)
	for i := range lines {
		lines[i] = truncateWidth(lines[i], contentWidth)
	}
	return style.Width(contentWidth).Render(strings.Join(lines, "\n"))
}
