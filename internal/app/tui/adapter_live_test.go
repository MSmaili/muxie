package tui

import (
	"context"
	"testing"

	"github.com/MSmaili/hetki/internal/backend"
	"github.com/MSmaili/hetki/internal/tui/core"
	"github.com/MSmaili/hetki/internal/tui/list"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubBackend struct {
	state       backend.StateResult
	stateErr    error
	applyErr    error
	applyCalls  [][]backend.Action
	applyHook   func([]backend.Action)
	switchCalls []string
}

func (s *stubBackend) Name() string { return "stub" }

func (s *stubBackend) QueryState(context.Context) (backend.StateResult, error) {
	if s.stateErr != nil {
		return backend.StateResult{}, s.stateErr
	}
	return s.state, nil
}

func (s *stubBackend) Apply(_ context.Context, actions []backend.Action) error {
	cloned := append([]backend.Action(nil), actions...)
	s.applyCalls = append(s.applyCalls, cloned)
	if s.applyHook != nil {
		s.applyHook(cloned)
	}
	return s.applyErr
}

func (s *stubBackend) DryRun([]backend.Action) ([]string, error) { return nil, nil }
func (s *stubBackend) Attach(context.Context, string) error      { return nil }
func (s *stubBackend) Switch(_ context.Context, target string) error {
	s.switchCalls = append(s.switchCalls, target)
	return nil
}

func liveState() backend.StateResult {
	return backend.StateResult{
		Active: backend.ActiveContext{Session: "core", WindowIndex: 1, Pane: 0},
		Sessions: []backend.Session{{
			ID: "$1", Name: "core", WorkspacePath: "/work/.hetki.yaml",
			Windows: []backend.Window{{ID: "@1", Index: 1, Name: "editor", Path: "/work/editor", Panes: []backend.Pane{{ID: "%1", Index: 0}}}},
		}},
	}
}

func loadedAdapter(t *testing.T, stub *stubBackend) *LiveAdapter {
	t.Helper()
	adapter := NewLiveAdapter(func(...string) (backend.Backend, error) { return stub, nil })
	_, err := adapter.Load(context.Background())
	require.NoError(t, err)
	return adapter
}

func text(value string) *string { return &value }

func TestLiveAdapterProjectsPresentationAndKeepsTargetsInOwnerIndex(t *testing.T) {
	stub := &stubBackend{state: liveState()}
	adapter := NewLiveAdapter(func(...string) (backend.Backend, error) { return stub, nil })

	snapshot, err := adapter.Load(context.Background())
	require.NoError(t, err)
	require.NoError(t, validateProjection(snapshot, adapter.index))
	require.Len(t, snapshot.Items, 1)
	require.Len(t, snapshot.Items[0].Children, 1)
	assert.Equal(t, list.ItemID("session:$1"), snapshot.Items[0].ID)
	assert.Equal(t, list.ItemID("window:@1"), snapshot.Items[0].Children[0].ID)
	assert.Equal(t, list.ItemID("window:@1"), snapshot.ActiveItemID)
	assert.Contains(t, snapshot.Items[0].Primary, "core")
	assert.Equal(t, "$1:@1", adapter.index["window:@1"].Target)
}

func TestLiveAdapterResolvesOpenFromCurrentItemIndex(t *testing.T) {
	stub := &stubBackend{state: liveState()}
	adapter := loadedAdapter(t, stub)

	result, err := adapter.Execute(context.Background(), core.ActionRequest{ActionID: core.ActionOpen, ItemID: "window:@1"})
	require.NoError(t, err)
	assert.Empty(t, stub.switchCalls)
	assert.Equal(t, core.BackendTarget("$1:@1"), result.Navigation)
	require.NoError(t, adapter.Navigate(context.Background(), result.Navigation))
	assert.Equal(t, []string{"$1:@1"}, stub.switchCalls)

	_, err = adapter.Execute(context.Background(), core.ActionRequest{ActionID: core.ActionOpen, ItemID: "missing"})
	require.ErrorContains(t, err, "stale")
}

func TestLiveAdapterRejectsAnItemRemovedAfterTheSnapshot(t *testing.T) {
	stub := &stubBackend{state: liveState()}
	adapter := loadedAdapter(t, stub)
	stub.state.Sessions[0].Windows = nil

	result, err := adapter.Execute(context.Background(), core.ActionRequest{ActionID: core.ActionOpen, ItemID: "window:@1"})
	require.ErrorContains(t, err, "stale")
	assert.Empty(t, result.Navigation)
	assert.Empty(t, stub.switchCalls)
}

func TestLiveAdapterCreateWindowPromptsThenRefreshesAndSelectsCreatedItem(t *testing.T) {
	stub := &stubBackend{state: liveState()}
	stub.applyHook = func(actions []backend.Action) {
		require.Equal(t, backend.CreateWindowAction{Session: "$1", Name: "logs"}, actions[0])
		stub.state.Sessions[0].Windows = append(stub.state.Sessions[0].Windows, backend.Window{ID: "@2", Index: 2, Name: "logs"})
	}
	adapter := loadedAdapter(t, stub)
	request := core.ActionRequest{ActionID: core.ActionCreateWindow, ItemID: "window:@1"}

	prompt, err := adapter.Execute(context.Background(), request)
	require.NoError(t, err)
	require.Equal(t, "CREATE WINDOW", prompt.Input.Title)

	request.Value = text("logs")
	result, err := adapter.Execute(context.Background(), request)
	require.NoError(t, err)
	require.NotNil(t, result.Snapshot)
	assert.Equal(t, list.ItemID("window:@2"), result.SelectItemID)
	require.Len(t, stub.applyCalls, 1)
}

func TestLiveAdapterRenameAndDeleteReResolveTheSameStableItem(t *testing.T) {
	stub := &stubBackend{state: liveState()}
	adapter := loadedAdapter(t, stub)
	rename := core.ActionRequest{ActionID: core.ActionRename, ItemID: "window:@1"}

	prompt, err := adapter.Execute(context.Background(), rename)
	require.NoError(t, err)
	assert.Equal(t, "editor", prompt.Input.InitialValue)

	stub.applyHook = func(actions []backend.Action) {
		require.Equal(t, backend.RenameWindowAction{Session: "$1", Window: "@1", WindowID: "@1", New: "api"}, actions[0])
		stub.state.Sessions[0].Windows[0].Name = "api"
	}
	rename.Value = text("api")
	result, err := adapter.Execute(context.Background(), rename)
	require.NoError(t, err)
	assert.Equal(t, list.ItemID("window:@1"), result.SelectItemID)

	remove := core.ActionRequest{ActionID: core.ActionDelete, ItemID: "window:@1"}
	confirmation, err := adapter.Execute(context.Background(), remove)
	require.NoError(t, err)
	assert.Equal(t, "DELETE WINDOW", confirmation.Confirmation.Title)
	stub.applyHook = func(actions []backend.Action) {
		require.Equal(t, backend.KillWindowAction{Session: "$1", Window: "@1", WindowID: "@1"}, actions[0])
		stub.state.Sessions[0].Windows = nil
	}
	remove.Confirmed = true
	result, err = adapter.Execute(context.Background(), remove)
	require.NoError(t, err)
	assert.Equal(t, list.ItemID("session:$1"), result.SelectItemID)
}

func TestLiveAdapterCreateSessionInheritsWorkspace(t *testing.T) {
	stub := &stubBackend{state: liveState()}
	stub.applyHook = func(actions []backend.Action) {
		require.Equal(t, []backend.Action{
			backend.CreateSessionAction{Name: "sandbox"},
			backend.SetSessionOptionAction{Session: "sandbox", Key: backend.WorkspacePathOption, Value: "/work/.hetki.yaml"},
		}, actions)
		stub.state.Sessions = append(stub.state.Sessions, backend.Session{ID: "$2", Name: "sandbox"})
	}
	adapter := loadedAdapter(t, stub)

	prompt, err := adapter.Execute(context.Background(), core.ActionRequest{ActionID: core.ActionCreateSession})
	require.NoError(t, err)
	assert.Equal(t, "CREATE SESSION", prompt.Input.Title)
	result, err := adapter.Execute(context.Background(), core.ActionRequest{ActionID: core.ActionCreateSession, Value: text("sandbox")})
	require.NoError(t, err)
	assert.Contains(t, result.Message, "workspace linked")
	assert.Equal(t, list.ItemID("session:$2"), result.SelectItemID)
}

func TestInvalidProjectionRetainsPreviousOwnerIndex(t *testing.T) {
	stub := &stubBackend{state: liveState()}
	adapter := loadedAdapter(t, stub)
	stub.state.Sessions = append(stub.state.Sessions, stub.state.Sessions[0])

	_, err := adapter.Load(context.Background())
	require.ErrorContains(t, err, "duplicate item ID")
	result, err := adapter.Execute(context.Background(), core.ActionRequest{ActionID: core.ActionOpen, ItemID: "window:@1"})
	require.NoError(t, err)
	assert.Equal(t, core.BackendTarget("$1:@1"), result.Navigation)
}

func TestLiveAdapterRejectsMissingStableSessionAndWindowIDsDuringProjection(t *testing.T) {
	for _, test := range []struct {
		name string
		edit func(*backend.StateResult)
		want string
	}{
		{name: "session", edit: func(state *backend.StateResult) { state.Sessions[0].ID = "" }, want: "stable $N ID"},
		{name: "window", edit: func(state *backend.StateResult) { state.Sessions[0].Windows[0].ID = "" }, want: "stable @N ID"},
	} {
		t.Run(test.name, func(t *testing.T) {
			state := liveState()
			test.edit(&state)
			stub := &stubBackend{state: state}
			adapter := NewLiveAdapter(func(...string) (backend.Backend, error) { return stub, nil })

			_, err := adapter.Load(context.Background())
			require.ErrorContains(t, err, test.want)
			assert.Empty(t, adapter.index)
			assert.Empty(t, stub.applyCalls)
		})
	}
}
