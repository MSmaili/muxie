package core

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/MSmaili/hetki/internal/tui/contracts"
	"github.com/MSmaili/hetki/internal/tui/core/components"
)

func (m model) View() tea.View {
	t := m.theme
	lineWidth := m.width
	if lineWidth <= 0 {
		lineWidth = 100
	}

	frameStyle := responsiveFrameStyle(t.appBorder, lineWidth)
	borderFrameSize := frameStyle.GetHorizontalFrameSize()
	innerW := lineWidth - borderFrameSize
	if innerW < 1 {
		innerW = 1
	}
	compact := innerW < 56

	start := m.offset
	end := m.offset + m.listH
	if end > len(m.rows) {
		end = len(m.rows)
	}

	contentLines := []string{
		components.RenderSearchBar(components.SearchBarProps{
			Width:       innerW,
			Filter:      m.filter,
			Right:       headerRight(m),
			Active:      m.mode == modeFilter,
			Compact:     compact,
			Style:       t.searchBox,
			PromptStyle: t.selectedHint,
			MetaStyle:   t.meta,
		}),
		t.sectionLine.Render(strings.Repeat("─", innerW)),
	}

	visibleRows := make([]components.TreeRowProps, 0, end-start)
	for i := start; i < end && len(visibleRows) < m.listH; i++ {
		r := m.rows[i]
		visibleRows = append(visibleRows, components.TreeRowProps{
			NodeID:     r.Node.ID,
			Kind:       r.Node.Kind,
			Label:      r.Node.Label,
			Path:       r.Node.Path,
			Depth:      r.Depth,
			TreePrefix: r.TreePrefix,
			Expanded:   r.Expanded,
			Branch:     r.Branch,
			Active:     r.Node.Active,
			Selected:   i == m.cursor,
		})
	}
	rowLines := components.RenderTree(components.TreeProps{
		Width:     innerW,
		EmptyText: emptyStateText(m),
		Rows:      visibleRows,
		Compact:   compact,
		Styles: components.TreeStyles{
			Meta:               t.meta,
			Row:                t.row,
			SessionRow:         t.sessionRow,
			WindowRow:          t.windowRow,
			WindowPath:         t.windowPath,
			WindowPathSelected: t.windowPathSelected,
			ActiveRow:          t.activeRow,
			SelectedRow:        t.selectedRow,
			Rail:               t.rail,
		},
	})

	header := strings.Join(contentLines, "\n")
	middle := strings.Join(rowLines, "\n")
	borderFrameV := frameStyle.GetVerticalFrameSize()
	middleH := m.height - borderFrameV - lipgloss.Height(header)
	if middleH < 1 {
		middleH = 1
	}
	middle = lipgloss.PlaceVertical(middleH, lipgloss.Top, middle)

	content := lipgloss.JoinVertical(lipgloss.Left, header, middle)
	rendered := frameStyle.Width(lineWidth).Render(content)

	var overlayContent string
	if m.mode == modeInput {
		overlayContent = components.RenderInputModal(components.InputModalProps{
			LineWidth:  lineWidth,
			Title:      m.input.Title,
			Prompt:     m.input.Prompt,
			Value:      m.input.Value,
			ModalStyle: t.modal,
			TitleStyle: t.modalTitle,
			HintStyle:  t.modalHint,
		})
	} else if m.mode == modeConfirm {
		overlayContent = components.RenderConfirmModal(components.ConfirmModalProps{
			LineWidth:  lineWidth,
			Title:      m.confirm.Title,
			Body:       m.confirm.Body,
			ModalStyle: t.modal,
			TitleStyle: t.modalTitle,
			HintStyle:  t.modalHint,
		})
	} else if m.helpOpen {
		overlayContent = components.RenderHelpOverlay(components.HelpOverlayProps{
			LineWidth: lineWidth,
			Title:     "KEYBINDINGS",
			Hint:      "? / esc / enter close",
			Sections: []components.HelpSection{
				{Title: "NAVIGATION", Entries: []components.HelpEntry{{Keys: "j, k, ↓, ↑", Desc: "move"}, {Keys: "ctrl+n, ctrl+p", Desc: "move (vim)"}, {Keys: "u, d, pgup, pgdn", Desc: "page up/down"}, {Keys: "g, G", Desc: "top / bottom"}, {Keys: "h, l, ←, →", Desc: "collapse / expand"}, {Keys: "H, L", Desc: "collapse all / expand all"}}},
				{Title: "ACTIONS", Entries: []components.HelpEntry{{Keys: "enter, ctrl+y", Desc: "select / switch"}, {Keys: "a", Desc: "new window"}, {Keys: "s", Desc: "new session"}, {Keys: "e", Desc: "rename"}, {Keys: "x", Desc: "delete"}, {Keys: "r", Desc: "refresh"}}},
				{Title: "FILTER", Entries: []components.HelpEntry{{Keys: "/", Desc: "start filter"}, {Keys: "n, N", Desc: "next / prev match"}, {Keys: "ctrl+l", Desc: "clear filter"}}},
				{Title: "EDIT", Entries: []components.HelpEntry{{Keys: "ctrl+w", Desc: "delete word"}, {Keys: "ctrl+u", Desc: "clear line"}}},
				{Title: "OTHER", Entries: []components.HelpEntry{{Keys: "esc", Desc: "cancel"}, {Keys: "q, ctrl+c", Desc: "quit"}, {Keys: "?", Desc: "toggle help"}}},
			},
			OverlayStyle: t.helpOverlay,
			TitleStyle:   t.modalTitle,
			MetaStyle:    t.meta,
			KeyStyle:     t.selectedHint,
			HintStyle:    t.modalHint,
		})
	}

	if overlayContent != "" {
		totalH := lipgloss.Height(rendered)
		overlayH := lipgloss.Height(overlayContent)
		overlayW := lipgloss.Width(overlayContent)
		x := (lineWidth - overlayW) / 2
		y := (totalH - overlayH) / 2
		if x < 0 {
			x = 0
		}
		if y < 0 {
			y = 0
		}
		bg := lipgloss.NewLayer(rendered)
		fg := lipgloss.NewLayer(overlayContent).X(x).Y(y).Z(1)
		rendered = lipgloss.NewCompositor(bg, fg).Render()
	}

	v := tea.NewView(rendered)
	v.AltScreen = true
	return v
}

func headerRight(m model) string {
	if m.err != nil {
		return m.err.Error()
	}
	if m.busy {
		return m.status
	}
	return sessionPositionLabel(m)
}

func sessionPositionLabel(m model) string {
	total := 0
	current := 0
	selectedSessionID := ""
	if selected, ok := m.selectedRow(); ok {
		selectedSessionID = selected.Node.ID
		if selected.Node.ParentID != "" {
			selectedSessionID = selected.Node.ParentID
		}
	}
	for _, row := range m.rows {
		if row.Node.Kind != contracts.NodeKindSession {
			continue
		}
		total++
		if row.Node.ID == selectedSessionID {
			current = total
		}
	}
	position := fmt.Sprintf("%d sessions", total)
	if current > 0 {
		position = fmt.Sprintf("%d/%d", current, total)
	}
	workspace := workspaceLabel(m.snapshot.ContextBars["workspace"])
	if workspace == "" {
		return position
	}
	return workspace + "  " + position
}

func responsiveFrameStyle(base lipgloss.Style, width int) lipgloss.Style {
	if width < 32 {
		return lipgloss.NewStyle()
	}
	if width < 52 {
		return lipgloss.NewStyle().Padding(0, 1)
	}
	if width < 72 {
		return base.Padding(0, 1)
	}
	return base
}

func emptyStateText(m model) string {
	if strings.TrimSpace(m.filter) != "" {
		return "no matching sessions or windows | esc cancel filter | ctrl+l clear"
	}
	return "no sessions found"
}
