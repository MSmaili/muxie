package core

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/MSmaili/hetki/internal/tui/list"
)

func (m model) View() tea.View {
	t := m.theme
	layout := m.layout()
	lineWidth, innerW := layout.lineWidth, layout.innerWidth

	contentLines := []string{
		RenderSearchBar(SearchBarProps{
			Width: innerW, Filter: m.items.Query(), Hint: headerHint(m), Right: headerRight(m),
			Prompt: headerPrompt(m), Active: m.mode == modeFilter,
			Style: t.searchBox, PromptStyle: t.searchPrompt, HintStyle: t.headerHint, MetaStyle: t.meta,
		}),
		t.sectionLine.Render(strings.Repeat("─", innerW)),
	}

	rows := m.items.VisibleRows()
	visibleRows := make([]TreeRowProps, 0, len(rows))
	for i, row := range rows {
		visibleRows = append(visibleRows, TreeRowProps{
			ItemID: string(row.Item.ID), Primary: row.Item.Primary, Secondary: row.Item.Secondary,
			JumpLabel: m.jumpLabel(row.Item.ID),
			Depth:     row.Depth, TreePrefix: row.TreePrefix, Expanded: row.Expanded, Branch: row.Branch,
			Active: m.items.IsActive(row.Item.ID), Selected: m.items.Offset()+i == m.items.Cursor(),
		})
	}
	rowLines := RenderTree(TreeProps{
		Width: innerW, EmptyText: emptyStateText(m), Rows: visibleRows, Compact: layout.compact,
		Styles: TreeStyles{
			Meta: t.meta, Row: t.row, RootRow: t.rootRow, ChildRow: t.childRow,
			Secondary: t.secondary, SecondarySelected: t.secondarySelected, ActiveRow: t.activeRow,
			SelectedRow: t.selectedRow, JumpLabel: t.jumpLabel, Rail: t.rail,
		},
	})

	header := strings.Join(contentLines, "\n")
	middle := lipgloss.PlaceVertical(layout.middleHeight, lipgloss.Top, strings.Join(rowLines, "\n"))
	rendered := layout.frameStyle.Width(lineWidth).Render(lipgloss.JoinVertical(lipgloss.Left, header, middle))

	var overlay string
	switch m.mode {
	case modeMenu:
		overlay = RenderMenu(MenuProps{
			LineWidth: lineWidth, MaxHeight: lipgloss.Height(rendered),
			Title: m.menu.Title, Entries: m.menu.Entries, Selected: m.menu.Cursor,
			Hint: menuControls(m.keys), ModalStyle: t.modal, TitleStyle: t.modalTitle,
			EntryStyle: t.row, KeyStyle: t.jumpLabel, SelectedStyle: t.selectedRow, HintStyle: t.modalHint,
		})
	case modeInput:
		overlay = RenderInputModal(InputModalProps{
			LineWidth: lineWidth, Title: m.input.Title, Prompt: m.input.Prompt, Value: m.input.Value,
			Hint:       modalControls(m.keys, KeyModeInput, "submit"),
			ModalStyle: t.modal, TitleStyle: t.modalTitle, HintStyle: t.modalHint,
		})
	case modeConfirm:
		overlay = RenderConfirmModal(ConfirmModalProps{
			LineWidth: lineWidth, Title: m.confirm.Title, Body: m.confirm.Body,
			Hint:       modalControls(m.keys, KeyModeConfirm, "confirm"),
			ModalStyle: t.modal, TitleStyle: t.modalTitle, HintStyle: t.modalHint,
		})
	}
	if overlay != "" {
		x := max(0, (lineWidth-lipgloss.Width(overlay))/2)
		y := max(0, (lipgloss.Height(rendered)-lipgloss.Height(overlay))/2)
		rendered = lipgloss.NewCompositor(
			lipgloss.NewLayer(rendered),
			lipgloss.NewLayer(overlay).X(x).Y(y).Z(1),
		).Render()
	}

	view := tea.NewView(rendered)
	view.AltScreen = true
	return view
}

func (m model) jumpLabel(id list.ItemID) string {
	if m.mode != modeJump {
		return ""
	}
	return m.jump.labelFor(id)
}

func modalControls(keys KeyMap, mode KeyMode, confirmLabel string) string {
	parts := make([]string, 0, 2)
	for _, control := range []struct {
		action ActionID
		label  string
	}{
		{action: ActionConfirm, label: confirmLabel},
		{action: ActionCancel, label: "cancel"},
	} {
		if bound := keys.Keys(mode, control.action); len(bound) > 0 {
			parts = append(parts, strings.Join(bound, "/")+" "+control.label)
		}
	}
	return strings.Join(parts, " | ")
}

func menuControls(keys KeyMap) string {
	up := keys.Keys(KeyModeMenu, ActionMoveUp)
	down := keys.Keys(KeyModeMenu, ActionMoveDown)
	confirm := keys.Keys(KeyModeMenu, ActionConfirm)
	cancel := keys.Keys(KeyModeMenu, ActionCancel)
	if len(up) == 0 || len(down) == 0 || len(confirm) == 0 || len(cancel) == 0 {
		return ""
	}
	return up[0] + "/" + down[0] + " move | " + confirm[0] + " select | " + cancel[0] + " cancel"
}

func headerPrompt(m model) string {
	switch m.mode {
	case modeBrowse:
		return " NORMAL "
	case modeFilter:
		return " FILTER "
	case modeJump:
		return " JUMP "
	default:
		return ""
	}
}

func headerHint(m model) string {
	switch m.mode {
	case modeBrowse:
		return "press / to filter or ; to jump"
	case modeJump:
		return "type a label to jump or / to filter"
	default:
		return ""
	}
}

func headerRight(m model) string {
	if m.err != nil {
		return m.err.Error()
	}
	if m.busy {
		return m.status
	}
	if notice := m.items.Snapshot().Notice; notice != "" {
		return notice
	}
	return rootCountLabel(m.items.Snapshot(), m.items.ShownRoots(), m.items.Query())
}

func rootCountLabel(snapshot list.Snapshot, shown int, query string) string {
	total := len(snapshot.Items)
	if total == 0 {
		return ""
	}
	if strings.TrimSpace(query) == "" {
		return fmt.Sprintf("%d", total)
	}
	return fmt.Sprintf("%d/%d", shown, total)
}

func responsiveFrameStyle(base lipgloss.Style, width, height int) lipgloss.Style {
	if width < 4 {
		return lipgloss.NewStyle()
	}
	if height < 5 {
		base = base.BorderTop(false).BorderBottom(false)
	}
	if width < 32 {
		return base.Padding(0, 0)
	}
	if width < 72 {
		return base.Padding(0, 1)
	}
	return base
}

func emptyStateText(m model) string {
	if strings.TrimSpace(m.items.Query()) != "" {
		return "no matching items"
	}
	return "no sessions found"
}
