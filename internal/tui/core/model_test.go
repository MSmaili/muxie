package core

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/MSmaili/hetki/internal/terminal"
	"github.com/MSmaili/hetki/internal/tui/list"
	"github.com/stretchr/testify/require"
)

func viewSnapshot() list.Snapshot {
	return list.Snapshot{Items: []list.Item{{
		ID: "session", Primary: "session", SearchFields: []list.SearchField{{Tier: list.SearchPrimary, Text: "session"}},
		Children: []list.Item{{
			ID: "window", Primary: "editor", Secondary: "~/code/editor",
			SearchFields: []list.SearchField{{Tier: list.SearchPrimary, Text: "editor"}, {Tier: list.SearchSecondary, Text: "~/code/editor"}},
		}},
	}}}
}

func TestRunRejectsInvalidInitialProjectionBeforeStartingTea(t *testing.T) {
	invalid := list.Snapshot{Items: []list.Item{
		{ID: "same", Primary: "one"},
		{ID: "same", Primary: "two"},
	}}
	_, err := Run(context.Background(), invalid, nil)
	if err == nil || !strings.Contains(err.Error(), "duplicate item ID") {
		t.Fatalf("Run error = %v, want duplicate item ID", err)
	}
}

func TestModelStartsInJumpUnlessTheProjectionIsEmpty(t *testing.T) {
	m := newModel(viewSnapshot(), nil)
	if m.mode != modeBrowse || !m.initialJump || len(m.jump.candidates) != 0 {
		t.Fatalf("non-empty startup was not waiting for its first layout: mode=%q pending=%t candidates=%d", m.mode, m.initialJump, len(m.jump.candidates))
	}
	m, _ = updateModel(t, m, tea.WindowSizeMsg{Width: 40, Height: 8})
	if m.mode != modeJump || len(m.jump.candidates) == 0 {
		t.Fatalf("first layout canceled startup jump: mode=%q candidates=%d", m.mode, len(m.jump.candidates))
	}
	m, _ = updateModel(t, m, tea.WindowSizeMsg{Width: 41, Height: 8})
	if m.mode != modeBrowse {
		t.Fatalf("later resize kept stale jump labels: mode=%q", m.mode)
	}

	empty := newModel(list.Snapshot{}, nil)
	if empty.mode != modeBrowse || len(empty.jump.candidates) != 0 {
		t.Fatalf("empty startup mode = %q with %d candidates, want normal", empty.mode, len(empty.jump.candidates))
	}
}

func TestViewKeepsSecondaryTextOnItsRowWithoutGlobalHelp(t *testing.T) {
	m := newModel(viewSnapshot(), nil)
	m.width, m.height = 80, 20
	m = m.reflow()
	content := m.View().Content
	lines := strings.Split(content, "\n")
	if len(lines) < 3 || !strings.Contains(terminal.Sanitize(lines[1]), "press / to filter") || strings.Contains(lines[1], "──") || !strings.Contains(lines[2], "──") {
		t.Fatalf("header hint should be one row with only a bottom divider: %q", lines[:min(3, len(lines))])
	}
	if strings.Contains(content, "KEYBINDINGS") || strings.Contains(content, "WORKSPACE:") || strings.Contains(content, "? help") {
		t.Fatalf("removed global help or workspace data is still rendered: %q", content)
	}
	for _, line := range lines {
		if strings.Contains(line, "editor") {
			if !strings.Contains(line, "~/code/editor") {
				t.Fatalf("secondary text wrapped off its row: %q", line)
			}
			return
		}
	}
	t.Fatal("editor row not rendered")
}

func TestCompactLayoutKeepsItsEdges(t *testing.T) {
	for _, width := range []int{4, 12, 24, 40} {
		style := responsiveFrameStyle(defaultTheme().appBorder, width, 8)
		require.True(t, style.GetBorderLeft(), "width %d lost its left edge", width)
		require.True(t, style.GetBorderRight(), "width %d lost its right edge", width)
	}

	short := responsiveFrameStyle(defaultTheme().appBorder, 24, 4)
	require.True(t, short.GetBorderLeft())
	require.True(t, short.GetBorderRight())
	require.False(t, short.GetBorderTop())
	require.False(t, short.GetBorderBottom())
}

func TestContextMenuOverlaySanitizesAndFitsNarrowTerminals(t *testing.T) {
	unsafe := "Open\x1b]0;owned\a\nnext"
	for _, width := range []int{1, 2, 3, 12, 24} {
		m := newModel(viewSnapshot(), nil)
		m.mode = modeMenu
		m.initialJump = false
		m.menu = menuState{Title: "WINDOW ACTIONS", Entries: []MenuEntry{{Action: ActionOpen, Label: unsafe, Activation: 'o'}}}
		m.width, m.height = width, 8
		m = m.reflow()
		content := m.View().Content
		if strings.Contains(content, "owned") || strings.Contains(content, "\nnext") {
			t.Fatalf("width %d rendered unsafe menu text: %q", width, content)
		}
		for _, line := range strings.Split(content, "\n") {
			if actual := terminal.Width(line); actual > width {
				t.Fatalf("width %d rendered menu line width %d: %q", width, actual, line)
			}
		}
	}
}

func TestContextMenuViewportFitsAndCentersTheFullMenu(t *testing.T) {
	entries := []MenuEntry{
		{Action: ActionOpen, Label: "Open destination", Activation: 'o'},
		{Action: ActionCreateSession, Label: "New session", Activation: 's'},
		{Action: ActionCreateWindow, Label: "New window", Activation: 'w'},
		{Action: ActionRename, Label: "Rename window", Activation: 'r'},
		{Action: ActionDelete, Label: "Delete window", Activation: 'd'},
		{Action: ActionRenameSession, Label: "Rename session", Activation: 'n'},
		{Action: ActionDeleteSession, Label: "Delete session", Activation: 'x'},
		{Action: ActionRefresh, Label: "Refresh", Activation: 'f'},
		{Action: ActionToggleProjection, Label: "Toggle projection", Activation: 't'},
	}

	short := newModel(viewSnapshot(), nil)
	short.mode = modeMenu
	short.initialJump = false
	short.menu = menuState{Title: "DESTINATION ACTIONS", Entries: entries, Cursor: len(entries) - 1}
	short.width = 24
	var content string
	for height := 1; height <= 8; height++ {
		short.height = height
		short = short.reflow()
		content = short.View().Content
		if got, limit := lipgloss.Height(content), max(3, height); got > limit {
			t.Fatalf("short menu height = %d, want at most %d: %q", got, limit, content)
		}
	}
	plain := terminal.Sanitize(content)
	if !strings.Contains(plain, "Toggle projec") || strings.Contains(plain, "Open destination") {
		t.Fatalf("menu viewport did not follow the selection: %q", plain)
	}

	centered := short
	centered.width, centered.height = 80, 20
	centered.menu.Cursor = 0
	centered = centered.reflow()
	content = centered.View().Content
	overlay := RenderMenu(MenuProps{
		LineWidth: 80, MaxHeight: lipgloss.Height(content), Title: centered.menu.Title,
		Entries: entries, ModalStyle: centered.theme.modal, TitleStyle: centered.theme.modalTitle,
		EntryStyle: centered.theme.row, KeyStyle: centered.theme.jumpLabel,
		SelectedStyle: centered.theme.selectedRow, HintStyle: centered.theme.modalHint,
	})
	wantX := (80 - lipgloss.Width(overlay)) / 2
	wantY := (lipgloss.Height(content) - lipgloss.Height(overlay)) / 2
	lines := strings.Split(content, "\n")
	require.True(t, wantY < len(lines))
	line := terminal.Sanitize(lines[wantY])
	border := strings.Index(line, "╭")
	require.NotEqual(t, -1, border, "centered overlay border missing from %q", line)
	require.Equal(t, wantX, terminal.Width(line[:border]))
}

func TestViewSanitizesExternalTextAndFitsWidth(t *testing.T) {
	unsafe := "\x1b]0;owned\a中👨‍👩‍👧é\nnext"
	snapshot := list.Snapshot{Items: []list.Item{{
		ID: "unsafe", Primary: unsafe, SearchFields: []list.SearchField{{Tier: list.SearchPrimary, Text: unsafe}},
	}}, ActiveItemID: "unsafe"}
	for _, width := range []int{1, 2, 3, 20} {
		m := newModel(snapshot, nil)
		m.width, m.height = width, 8
		m = m.reflow()
		content := m.View().Content
		if strings.Contains(content, "owned") || strings.Contains(content, "\nnext") {
			t.Fatalf("width %d rendered unsafe external text: %q", width, content)
		}
		for _, line := range strings.Split(content, "\n") {
			if actual := terminal.Width(line); actual > width {
				t.Fatalf("width %d rendered line width %d: %q", width, actual, line)
			}
		}
	}
}
