package tmux

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/MSmaili/hetki/internal/backend"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type unsupportedBackendAction struct{}

func (unsupportedBackendAction) Comment() string { return "unsupported" }
func (unsupportedBackendAction) Validate() error { return nil }

func TestCanceledContextPreventsQueryDispatch(t *testing.T) {
	called := false
	b := &TmuxBackend{client: &MockClient{RunFunc: func(context.Context, ...string) (string, error) {
		called = true
		return "", nil
	}}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := b.QueryState(ctx)

	require.ErrorIs(t, err, context.Canceled)
	assert.False(t, called)
}

func TestQueryAndMutationObserveCancellation(t *testing.T) {
	t.Run("query", func(t *testing.T) {
		started := make(chan struct{})
		b := &TmuxBackend{client: &MockClient{RunFunc: func(ctx context.Context, _ ...string) (string, error) {
			close(started)
			<-ctx.Done()
			return "", ctx.Err()
		}}}
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() {
			_, err := b.QueryState(ctx)
			done <- err
		}()
		awaitTmuxChannel(t, started)
		cancel()
		require.ErrorIs(t, awaitTmuxChannel(t, done), context.Canceled)
	})

	t.Run("mutation", func(t *testing.T) {
		started := make(chan struct{})
		b := &TmuxBackend{client: &MockClient{ExecuteFunc: func(ctx context.Context, _ Action) error {
			close(started)
			<-ctx.Done()
			return ctx.Err()
		}}}
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() {
			done <- b.Apply(ctx, []backend.Action{backend.KillSessionAction{Name: "dev"}})
		}()
		awaitTmuxChannel(t, started)
		cancel()
		require.ErrorIs(t, awaitTmuxChannel(t, done), context.Canceled)
	})
}

func TestUnsupportedBackendActionFailsApplyAndDryRunBeforeDispatch(t *testing.T) {
	dispatched := false
	b := &TmuxBackend{client: &MockClient{ExecuteFunc: func(context.Context, Action) error {
		dispatched = true
		return nil
	}}}
	actions := []backend.Action{backend.KillSessionAction{Name: "dev"}, unsupportedBackendAction{}}

	assert.ErrorContains(t, b.Apply(context.Background(), actions), "unsupported backend action")
	assert.False(t, dispatched)
	_, err := b.DryRun(actions)
	assert.ErrorContains(t, err, "unsupported backend action")
}

func TestQueryStateToleratesEmptyServer(t *testing.T) {
	t.Setenv("TMUX", "")

	cases := []error{
		errors.New("tmux [list-panes -a] failed: exit status 1 (no current target)"),
		errors.New("tmux [list-panes -a] failed: exit status 1 (no server running on /tmp/tmux-1000/default)"),
	}
	for _, runErr := range cases {
		b := &TmuxBackend{client: &MockClient{
			RunFunc: func(_ context.Context, args ...string) (string, error) {
				return "0\n0\n", runErr
			},
		}}
		res, err := b.QueryState(context.Background())
		assert.NoError(t, err)
		assert.Empty(t, res.Sessions)
	}
}

func TestQueryStateDoesNotSuppressCancellationFromEmptyServer(t *testing.T) {
	b := &TmuxBackend{client: &MockClient{RunFunc: func(context.Context, ...string) (string, error) {
		return "0\n0\n", errors.Join(context.Canceled, errors.New("no server running on /tmp/tmux/default"))
	}}}

	_, err := b.QueryState(context.Background())

	require.ErrorIs(t, err, context.Canceled)
}

func TestQueryStateDoesNotSuppressParseFailureFromEmptyServer(t *testing.T) {
	for _, output := range []string{"", "malformed"} {
		b := &TmuxBackend{client: &MockClient{RunFunc: func(context.Context, ...string) (string, error) {
			return output, errors.New("no server running on /tmp/tmux/default")
		}}}

		_, err := b.QueryState(context.Background())

		assert.ErrorContains(t, err, "missing base indexes")
		assert.ErrorContains(t, err, "no server running")
	}
}

func TestQueryStatePropagatesRealFailures(t *testing.T) {
	t.Setenv("TMUX", "")

	for _, output := range []string{
		"0\n0",
		"0\n0\n$1|dev|@1|editor|0|layout-a|0|1|%1|0|1|~/code|vim|",
	} {
		b := &TmuxBackend{client: &MockClient{
			RunFunc: func(_ context.Context, args ...string) (string, error) {
				return output, errors.New("permission denied")
			},
		}}
		_, err := b.QueryState(context.Background())
		assert.ErrorContains(t, err, "permission denied")
	}
}

func TestQueryStatePreservesStableObjectIDsAndPaneIndex(t *testing.T) {
	t.Setenv("TMUX", ",,1")
	b := &TmuxBackend{client: &MockClient{RunFunc: func(_ context.Context, args ...string) (string, error) {
		return "0\n1\n$1|dev|@2|editor|3|layout-a|0|1|%7|4|1|~/code|vim|", nil
	}}}

	result, err := b.QueryState(context.Background())
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
	b := &TmuxBackend{client: &MockClient{ExecuteFunc: func(context.Context, Action) error {
		executed = true
		return nil
	}}}

	err := b.Apply(context.Background(), []backend.Action{backend.KillWindowAction{Session: "dev", Window: "editor"}})
	assert.ErrorContains(t, err, "stable ID")
	assert.False(t, executed)
}

func TestApplyRejectsMalformedStableWindowIDs(t *testing.T) {
	for _, action := range []backend.Action{
		backend.RenameWindowAction{Session: "dev", Window: "editor", WindowID: "@bad", New: "logs"},
		backend.KillWindowAction{Session: "dev", Window: "editor", WindowID: "2"},
	} {
		executed := false
		b := &TmuxBackend{client: &MockClient{ExecuteFunc: func(context.Context, Action) error {
			executed = true
			return nil
		}}}

		err := b.Apply(context.Background(), []backend.Action{action})

		assert.ErrorContains(t, err, "invalid window ID")
		assert.False(t, executed)
	}
}

func TestCreateTargetRejectsMalformedReturnedIDs(t *testing.T) {
	for _, output := range []string{"@bad|%1", "@1|%bad"} {
		b := &TmuxBackend{client: &MockClient{RunFunc: func(context.Context, ...string) (string, error) {
			return output, nil
		}}}

		err := b.Apply(context.Background(), []backend.Action{backend.CreateWindowAction{Session: "dev", Name: "logs"}})

		assert.ErrorContains(t, err, "invalid created target")
	}
}

func TestApplyRejectsMalformedReturnedSplitPaneID(t *testing.T) {
	b := &TmuxBackend{client: &MockClient{RunFunc: func(_ context.Context, args ...string) (string, error) {
		if args[0] == "new-session" {
			return "@1|%1", nil
		}
		return "%bad", nil
	}}}

	err := b.Apply(context.Background(), []backend.Action{
		backend.CreateSessionAction{Name: "dev", WindowName: "editor"},
		backend.SplitPaneAction{Session: "dev", Window: "editor"},
	})

	assert.ErrorContains(t, err, "invalid pane ID")
}

func TestApplyUsesReturnedCreationIDsForFollowups(t *testing.T) {
	var events []string
	client := &MockClient{
		RunFunc: func(_ context.Context, args ...string) (string, error) {
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
		ExecuteFunc: func(_ context.Context, action Action) error {
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

	assert.NoError(t, b.Apply(context.Background(), actions))
	assert.Equal(t, []string{
		"run new-session -d -s dev -n editor -c ~/code -P -F #{window_id}|#{pane_id}",
		"run split-window -t @10 -c ~/api -P -F #{pane_id}",
		"exec select-layout -t @10 tiled",
		"exec send-keys -l -t %21 -- npm test ; send-keys -t %21 Enter",
		"exec resize-pane -Z -t %21",
		"run new-window -t dev: -n server -c ~/srv -P -F #{window_id}|#{pane_id}",
		"exec rename-window -t @11 logs",
		"exec kill-window -t @11",
	}, events)
}

func TestDryRunUsesSymbolicCreatedIDsInsteadOfPredictedIndexes(t *testing.T) {
	b := &TmuxBackend{}
	lines, err := b.DryRun([]backend.Action{
		backend.CreateSessionAction{Name: "dev", WindowName: "editor"},
		backend.SplitPaneAction{Session: "dev", Window: "editor", Path: "~/api"},
		backend.SendKeysAction{Session: "dev", Window: "editor", Pane: 1, Command: "npm test"},
	})
	require.NoError(t, err)

	assert.Equal(t, []string{
		"tmux new-session -d -s dev -n editor -P -F #{window_id}|#{pane_id}",
		"tmux split-window -t <new-window:dev:editor> -c ~/api -P -F #{pane_id}",
		"tmux send-keys -l -t <new-pane:dev:editor:1> -- npm test ; send-keys -t <new-pane:dev:editor:1> Enter",
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

func BenchmarkSwitchStablePaneTenThousandPanes(b *testing.B) {
	b.Setenv("TMUX", "")
	var output strings.Builder
	output.WriteString("0\n0\n")
	for i := range 10_000 {
		active := 0
		if i == 9_999 {
			active = 1
		}
		fmt.Fprintf(&output, "$1|dev|@1|editor|0|layout|0|1|%%%d|%d|%d|/work/%d|sh|\n", i, i, active, i)
	}
	stateOutput := strings.TrimSuffix(output.String(), "\n")
	backend := &TmuxBackend{client: &MockClient{RunFunc: func(context.Context, ...string) (string, error) {
		return stateOutput, nil
	}}}

	b.ReportAllocs()
	for b.Loop() {
		if err := backend.Switch(context.Background(), "%9999"); err != nil {
			b.Fatal(err)
		}
	}
}

func TestSwitchValidatesAndResolvesTargets(t *testing.T) {
	// pane base index 1; session "core" ($1) has editor (@1), logs (@2), and a
	// window literally named "logs.0" (@3); session "$2" is literally named
	// "$1" to shadow the ID; session "$3" is literally named "$1:@1".
	stateOutput := "0\n1\n" +
		"$1|core|@1|editor|1|layout|0|1|%1|0|1|/p|vim|\n" +
		"$1|core|@2|logs|2|layout|0|0|%2|0|1|/p|sh|\n" +
		"$1|core|@3|logs.0|3|layout|0|0|%3|0|1|/p|sh|\n" +
		"$2|$1|@5|editor|5|layout|0|1|%5|0|1|/p|sh|\n" +
		"$3|$1:@1|@6|w|0|layout|0|1|%6|0|1|/p|sh|\n" +
		"$4|a:b|@7|w|0|layout|0|1|%7|0|1|/p|sh|"

	tests := []struct {
		name    string
		target  string
		wantErr string
		wantNav string // captured attach-session target, empty when erroring
	}{
		{name: "empty", target: "", wantErr: "empty switch target"},
		{name: "empty session", target: ":win", wantErr: "empty session or window"},
		{name: "empty window", target: "core:", wantErr: "empty session or window"},
		{name: "empty pane", target: "core:ed.", wantErr: "invalid pane index"},
		{name: "negative pane", target: "core:ed.-1", wantErr: "invalid pane index"},
		{name: "non-numeric pane", target: "core:ed.x", wantErr: "invalid pane index"},
		{name: "malformed session ID", target: "$bad", wantErr: "invalid session ID"},
		{name: "malformed window ID", target: "core:@bad", wantErr: "invalid window ID"},
		{name: "malformed pane ID", target: "%bad", wantErr: "invalid pane ID"},
		{name: "pane index overflow", target: "core:editor." + strconv.Itoa(math.MaxInt), wantErr: "pane index overflows"},
		{name: "ambiguous name collision", target: "a:b", wantErr: "ambiguous switch target"},
		{name: "ID ref ignores name shadow", target: "$1:editor", wantNav: "$1:1"},
		{name: "ID target not blocked by ID-shaped name", target: "$1:@1", wantNav: "$1:1"},
		{name: "dotted window name collision", target: "core:logs.0", wantErr: "ambiguous switch target"},
		{name: "dotted window by ID", target: "core:@3", wantNav: "core:3"},
		{name: "plain pane target still works", target: "core:editor.0", wantNav: "core:1.1"},
		{name: "bare session name", target: "core", wantNav: "core"},
		{name: "bare session ID", target: "$1", wantNav: "$1"},
		{name: "name window by ID", target: "core:@1", wantNav: "core:1"},
		{name: "ID session with window ID", target: "$1:@1", wantNav: "$1:1"},
		{name: "ID session with window name", target: "$1:editor", wantNav: "$1:1"},
		{name: "pane offset by base index", target: "core:editor.0", wantNav: "core:1.1"},
		{name: "stable pane ID resolves current indexes", target: "%1", wantNav: "$1:1.0"},
		{name: "unknown pane ID", target: "%99", wantErr: "pane ID \"%99\" not found"},
		{name: "unknown session ID", target: "$9:editor", wantErr: "session \"$9\" not found"},
		{name: "unknown window ID", target: "core:@9", wantErr: "window ID \"@9\" not found"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("TMUX", "")
			var nav []string
			b := &TmuxBackend{client: &MockClient{
				RunFunc: func(_ context.Context, args ...string) (string, error) {
					if args[0] == "start-server" {
						return stateOutput, nil
					}
					return "", nil
				},
				ExecuteFunc: func(_ context.Context, action Action) error {
					nav = append(nav, strings.Join(action.Args(), " "))
					return nil
				},
			}}

			err := b.Switch(context.Background(), tt.target)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Len(t, nav, 1)
			assert.Equal(t, "attach-session -t "+tt.wantNav, nav[0])
		})
	}
}
