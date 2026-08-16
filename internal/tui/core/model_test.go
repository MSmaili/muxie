package core

import (
	"context"
	"strings"
	"testing"

	"github.com/MSmaili/hetki/internal/terminal"
	"github.com/MSmaili/hetki/internal/tui/list"
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

func TestViewKeepsSecondaryTextOnItsRowWithoutGlobalHelp(t *testing.T) {
	m := newModel(viewSnapshot(), nil)
	m.width, m.height = 80, 20
	m = m.reflow()
	content := m.View().Content
	lines := strings.Split(content, "\n")
	if len(lines) < 3 || !strings.Contains(lines[1], "\uf002") || strings.Contains(lines[1], "──") || !strings.Contains(lines[2], "──") {
		t.Fatalf("search input should be one row with only a bottom divider: %q", lines[:min(3, len(lines))])
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
