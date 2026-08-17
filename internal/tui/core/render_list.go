package core

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/MSmaili/hetki/internal/terminal"
)

type TreeStyles struct {
	Meta              lipgloss.Style
	Row               lipgloss.Style
	RootRow           lipgloss.Style
	ChildRow          lipgloss.Style
	Secondary         lipgloss.Style
	SecondarySelected lipgloss.Style
	ActiveRow         lipgloss.Style
	SelectedRow       lipgloss.Style
	JumpLabel         lipgloss.Style
	Rail              lipgloss.Style
}

type TreeRowProps struct {
	ItemID     string
	Primary    string
	Secondary  string
	JumpLabel  string
	Depth      int
	TreePrefix string
	Expanded   bool
	Branch     bool
	Active     bool
	Selected   bool
}

type TreeProps struct {
	Width     int
	EmptyText string
	Rows      []TreeRowProps
	Compact   bool
	Styles    TreeStyles
}

func RenderTree(props TreeProps) []string {
	if len(props.Rows) == 0 {
		return []string{props.Styles.Meta.Render(truncateWidth(props.EmptyText, props.Width))}
	}
	lines := make([]string, 0, len(props.Rows))
	for _, row := range props.Rows {
		line := renderRowLine(row, props.Styles, props.Compact)
		line = composeRowLine(row, line, props.Width, props.Compact, props.Styles)
		lines = append(lines, styleRowLine(row, line, props.Width, props.Styles))
	}
	return lines
}

func composeRowLine(row TreeRowProps, left string, width int, compact bool, styles TreeStyles) string {
	secondary := strings.TrimSpace(terminal.Sanitize(row.Secondary))
	if compact || secondary == "" {
		return truncateWidth(left, width)
	}
	const gap = 2
	const minSecondaryWidth = 6
	leftWidth := terminal.Width(left)
	available := width - leftWidth - gap
	if available < minSecondaryWidth {
		return truncateWidth(left, width)
	}
	shortened := shortenPath(secondary, available)
	padding := max(gap, width-leftWidth-terminal.Width(shortened))
	style := styles.Secondary
	if row.Selected {
		style = styles.SecondarySelected
	}
	return left + strings.Repeat(" ", padding) + style.Render(shortened)
}

func shortenPath(path string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	pathWidth := terminal.Width(path)
	if pathWidth <= maxWidth {
		return path
	}
	const ellipsis = "..."
	if maxWidth <= len(ellipsis) {
		return terminal.Truncate(path, maxWidth)
	}
	if index := strings.LastIndex(path, "/"); index > 0 {
		tail := path[index:]
		if tailWidth := terminal.Width(tail); tailWidth+len(ellipsis)+1 <= maxWidth {
			headBudget := maxWidth - len(ellipsis) - tailWidth
			return terminal.Cut(path, 0, headBudget) + ellipsis + tail
		}
	}
	available := maxWidth - len(ellipsis)
	headWidth := available - available/2
	tailWidth := available / 2
	return terminal.Cut(path, 0, headWidth) + ellipsis + terminal.Cut(path, pathWidth-tailWidth, pathWidth)
}

func renderRowLine(row TreeRowProps, styles TreeStyles, compact bool) string {
	jumpLabel := strings.TrimSpace(terminal.Sanitize(row.JumpLabel))
	if jumpLabel != "" {
		jumpLabel = styles.JumpLabel.Render(jumpLabel) + " "
	}
	cursor := " "
	if row.Selected {
		cursor = "❯"
	}
	marker := "  "
	if row.Active {
		marker = "● "
	}
	return jumpLabel + fmt.Sprintf("%s %s%s", cursor, marker, decoratedLabel(row, styles, compact))
}

func styleRowLine(row TreeRowProps, line string, width int, styles TreeStyles) string {
	if row.Selected {
		return styles.SelectedRow.Width(width).Render(line)
	}
	if row.Depth == 0 {
		return styles.RootRow.Render(line)
	}
	if row.Active {
		return styles.ActiveRow.Render(line)
	}
	if row.Depth > 0 {
		return styles.ChildRow.Render(line)
	}
	return styles.Row.Render(line)
}

func decoratedLabel(row TreeRowProps, styles TreeStyles, compact bool) string {
	primary := strings.TrimSpace(terminal.Sanitize(row.Primary))
	if primary == "" {
		primary = terminal.Sanitize(row.ItemID)
	}
	if compact {
		return strings.Repeat("  ", max(row.Depth-1, 0)) + primary
	}
	prefix := row.TreePrefix
	branch := branchGlyph(row)
	if !row.Selected {
		prefix = styles.Rail.Render(prefix)
		branch = styles.Rail.Render(branch)
	}
	if row.Depth == 0 {
		return fmt.Sprintf("%s %s", branch, primary)
	}
	return fmt.Sprintf("  %s%s", prefix, primary)
}

func branchGlyph(row TreeRowProps) string {
	if !row.Branch {
		return "  "
	}
	if row.Expanded {
		return "\uf0d7 "
	}
	return "\uf0da "
}
