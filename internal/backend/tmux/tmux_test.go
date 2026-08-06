package tmux

import (
	"errors"
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

	// Error with parsed sessions present means something is actually wrong; don't swallow it.
	b := &TmuxBackend{client: &MockClient{
		RunFunc: func(args ...string) (string, error) {
			return "0\n0\n$1|dev|@1|editor|0|layout-a|0|1|%1|0|1|~/code|vim", errors.New("tmux boom")
		},
	}}
	_, err := b.QueryState()
	assert.Error(t, err)
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
	b := &TmuxBackend{client: &MockClient{ExecuteBatchFunc: func([]Action) error {
		executed = true
		return nil
	}}}

	err := b.Apply([]backend.Action{backend.KillWindowAction{Session: "dev", Window: "editor"}})
	assert.ErrorContains(t, err, "stable ID")
	assert.False(t, executed)
}

func TestMapActionsUsesBackendActionTypes(t *testing.T) {
	b := &TmuxBackend{windowBaseIndex: 1, paneBaseIndex: 1}

	actions := []backend.Action{
		backend.CreateSessionAction{Name: "dev", WindowName: "editor", Path: "~/code"},
		backend.RenameSessionAction{Current: "dev", New: "core"},
		backend.SplitPaneAction{Session: "dev", Window: "editor", Path: "~/api"},
		backend.SendKeysAction{Session: "dev", Window: "editor", Pane: 1, Command: "npm test"},
		backend.SelectLayoutAction{Session: "dev", Window: "editor", Layout: "tiled"},
		backend.ZoomPaneAction{Session: "dev", Window: "editor", Pane: 1},
		backend.CreateWindowAction{Session: "dev", Name: "server", Path: "~/srv"},
		backend.RenameWindowAction{Session: "dev", Window: "server", WindowID: "@2", New: "logs"},
		backend.KillSessionAction{Name: "old"},
		backend.KillWindowAction{Session: "dev", Window: "server", WindowID: "@2"},
	}

	assert.Equal(t, []Action{
		CreateSession{Name: "dev", WindowName: "editor", Path: "~/code"},
		RenameSession{Target: "dev", Name: "core"},
		SplitPane{Target: "dev:1", Path: "~/api"},
		SendKeys{Target: "dev:1.2", Keys: "npm test"},
		SelectLayout{Target: "dev:1", Layout: "tiled"},
		ZoomPane{Target: "dev:1.2"},
		CreateWindow{Session: "dev", Name: "server", Path: "~/srv"},
		RenameWindow{Target: "@2", Name: "logs"},
		KillSession{Name: "old"},
		KillWindow{Target: "@2"},
	}, b.mapActions(actions))
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
