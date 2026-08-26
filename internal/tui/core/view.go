package core

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/MSmaili/hetki/internal/terminal"
	"github.com/MSmaili/hetki/internal/tui/list"
)

func (m model) View() tea.View {
	t := m.theme
	layout := m.layout()
	lineWidth, innerW := layout.lineWidth, layout.innerWidth

	contentLines := []string{
		RenderSearchBar(SearchBarProps{
			Width: innerW, Filter: m.items.Query(), Right: headerRight(m),
			Active: m.mode == modeFilter, Compact: layout.compact,
			Style: t.searchBox, PromptStyle: t.searchPrompt, PlaceholderStyle: t.searchPlaceholder, MetaStyle: t.meta,
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
	if m.mode == modeInput {
		overlay = RenderInputModal(InputModalProps{
			LineWidth: lineWidth, Title: m.input.Title, Prompt: m.input.Prompt, Value: m.input.Value,
			Hint:       modalControls(m.keys, KeyModeInput, "submit"),
			ModalStyle: t.modal, TitleStyle: t.modalTitle, HintStyle: t.modalHint,
		})
	} else if m.mode == modeConfirm {
		overlay = RenderConfirmModal(ConfirmModalProps{
			LineWidth: lineWidth, Title: m.confirm.Title, Body: m.confirm.Body,
			Hint:       modalControls(m.keys, KeyModeConfirm, "confirm"),
			ModalStyle: t.modal, TitleStyle: t.modalTitle, HintStyle: t.modalHint,
		})
	}
	if overlay != "" {
		x := max(0, (lineWidth-terminal.Width(overlay))/2)
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
	return rootCountLabel(m.items.Snapshot(), m.items.ShownRoots())
}

func rootCountLabel(snapshot list.Snapshot, shown int) string {
	total := len(snapshot.Items)
	if total == 0 {
		return ""
	}
	return fmt.Sprintf("%d/%d", shown, total)
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
	if strings.TrimSpace(m.items.Query()) != "" {
		return "no matching items"
	}
	return "no sessions found"
}
