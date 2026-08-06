package tmux

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadStateQuery(t *testing.T) {
	q := LoadStateQuery{}

	t.Run("args", func(t *testing.T) {
		expected := []string{
			"start-server",
			";", "show-options", "-gv", "base-index",
			";", "show-options", "-gv", "pane-base-index",
			";", "list-panes", "-a", "-F",
			"#{session_id}|#{session_name}|#{window_id}|#{window_name}|#{window_index}|#{window_layout}|#{window_zoomed_flag}|#{window_active}|#{pane_id}|#{pane_index}|#{pane_active}|#{pane_current_path}|#{pane_current_command}|#{@hetki_workspace_path}",
		}
		assert.Equal(t, expected, q.Args())
	})

	t.Setenv("TMUX", "")

	tests := []struct {
		name   string
		output string
		want   LoadStateResult
	}{
		{"empty", "", LoadStateResult{}},
		{
			name:   "single session single window single pane",
			output: "0\n0\n$1|dev|@1|editor|0|layout-a|0|1|%1|0|1|~/code|vim",
			want: LoadStateResult{
				Sessions: []Session{{
					ID:   "$1",
					Name: "dev",
					Windows: []Window{{
						ID:     "@1",
						Name:   "editor",
						Index:  0,
						Path:   "~/code",
						Layout: "layout-a",
						Panes:  []Pane{{ID: "%1", Index: 0, Path: "~/code", Command: "vim"}},
					}},
				}},
			},
		},
		{
			name:   "multiple panes same window",
			output: "0\n1\n$1|dev|@1|editor|0|layout-a|1|1|%1|0|0|~/code|vim\n$1|dev|@1|editor|0|layout-a|1|1|%2|1|1|~/api|node",
			want: LoadStateResult{
				Sessions: []Session{{
					ID:   "$1",
					Name: "dev",
					Windows: []Window{{
						ID:     "@1",
						Name:   "editor",
						Index:  0,
						Path:   "~/code",
						Layout: "layout-a",
						Panes:  []Pane{{ID: "%1", Index: 0, Path: "~/code", Command: "vim"}, {ID: "%2", Index: 1, Path: "~/api", Command: "node", Zoom: true}},
					}},
				}},
				PaneBaseIndex: 1,
			},
		},
		{
			name:   "multiple windows",
			output: "1\n1\n$1|dev|@1|editor|0|layout-a|0|0|%1|0|0|~/code|vim\n$1|dev|@2|server|1|layout-b|0|1|%2|0|1|~/api|node",
			want: LoadStateResult{
				Sessions: []Session{{
					ID:   "$1",
					Name: "dev",
					Windows: []Window{
						{ID: "@1", Name: "editor", Index: 0, Path: "~/code", Layout: "layout-a", Panes: []Pane{{ID: "%1", Index: 0, Path: "~/code", Command: "vim"}}},
						{ID: "@2", Name: "server", Index: 1, Path: "~/api", Layout: "layout-b", Panes: []Pane{{ID: "%2", Index: 0, Path: "~/api", Command: "node"}}},
					},
				}},
				WindowBaseIndex: 1,
				PaneBaseIndex:   1,
			},
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

func TestLoadStateQueryParsesWorkspacePathOption(t *testing.T) {
	q := LoadStateQuery{}
	t.Setenv("TMUX", "")

	output := "0\n0\n$1|dev|@1|editor|0|layout-a|0|1|%1|0|1|~/code|vim|/tmp/workspace.yaml"

	got, err := q.Parse(output)
	assert.NoError(t, err)
	if assert.Len(t, got.Sessions, 1) {
		assert.Equal(t, "/tmp/workspace.yaml", got.Sessions[0].WorkspacePath)
	}
}

func TestLoadStateQueryOrdersSessionsAndWindows(t *testing.T) {
	q := LoadStateQuery{}
	t.Setenv("TMUX", "")

	output := "0\n0\n$2|zeta|@4|server|1|layout-z1|0|1|%4|0|1|~/zeta/server|node\n$2|zeta|@3|editor|0|layout-z0|0|0|%3|0|0|~/zeta/editor|vim\n$1|alpha|@2|worker|1|layout-a1|0|1|%2|0|1|~/alpha/worker|make\n$1|alpha|@1|editor|0|layout-a0|0|0|%1|0|0|~/alpha/editor|vim"

	want := []Session{
		{
			ID:   "$1",
			Name: "alpha",
			Windows: []Window{
				{ID: "@1", Name: "editor", Index: 0, Path: "~/alpha/editor", Layout: "layout-a0", Panes: []Pane{{ID: "%1", Index: 0, Path: "~/alpha/editor", Command: "vim"}}},
				{ID: "@2", Name: "worker", Index: 1, Path: "~/alpha/worker", Layout: "layout-a1", Panes: []Pane{{ID: "%2", Index: 0, Path: "~/alpha/worker", Command: "make"}}},
			},
		},
		{
			ID:   "$2",
			Name: "zeta",
			Windows: []Window{
				{ID: "@3", Name: "editor", Index: 0, Path: "~/zeta/editor", Layout: "layout-z0", Panes: []Pane{{ID: "%3", Index: 0, Path: "~/zeta/editor", Command: "vim"}}},
				{ID: "@4", Name: "server", Index: 1, Path: "~/zeta/server", Layout: "layout-z1", Panes: []Pane{{ID: "%4", Index: 0, Path: "~/zeta/server", Command: "node"}}},
			},
		},
	}

	for range 100 {
		got, err := q.Parse(output)
		assert.NoError(t, err)
		assert.Equal(t, want, got.Sessions)
	}
}
