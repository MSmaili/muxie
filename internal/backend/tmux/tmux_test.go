package tmux

import (
	"errors"
	"strings"
	"testing"

	"github.com/MSmaili/hetki/internal/backend"
	"github.com/stretchr/testify/assert"
)

func TestQueryStateToleratesEmptyServer(t *testing.T) {
	t.Setenv("TMUX", "")

	cases := []error{
		errors.New("tmux [list-panes -a] failed: exit status 1 (no current target)"),
		errors.New("tmux [list-panes -a] failed: exit status 1 (no server running on /tmp/tmux-1000/default)"),
	}
	for _, runErr := range cases {
		b := &TmuxBackend{client: &MockClient{
			RunFunc: func(args ...string) (string, error) {
				return "0\n0\n", runErr
			},
		}}
		res, err := b.QueryState()
		assert.NoError(t, err)
		assert.Empty(t, res.Sessions)
	}
}

func TestQueryStatePropagatesRealFailures(t *testing.T) {
	t.Setenv("TMUX", "")

	for _, output := range []string{
		"0\n0",
		"0\n0\n$1|dev|@1|editor|0|layout-a|0|1|%1|0|1|~/code|vim",
	} {
		b := &TmuxBackend{client: &MockClient{
			RunFunc: func(args ...string) (string, error) {
				return output, errors.New("permission denied")
			},
		}}
		_, err := b.QueryState()
		assert.ErrorContains(t, err, "permission denied")
	}
}

func TestQueryStatePreservesStableObjectIDsAndPaneIndex(t *testing.T) {
	t.Setenv("TMUX", ",,1")
	b := &TmuxBackend{client: &MockClient{RunFunc: func(args ...string) (string, error) {
		return "0\n1\n$1|dev|@2|editor|3|layout-a|0|1|%7|4|1|~/code|vim", nil
	}}}

	result, err := b.QueryState()
	assert.NoError(t, err)
	if assert.Len(t, result.Sessions, 1) && assert.Len(t, result.Sessions[0].Windows, 1) && assert.Len(t, result.Sessions[0].Windows[0].Panes, 1) {
		assert.Equal(t, "$1", result.Sessions[0].ID)
		assert.Equal(t, "@2", result.Sessions[0].Windows[0].ID)
		assert.Equal(t, "%7", result.Sessions[0].Windows[0].Panes[0].ID)
		assert.Equal(t, 4, result.Sessions[0].Windows[0].Panes[0].Index)
		assert.Equal(t, backend.ActiveContext{SessionID: "$1", Session: "dev", WindowID: "@2", Window: "editor", WindowIndex: 3, PaneID: "%7", Pane: 4, Path: "~/code"}, result.Active)
	}
}

func TestApplyRejectsWindowDestructionWithoutStableID(t *testing.T) {
	executed := false
	b := &TmuxBackend{client: &MockClient{ExecuteFunc: func(Action) error {
		executed = true
		return nil
	}}}

	err := b.Apply([]backend.Action{backend.KillWindowAction{Session: "dev", Window: "editor"}})
	assert.ErrorContains(t, err, "stable ID")
	assert.False(t, executed)
}

func TestApplyUsesReturnedCreationIDsForFollowups(t *testing.T) {
	var events []string
	client := &MockClient{
		RunFunc: func(args ...string) (string, error) {
			events = append(events, "run "+strings.Join(args, " "))
			switch args[0] {
			case "new-session":
				return "@10|%20", nil
			case "new-window":
				return "@11|%30", nil
			case "split-window":
				return "%21", nil
			default:
				return "", nil
			}
		},
		ExecuteFunc: func(action Action) error {
			events = append(events, "exec "+strings.Join(action.Args(), " "))
			return nil
		},
	}
	b := &TmuxBackend{client: client}
	actions := []backend.Action{
		backend.CreateSessionAction{Name: "dev", WindowName: "editor", Path: "~/code"},
		backend.SplitPaneAction{Session: "dev", Window: "editor", Path: "~/api"},
		backend.SelectLayoutAction{Session: "dev", Window: "editor", Layout: "tiled"},
		backend.SendKeysAction{Session: "dev", Window: "editor", Pane: 1, Command: "npm test"},
		backend.ZoomPaneAction{Session: "dev", Window: "editor", Pane: 1},
		backend.CreateWindowAction{Session: "dev", Name: "server", Path: "~/srv"},
		backend.RenameWindowAction{Session: "dev", Window: "server", WindowID: "@11", New: "logs"},
		backend.KillWindowAction{Session: "dev", Window: "logs", WindowID: "@11"},
	}

	assert.NoError(t, b.Apply(actions))
	assert.Equal(t, []string{
		"run new-session -d -s dev -n editor -c ~/code -P -F #{window_id}|#{pane_id}",
		"run split-window -t @10 -c ~/api -P -F #{pane_id}",
		"exec select-layout -t @10 tiled",
		"exec send-keys -t %21 npm test Enter",
		"exec resize-pane -Z -t %21",
		"run new-window -t dev: -n server -c ~/srv -P -F #{window_id}|#{pane_id}",
		"exec rename-window -t @11 logs",
		"exec kill-window -t @11",
	}, events)
}

func TestDryRunUsesSymbolicCreatedIDsInsteadOfPredictedIndexes(t *testing.T) {
	b := &TmuxBackend{}
	lines := b.DryRun([]backend.Action{
		backend.CreateSessionAction{Name: "dev", WindowName: "editor"},
		backend.SplitPaneAction{Session: "dev", Window: "editor", Path: "~/api"},
		backend.SendKeysAction{Session: "dev", Window: "editor", Pane: 1, Command: "npm test"},
	})

	assert.Equal(t, []string{
		"tmux new-session -d -s dev -n editor -P -F #{window_id}|#{pane_id}",
		"tmux split-window -t <new-window:dev:editor> -c ~/api -P -F #{pane_id}",
		"tmux send-keys -t <new-pane:dev:editor:1> npm test Enter",
	}, lines)
}

func TestResolveWindowIndex(t *testing.T) {
	sessions := []Session{
		{
			Name: "dev",
			Windows: []Window{
				{ID: "@1", Name: "editor", Index: 1},
				{ID: "@2", Name: "editor", Index: 2},
				{ID: "@3", Name: "logs", Index: 3},
			},
		},
	}

	t.Run("resolves numeric index directly", func(t *testing.T) {
		idx, err := resolveWindowIndex(sessions, "dev", "2")
		assert.NoError(t, err)
		assert.Equal(t, 2, idx)
	})

	t.Run("resolves stable ID", func(t *testing.T) {
		idx, err := resolveWindowIndex(sessions, "dev", "@2")
		assert.NoError(t, err)
		assert.Equal(t, 2, idx)
	})

	t.Run("resolves unambiguous name for legacy targets", func(t *testing.T) {
		idx, err := resolveWindowIndex(sessions, "dev", "logs")
		assert.NoError(t, err)
		assert.Equal(t, 3, idx)
	})

	t.Run("fails for ambiguous legacy name", func(t *testing.T) {
		_, err := resolveWindowIndex(sessions, "dev", "editor")
		assert.ErrorContains(t, err, "ambiguous")
	})

	t.Run("fails for unknown numeric index", func(t *testing.T) {
		_, err := resolveWindowIndex(sessions, "dev", "99")
		assert.Error(t, err)
	})
}
