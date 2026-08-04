package components

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/MSmaili/hetki/internal/tui/contracts"
)

type TreeStyles struct {
	Meta        lipgloss.Style
	Row         lipgloss.Style
	SessionRow  lipgloss.Style
	WindowRow   lipgloss.Style
	WindowPath  lipgloss.Style
	ActiveRow   lipgloss.Style
	SelectedRow lipgloss.Style
	Rail        lipgloss.Style
}

type TreeRowProps struct {
	NodeID     string
	Kind       contracts.NodeKind
	Label      string
	Path       string
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

// composeRowLine truncates the row's left content and, for window rows,
// right-aligns a subtle, middle-truncated path in the remaining space.
func composeRowLine(row TreeRowProps, left string, width int, compact bool, styles TreeStyles) string {
	path := strings.TrimSpace(row.Path)
	if compact || path == "" || row.Kind != contracts.NodeKindWindow {
		return truncateWidth(left, width)
	}

	const gap = 2
	const minPathWidth = 6
	leftW := lipgloss.Width(left)
	avail := width - leftW - gap
	if avail < minPathWidth {
		return truncateWidth(left, width)
	}

	shortened := shortenPath(path, avail)
	padding := width - leftW - lipgloss.Width(shortened)
	if padding < gap {
		padding = gap
	}
	return left + strings.Repeat(" ", padding) + styles.WindowPath.Render(shortened)
}

// shortenPath fits a path into maxW columns, preferring to keep the final
// path segment intact and eliding the middle (e.g. ~/if_it_is_lo.../the-end).
func shortenPath(path string, maxW int) string {
	if maxW <= 0 {
		return ""
	}
	if lipgloss.Width(path) <= maxW {
		return path
	}

	const ellipsis = "..."
	runes := []rune(path)
	if maxW <= len(ellipsis) {
		if maxW > len(runes) {
			maxW = len(runes)
		}
		return string(runes[:maxW])
	}

	if idx := strings.LastIndex(path, "/"); idx > 0 {
		tail := path[idx:]
		if lipgloss.Width(tail)+len(ellipsis)+1 <= maxW {
			headBudget := maxW - len(ellipsis) - lipgloss.Width(tail)
			return truncateRunes(path, headBudget) + ellipsis + tail
		}
	}

	avail := maxW - len(ellipsis)
	headW := avail - avail/2
	tailW := avail / 2
	return string(runes[:headW]) + ellipsis + string(runes[len(runes)-tailW:])
}

func truncateRunes(s string, w int) string {
	if w <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= w {
		return s
	}
	return string(runes[:w])
}

func renderRowLine(row TreeRowProps, styles TreeStyles, compact bool) string {
	cursor := " "
	if row.Selected {
		cursor = "❯"
	}

	marker := "  "
	if row.Active {
		marker = "● "
	}

	label := decoratedLabel(row, styles, compact)
	return fmt.Sprintf("%s %s%s", cursor, marker, label)
}

func styleRowLine(row TreeRowProps, line string, width int, styles TreeStyles) string {
	if row.Selected {
		return styles.SelectedRow.Width(width).Render(line)
	}
	if row.Kind == contracts.NodeKindSession {
		return styles.SessionRow.Render(line)
	}
	if row.Active {
		return styles.ActiveRow.Render(line)
	}
	if row.Kind == contracts.NodeKindWindow {
		return styles.WindowRow.Render(line)
	}
	return styles.Row.Render(line)
}

func decoratedLabel(row TreeRowProps, styles TreeStyles, compact bool) string {
	label := strings.TrimSpace(row.Label)
	if label == "" {
		label = row.NodeID
	}
	if compact {
		return compactLabel(row, label)
	}
	prefix := row.TreePrefix
	branch := branchGlyph(row)
	if !row.Selected {
		prefix = styles.Rail.Render(prefix)
		branch = styles.Rail.Render(branch)
	}
	if row.Depth == 0 {
		return fmt.Sprintf("%s %s %s", branch, nodeIcon(row.Kind), label)
	}
	return fmt.Sprintf("  %s%s %s", prefix, nodeIcon(row.Kind), label)
}

func compactLabel(row TreeRowProps, label string) string {
	indent := strings.Repeat("  ", max(row.Depth-1, 0))
	if row.Kind == contracts.NodeKindSession {
		return fmt.Sprintf("%s %s", nodeIcon(row.Kind), label)
	}
	return fmt.Sprintf("%s%s %s", indent, nodeIcon(row.Kind), label)
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

func nodeIcon(kind contracts.NodeKind) string {
	if kind == contracts.NodeKindSession {
		return "\U000f018d"
	}
	return "\ueb14"
}
