package tmux

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadStateQuery(t *testing.T) {
	q := LoadStateQuery{}

	t.Run("args", func(t *testing.T) {
		assert.Equal(t, []string{"list-panes", "-a", "-F", "#{session_name}|#{window_name}|#{pane_current_path}|#{pane_current_command}|#{TMS_WORKSPACE_PATH}"}, q.Args())
	})

	tests := []struct {
		name   string
		output string
		want   []Session
	}{
		{"empty", "", []Session{}},
		{
			name:   "single session single window single pane",
			output: "dev|editor|~/code|vim|/path/to/workspace.yaml",
			want: []Session{{
				Name:          "dev",
				WorkspacePath: "/path/to/workspace.yaml",
				Windows: []Window{{
					Name:  "editor",
					Path:  "~/code",
					Panes: []Pane{{Path: "~/code", Command: "vim"}},
				}},
			}},
		},
		{
			name:   "multiple panes same window",
			output: "dev|editor|~/code|vim|\ndev|editor|~/api|node|",
			want: []Session{{
				Name:          "dev",
				WorkspacePath: "",
				Windows: []Window{{
					Name:  "editor",
					Path:  "~/code",
					Panes: []Pane{{Path: "~/code", Command: "vim"}, {Path: "~/api", Command: "node"}},
				}},
			}},
		},
		{
			name:   "multiple windows",
			output: "dev|editor|~/code|vim|/ws.yaml\ndev|server|~/api|node|/ws.yaml",
			want: []Session{{
				Name:          "dev",
				WorkspacePath: "/ws.yaml",
				Windows: []Window{
					{Name: "editor", Path: "~/code", Panes: []Pane{{Path: "~/code", Command: "vim"}}},
					{Name: "server", Path: "~/api", Panes: []Pane{{Path: "~/api", Command: "node"}}},
				},
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := q.Parse(tt.output)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestPaneBaseIndexQuery(t *testing.T) {
	q := PaneBaseIndexQuery{}

	t.Run("args", func(t *testing.T) {
		assert.Equal(t, []string{"show-options", "-gv", "pane-base-index"}, q.Args())
	})

	tests := []struct {
		name   string
		output string
		want   int
	}{
		{"empty", "", 0},
		{"zero", "0", 0},
		{"one", "1", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := q.Parse(tt.output)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
