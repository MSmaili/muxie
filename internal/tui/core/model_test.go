package core

import (
	"strings"
	"testing"
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
