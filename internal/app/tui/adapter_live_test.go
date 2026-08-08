package tui

import (
	"context"
	"testing"

	"github.com/MSmaili/hetki/internal/backend"
	"github.com/MSmaili/hetki/internal/tui/contracts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubBackend struct {
	state       backend.StateResult
	stateErr    error
	applyErr    error
	applyCalls  [][]backend.Action
	switchCalls []string
}

func (s *stubBackend) Name() string { return "stub" }

func (s *stubBackend) QueryState() (backend.StateResult, error) {
	if s.stateErr != nil {
		return backend.StateResult{}, s.stateErr
	}
	return s.state, nil
}

func (s *stubBackend) Apply(actions []backend.Action) error {
	cloned := make([]backend.Action, len(actions))
	copy(cloned, actions)
	s.applyCalls = append(s.applyCalls, cloned)
	return s.applyErr
}

func (s *stubBackend) DryRun(actions []backend.Action) []string { return nil }
func (s *stubBackend) Attach(session string) error              { return nil }

func (s *stubBackend) Switch(target string) error {
	s.switchCalls = append(s.switchCalls, target)
	return nil
}

func TestLiveAdapterExecuteSwitchDoesNotRefresh(t *testing.T) {
	stub := &stubBackend{}
	adapter := NewLiveAdapter(func(...string) (backend.Backend, error) { return stub, nil })

	result, err := adapter.Execute(context.Background(), contracts.Intent{Type: contracts.IntentSwitch, Target: "core:1"})
	require.NoError(t, err)
	assert.False(t, result.NeedsRefresh)
	assert.Equal(t, []string{"core:1"}, stub.switchCalls)
}

func TestLiveAdapterExecuteCreateSessionInheritsWorkspace(t *testing.T) {
	stub := &stubBackend{state: backend.StateResult{
		Active: backend.ActiveContext{Session: "core"},
		Sessions: []backend.Session{
			{Name: "core", WorkspacePath: "/work/.hetki.yaml"},
		},
	}}
	adapter := NewLiveAdapter(func(...string) (backend.Backend, error) { return stub, nil })

	result, err := adapter.Execute(context.Background(), contracts.Intent{
		Type: contracts.IntentCreateSession,
		Payload: map[string]string{
			"name": "sandbox",
		},
	})
	require.NoError(t, err)
	assert.True(t, result.NeedsRefresh)
	assert.Contains(t, result.Message, "created session sandbox")
	assert.Contains(t, result.Message, "workspace linked")

	require.Len(t, stub.applyCalls, 1)
	require.Len(t, stub.applyCalls[0], 2)
	assert.Equal(t, backend.CreateSessionAction{Name: "sandbox"}, stub.applyCalls[0][0])
	assert.Equal(t, backend.SetSessionOptionAction{
		Session: "sandbox",
		Key:     backend.WorkspacePathOption,
		Value:   "/work/.hetki.yaml",
	}, stub.applyCalls[0][1])
}

func TestLiveAdapterExecuteCreateWindow(t *testing.T) {
	stub := &stubBackend{}
	adapter := NewLiveAdapter(func(...string) (backend.Backend, error) { return stub, nil })

	result, err := adapter.Execute(context.Background(), contracts.Intent{
		Type:   contracts.IntentCreateWindow,
		Target: "core:1",
		Payload: map[string]string{
			"name": "logs",
		},
	})
	require.NoError(t, err)
	assert.True(t, result.NeedsRefresh)
	assert.Equal(t, "created window core:logs", result.Message)

	require.Len(t, stub.applyCalls, 1)
	require.Len(t, stub.applyCalls[0], 1)
	assert.Equal(t, backend.CreateWindowAction{Session: "core", Name: "logs"}, stub.applyCalls[0][0])
}

func TestLiveAdapterExecuteRenameSession(t *testing.T) {
	stub := &stubBackend{}
	adapter := NewLiveAdapter(func(...string) (backend.Backend, error) { return stub, nil })

	result, err := adapter.Execute(context.Background(), contracts.Intent{
		Type:   contracts.IntentRenameSession,
		Target: "core",
		Payload: map[string]string{
			"name": "prod",
		},
	})
	require.NoError(t, err)
	assert.True(t, result.NeedsRefresh)
	assert.Equal(t, "renamed session core -> prod", result.Message)

	require.Len(t, stub.applyCalls, 1)
	require.Len(t, stub.applyCalls[0], 1)
	assert.Equal(t, backend.RenameSessionAction{Current: "core", New: "prod"}, stub.applyCalls[0][0])
}

func TestLiveAdapterExecuteRenameWindow(t *testing.T) {
	stub := &stubBackend{}
	adapter := NewLiveAdapter(func(...string) (backend.Backend, error) { return stub, nil })

	result, err := adapter.Execute(context.Background(), contracts.Intent{
		Type:   contracts.IntentRenameWindow,
		Target: "core:2",
		Payload: map[string]string{
			"name": "logs",
		},
	})
	require.NoError(t, err)
	assert.True(t, result.NeedsRefresh)
	assert.Equal(t, "renamed window core:2 -> logs", result.Message)

	require.Len(t, stub.applyCalls, 1)
	require.Len(t, stub.applyCalls[0], 1)
	assert.Equal(t, backend.RenameWindowAction{Session: "core", Window: "2", WindowID: "2", New: "logs"}, stub.applyCalls[0][0])
}

func TestLiveAdapterExecuteDeleteWindowParsesPaneTarget(t *testing.T) {
	stub := &stubBackend{}
	adapter := NewLiveAdapter(func(...string) (backend.Backend, error) { return stub, nil })

	result, err := adapter.Execute(context.Background(), contracts.Intent{
		Type:   contracts.IntentDeleteWindow,
		Target: "core:2.3",
	})
	require.NoError(t, err)
	assert.True(t, result.NeedsRefresh)
	assert.Equal(t, "deleted window core:2", result.Message)

	require.Len(t, stub.applyCalls, 1)
	require.Len(t, stub.applyCalls[0], 1)
	assert.Equal(t, backend.KillWindowAction{Session: "core", Window: "2", WindowID: "2"}, stub.applyCalls[0][0])
}

func TestLiveAdapterLoadBuildsSessionWindowTreeAndCRUDCapabilities(t *testing.T) {
	stub := &stubBackend{state: backend.StateResult{
		Active: backend.ActiveContext{Session: "core", WindowIndex: 1, Pane: 0},
		Sessions: []backend.Session{{
			ID:            "$1",
			Name:          "core",
			WorkspacePath: "/work/.hetki.yaml",
			Windows: []backend.Window{{
				ID:    "@1",
				Index: 1,
				Name:  "editor",
				Panes: []backend.Pane{{Index: 0, Command: "vim"}},
			}},
		}},
	}}
	adapter := NewLiveAdapter(func(...string) (backend.Backend, error) { return stub, nil })

	snapshot, err := adapter.Load(context.Background())
	require.NoError(t, err)
	assert.True(t, snapshot.Capabilities[contracts.CapabilityCreateSession])
	assert.True(t, snapshot.Capabilities[contracts.CapabilityCreateWindow])
	assert.True(t, snapshot.Capabilities[contracts.CapabilityRenameSession])
	assert.True(t, snapshot.Capabilities[contracts.CapabilityRenameWindow])
	assert.True(t, snapshot.Capabilities[contracts.CapabilityDeleteSession])
	assert.True(t, snapshot.Capabilities[contracts.CapabilityDeleteWindow])
	require.Len(t, snapshot.Nodes, 1)
	require.Len(t, snapshot.Nodes[0].Children, 1)
	assert.Empty(t, snapshot.Nodes[0].Children[0].Children)
	assert.Equal(t, contracts.NodeKindWindow, snapshot.Nodes[0].Children[0].Kind)
	assert.Equal(t, "$1:@1", snapshot.Nodes[0].Children[0].Target, "targets use stable IDs")
	assert.Equal(t, "$1", snapshot.Nodes[0].Target)
	assert.Equal(t, "window:@1", snapshot.ActiveNodeID)
}
