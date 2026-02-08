package tmux

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAction_Args(t *testing.T) {
	tests := []struct {
		name   string
		action Action
		want   []string
	}{
		{
			name:   "create session",
			action: CreateSession{Name: "dev", WindowName: "editor", Path: "~/code"},
			want:   []string{"new-session", "-d", "-s", "dev", "-n", "editor", "-c", "~/code"},
		},
		{
			name:   "create session no path",
			action: CreateSession{Name: "dev", WindowName: "main"},
			want:   []string{"new-session", "-d", "-s", "dev", "-n", "main"},
		},
		{
			name:   "create window",
			action: CreateWindow{Session: "dev", Name: "editor", Path: "~/code"},
			want:   []string{"new-window", "-t", "dev", "-n", "editor", "-c", "~/code"},
		},
		{
			name:   "split pane",
			action: SplitPane{Target: "dev:editor", Path: "~/code"},
			want:   []string{"split-window", "-t", "dev:editor", "-c", "~/code"},
		},
		{
			name:   "send keys",
			action: SendKeys{Target: "dev:editor.0", Keys: "vim"},
			want:   []string{"send-keys", "-t", "dev:editor.0", "vim", "Enter"},
		},
		{
			name:   "kill session",
			action: KillSession{Name: "dev"},
			want:   []string{"kill-session", "-t", "dev"},
		},
		{
			name:   "kill window",
			action: KillWindow{Target: "dev:editor"},
			want:   []string{"kill-window", "-t", "dev:editor"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.action.Args())
		})
	}
}
