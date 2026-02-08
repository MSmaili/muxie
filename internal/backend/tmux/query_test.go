package tmux

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadStateQuery(t *testing.T) {
	q := LoadStateQuery{}

	t.Run("args", func(t *testing.T) {
		expected := []string{
			"list-panes", "-a", "-F",
			"#{session_id}|#{session_name}|#{window_name}|#{window_index}|#{window_active}|#{pane_index}|#{pane_active}|#{pane_current_path}|#{pane_current_command}",
			";", "show-options", "-gv", "pane-base-index",
		}
		assert.Equal(t, expected, q.Args())
	})

	// Ensure tests don't depend on the real tmux environment
	t.Setenv("TMUX", "")

	tests := []struct {
		name   string
		output string
		want   LoadStateResult
	}{
		{"empty", "", LoadStateResult{}},
		{
			name:   "single session single window single pane",
			output: "$1|dev|editor|0|1|0|1|~/code|vim\n0",
			want: LoadStateResult{
				Sessions: []Session{{
					Name: "dev",
					Windows: []Window{{
						Name:  "editor",
						Index: 0,
						Path:  "~/code",
						Panes: []Pane{{Path: "~/code", Command: "vim"}},
					}},
				}},
				PaneBaseIndex: 0,
			},
		},
		{
			name:   "multiple panes same window",
			output: "$1|dev|editor|0|1|0|0|~/code|vim\n$1|dev|editor|0|1|1|1|~/api|node\n1",
			want: LoadStateResult{
				Sessions: []Session{{
					Name: "dev",
					Windows: []Window{{
						Name:  "editor",
						Index: 0,
						Path:  "~/code",
						Panes: []Pane{{Path: "~/code", Command: "vim"}, {Path: "~/api", Command: "node"}},
					}},
				}},
				PaneBaseIndex: 1,
			},
		},
		{
			name:   "multiple windows",
			output: "$1|dev|editor|0|0|0|0|~/code|vim\n$1|dev|server|1|1|0|1|~/api|node\n1",
			want: LoadStateResult{
				Sessions: []Session{{
					Name: "dev",
					Windows: []Window{
						{Name: "editor", Index: 0, Path: "~/code", Panes: []Pane{{Path: "~/code", Command: "vim"}}},
						{Name: "server", Index: 1, Path: "~/api", Panes: []Pane{{Path: "~/api", Command: "node"}}},
					},
				}},
				PaneBaseIndex: 1,
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
