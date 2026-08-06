package core

import (
	"strings"
	"testing"

	"github.com/MSmaili/hetki/internal/terminal"
	"github.com/MSmaili/hetki/internal/tui/contracts"
)

func TestViewKeepsWindowPathOnItsRowWithoutFooter(t *testing.T) {
	m := newModel(filterTestSnapshot(), nil)
	m.width, m.height = 80, 20
	m = m.reflow()
	content := m.View().Content
	lines := strings.Split(content, "\n")
	if len(lines) < 3 || !strings.Contains(lines[1], "\uf002") || strings.Contains(lines[1], "──") || !strings.Contains(lines[2], "──") {
		t.Fatalf("search input should be one row with only a bottom divider: %q", lines[:min(3, len(lines))])
	}

	for _, line := range lines {
		if strings.Contains(line, "editor") {
			if !strings.Contains(line, "~/code/editor") {
				t.Fatalf("window path wrapped off its row: %q", line)
			}
			if strings.Contains(content, "? help") {
				t.Fatal("persistent footer is still rendered")
			}
			return
		}
	}
	t.Fatal("editor row not rendered")
}

func TestViewSanitizesExternalTextAndFitsWidth(t *testing.T) {
	snapshot := contracts.Snapshot{Nodes: []contracts.Node{{
		ID: "session:unsafe", Kind: contracts.NodeKindSession, Label: "\x1b]0;owned\a中👨‍👩‍👧é\nnext", Active: true,
	}}}
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

func TestWorkspaceContextUsesFriendlyWorkspaceLabel(t *testing.T) {
	tests := []struct {
		name      string
		workspace string
		want      string
	}{
		{name: "named workspace path", workspace: "/Users/me/.config/hetki/workspaces/personal.yaml", want: "WORKSPACE: personal"},
		{name: "local hetki file uses parent dir", workspace: "/Users/me/projects/muxie/.hetki.yaml", want: "WORKSPACE: muxie"},
		{name: "plain label unchanged", workspace: "unmanaged", want: "WORKSPACE: unmanaged"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := workspaceContext(map[string]string{"workspace": tt.workspace})
			if got != tt.want {
				t.Fatalf("workspaceContext() = %q, want %q", got, tt.want)
			}
		})
	}
}
