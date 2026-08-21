package tui

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/MSmaili/hetki/internal/backend"
	"github.com/MSmaili/hetki/internal/frecency"
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
	switchErr   error
	switchHook  func(string)
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
	if s.switchHook != nil {
		s.switchHook(target)
	}
	return s.switchErr
}

func liveState() backend.StateResult {
	return backend.StateResult{
		Active: backend.ActiveContext{SessionID: "$1", Session: "core", WindowID: "@1", WindowIndex: 1, PaneID: "%1", Pane: 0, Path: "/work/editor"},
		Sessions: []backend.Session{{
			ID: "$1", Name: "core", WorkspacePath: "/work/.hetki.yaml",
			Windows: []backend.Window{{ID: "@1", Index: 1, Name: "editor", Path: "/work/editor", Active: true, Panes: []backend.Pane{{ID: "%1", Index: 0, Path: "/work/editor", Active: true}}}},
		}},
	}
}

func testAdapter(stub *stubBackend) *LiveAdapter {
	adapter := newLiveAdapter(func(...string) (backend.Backend, error) { return stub, nil }, nil, nil)
	adapter.projection = projectionTree
	return adapter
}

func loadedAdapter(t *testing.T, stub *stubBackend) *LiveAdapter {
	t.Helper()
	adapter := testAdapter(stub)
	_, err := adapter.Load(context.Background())
	require.NoError(t, err)
	return adapter
}

func text(value string) *string { return &value }

func TestLiveAdapterProjectsPresentationAndKeepsTargetsInOwnerIndex(t *testing.T) {
	stub := &stubBackend{state: liveState()}
	adapter := testAdapter(stub)

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

func TestLiveAdapterStartsWithTheFlatProjection(t *testing.T) {
	stub := &stubBackend{state: liveState()}
	adapter := newLiveAdapter(func(...string) (backend.Backend, error) { return stub, nil }, nil, nil)

	snapshot, err := adapter.Load(context.Background())
	require.NoError(t, err)
	require.Len(t, snapshot.Items, 1)
	assert.Empty(t, snapshot.Items[0].Children)
	assert.Equal(t, destinationItemID("$1", "@1", "/work/editor"), snapshot.ActiveItemID)
}

func TestLiveAdapterLoadsFrecencyBeforeProjectingFlatRows(t *testing.T) {
	path := filepath.Join(t.TempDir(), "frecency.json")
	store := frecency.NewStore(path)
	require.NoError(t, store.Record(context.Background(), "/z", "zeta"))
	state := backend.StateResult{Sessions: []backend.Session{
		{ID: "$1", Name: "alpha", Windows: []backend.Window{{ID: "@1", Name: "a", Panes: []backend.Pane{{ID: "%1", Path: "/a"}}}}},
		{ID: "$2", Name: "zeta", Windows: []backend.Window{{ID: "@2", Name: "z", Panes: []backend.Pane{{ID: "%2", Path: "/z"}}}}},
	}}
	stub := &stubBackend{state: state}
	adapter := newLiveAdapter(func(...string) (backend.Backend, error) { return stub, nil }, store, nil)

	snapshot, err := adapter.Load(context.Background())
	require.NoError(t, err)
	require.Len(t, snapshot.Items, 2)
	assert.Equal(t, destinationItemID("$2", "@2", "/z"), snapshot.Items[0].ID)
}

func TestLiveAdapterRecordsOnlyAfterSuccessfulNavigation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "frecency.json")
	store := frecency.NewStore(path)
	stub := &stubBackend{state: liveState()}
	adapter := newLiveAdapter(func(...string) (backend.Backend, error) { return stub, nil }, store, nil)
	snapshot, err := adapter.Load(context.Background())
	require.NoError(t, err)
	id := snapshot.Items[0].ID
	result, err := adapter.Execute(context.Background(), core.ActionRequest{ActionID: core.ActionOpen, ItemID: id})
	require.NoError(t, err)
	stub.switchHook = func(string) {
		records, loadErr := store.Load()
		require.NoError(t, loadErr)
		assert.Empty(t, records, "navigation was recorded before the backend switch")
	}

	require.NoError(t, adapter.Navigate(context.Background(), result.Navigation))
	records, err := store.Load()
	require.NoError(t, err)
	require.Len(t, records, 1)
	assert.Equal(t, frecency.Record{Path: "/work/editor", Session: "core", Rank: 1, LastUsed: records[0].LastUsed}, records[0])

	failedPath := filepath.Join(t.TempDir(), "frecency.json")
	failedStore := frecency.NewStore(failedPath)
	failedBackend := &stubBackend{state: liveState(), switchErr: errors.New("switch failed")}
	failed := newLiveAdapter(func(...string) (backend.Backend, error) { return failedBackend, nil }, failedStore, nil)
	failedSnapshot, err := failed.Load(context.Background())
	require.NoError(t, err)
	result, err = failed.Execute(context.Background(), core.ActionRequest{ActionID: core.ActionOpen, ItemID: failedSnapshot.Items[0].ID})
	require.NoError(t, err)
	require.Error(t, failed.Navigate(context.Background(), result.Navigation))
	_, err = os.Stat(failedPath)
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestLiveAdapterRecordsTreeNavigationAtTheActivePanePath(t *testing.T) {
	store := frecency.NewStore(filepath.Join(t.TempDir(), "frecency.json"))
	stub := &stubBackend{state: liveState()}
	adapter := newLiveAdapter(func(...string) (backend.Backend, error) { return stub, nil }, store, nil)
	adapter.projection = projectionTree
	snapshot, err := adapter.Load(context.Background())
	require.NoError(t, err)
	result, err := adapter.Execute(context.Background(), core.ActionRequest{ActionID: core.ActionOpen, ItemID: snapshot.Items[0].Children[0].ID})
	require.NoError(t, err)
	require.NoError(t, adapter.Navigate(context.Background(), result.Navigation))

	records, err := store.Load()
	require.NoError(t, err)
	require.Len(t, records, 1)
	assert.Equal(t, "/work/editor", records[0].Path)
	assert.Equal(t, "core", records[0].Session)
}

func TestLiveAdapterShowsInvalidFrecencyAndPreservesItBeforeRecording(t *testing.T) {
	path := filepath.Join(t.TempDir(), "frecency.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"version":`), 0600))
	store := frecency.NewStore(path)
	stub := &stubBackend{state: liveState()}
	adapter := newLiveAdapter(func(...string) (backend.Backend, error) { return stub, nil }, store, nil)

	snapshot, err := adapter.Load(context.Background())
	require.NoError(t, err)
	assert.Contains(t, snapshot.Notice, "frecency state")
	result, err := adapter.Execute(context.Background(), core.ActionRequest{ActionID: core.ActionOpen, ItemID: snapshot.Items[0].ID})
	require.NoError(t, err)
	require.NoError(t, adapter.Navigate(context.Background(), result.Navigation))

	backups, err := filepath.Glob(path + ".corrupt-*")
	require.NoError(t, err)
	require.Len(t, backups, 1)
	original, err := os.ReadFile(backups[0])
	require.NoError(t, err)
	assert.Equal(t, `{"version":`, string(original))
}

func TestLiveAdapterPropagatesFrecencyCancellation(t *testing.T) {
	adapter := newLiveAdapter(nil, frecency.NewStore(filepath.Join(t.TempDir(), "frecency.json")), nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, notice, err := adapter.loadFrecency(ctx)
	require.ErrorIs(t, err, context.Canceled)
	assert.Empty(t, notice)
}

func TestLiveAdapterSurfacesUnavailableFrecencyWithoutBlockingLoad(t *testing.T) {
	stub := &stubBackend{state: liveState()}
	adapter := newLiveAdapter(func(...string) (backend.Backend, error) { return stub, nil }, nil, errors.New("home is unavailable"))

	snapshot, err := adapter.Load(context.Background())
	require.NoError(t, err)
	assert.Contains(t, snapshot.Notice, "home is unavailable")
	require.Len(t, snapshot.Items, 1)
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

func TestLiveAdapterRejectsStaleFlatPaneIDsAndPaths(t *testing.T) {
	for _, test := range []struct {
		name string
		edit func(*backend.Pane)
	}{
		{name: "pane replaced", edit: func(pane *backend.Pane) { pane.ID = "%2" }},
		{name: "pane changed directory", edit: func(pane *backend.Pane) { pane.Path = "/work/other" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			stub := &stubBackend{state: liveState()}
			adapter := loadedAdapter(t, stub)
			_, err := adapter.Execute(context.Background(), core.ActionRequest{ActionID: core.ActionToggleProjection, ItemID: "window:@1"})
			require.NoError(t, err)
			id := destinationItemID("$1", "@1", "/work/editor")
			test.edit(&stub.state.Sessions[0].Windows[0].Panes[0])

			result, err := adapter.Execute(context.Background(), core.ActionRequest{ActionID: core.ActionOpen, ItemID: id})
			require.ErrorContains(t, err, "stale")
			assert.Empty(t, result.Navigation)
		})
	}
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

func TestLiveAdapterTogglesBetweenTreeAndFlatWithStableSelection(t *testing.T) {
	stub := &stubBackend{state: liveState()}
	adapter := loadedAdapter(t, stub)
	destinationID := destinationItemID("$1", "@1", "/work/editor")

	flat, err := adapter.Execute(context.Background(), core.ActionRequest{
		ActionID: core.ActionToggleProjection, ItemID: "window:@1",
	})
	require.NoError(t, err)
	require.Len(t, flat.Snapshot.Items, 1)
	assert.Equal(t, destinationID, flat.SelectItemID)
	assert.Empty(t, flat.Snapshot.Items[0].Children)

	opened, err := adapter.Execute(context.Background(), core.ActionRequest{ActionID: core.ActionOpen, ItemID: destinationID})
	require.NoError(t, err)
	assert.Equal(t, core.BackendTarget("%1"), opened.Navigation)

	tree, err := adapter.Execute(context.Background(), core.ActionRequest{
		ActionID: core.ActionToggleProjection, ItemID: destinationID,
	})
	require.NoError(t, err)
	assert.Equal(t, list.ItemID("window:@1"), tree.SelectItemID)
	assert.Equal(t, projectionTree, adapter.projection)
}

func TestTreeToFlatPrefersTheSelectedSessionsActiveDestination(t *testing.T) {
	state := liveState()
	state.Sessions = append(state.Sessions, backend.Session{
		ID: "$2", Name: "ops", Windows: []backend.Window{{
			ID: "@2", Name: "logs", Active: true,
			Panes: []backend.Pane{{ID: "%2", Index: 0, Path: "/ops/logs", Active: true}},
		}},
	})
	stub := &stubBackend{state: state}
	adapter := loadedAdapter(t, stub)

	result, err := adapter.Execute(context.Background(), core.ActionRequest{
		ActionID: core.ActionToggleProjection, ItemID: "session:$2",
	})
	require.NoError(t, err)
	assert.Equal(t, destinationItemID("$2", "@2", "/ops/logs"), result.SelectItemID)
}

func TestFlatDestinationActionsUseTheOwningWindowAndSession(t *testing.T) {
	stub := &stubBackend{state: liveState()}
	adapter := loadedAdapter(t, stub)
	_, err := adapter.Execute(context.Background(), core.ActionRequest{ActionID: core.ActionToggleProjection, ItemID: "window:@1"})
	require.NoError(t, err)
	destinationID := destinationItemID("$1", "@1", "/work/editor")

	rename := core.ActionRequest{ActionID: core.ActionRename, ItemID: destinationID, Value: text("api")}
	stub.applyHook = func(actions []backend.Action) {
		require.Equal(t, backend.RenameWindowAction{Session: "$1", Window: "@1", WindowID: "@1", New: "api"}, actions[0])
		stub.state.Sessions[0].Windows[0].Name = "api"
	}
	result, err := adapter.Execute(context.Background(), rename)
	require.NoError(t, err)
	assert.Equal(t, destinationID, result.SelectItemID)

	create := core.ActionRequest{ActionID: core.ActionCreateWindow, ItemID: destinationID, Value: text("logs")}
	stub.applyHook = func(actions []backend.Action) {
		require.Equal(t, backend.CreateWindowAction{Session: "$1", Name: "logs"}, actions[0])
		stub.state.Sessions[0].Windows = append(stub.state.Sessions[0].Windows, backend.Window{
			ID: "@2", Name: "logs", Index: 2, Panes: []backend.Pane{{ID: "%2", Path: "/work/logs"}},
		})
	}
	result, err = adapter.Execute(context.Background(), create)
	require.NoError(t, err)
	assert.Equal(t, destinationItemID("$1", "@2", "/work/logs"), result.SelectItemID)

	remove := core.ActionRequest{ActionID: core.ActionDelete, ItemID: destinationID, Confirmed: true}
	stub.applyHook = func(actions []backend.Action) {
		require.Equal(t, backend.KillWindowAction{Session: "$1", Window: "@1", WindowID: "@1"}, actions[0])
		stub.state.Sessions[0].Windows = stub.state.Sessions[0].Windows[1:]
	}
	_, err = adapter.Execute(context.Background(), remove)
	require.NoError(t, err)
}

func TestInvalidFlatProjectionRetainsTreeAndOwnerIndex(t *testing.T) {
	state := liveState()
	state.Sessions[0].Windows[0].Panes[0].ID = "unstable"
	stub := &stubBackend{state: state}
	adapter := loadedAdapter(t, stub)

	_, err := adapter.Execute(context.Background(), core.ActionRequest{
		ActionID: core.ActionToggleProjection, ItemID: "window:@1",
	})
	require.ErrorContains(t, err, "stable %N ID")
	assert.Equal(t, projectionTree, adapter.projection)
	assert.Contains(t, adapter.index, list.ItemID("window:@1"))
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
			adapter := newLiveAdapter(func(...string) (backend.Backend, error) { return stub, nil }, nil, nil)

			_, err := adapter.Load(context.Background())
			require.ErrorContains(t, err, test.want)
			assert.Empty(t, adapter.index)
			assert.Empty(t, stub.applyCalls)
		})
	}
}
