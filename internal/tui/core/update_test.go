package core

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/MSmaili/hetki/internal/tui/contracts"
	"github.com/stretchr/testify/require"
)

func interactionSnapshot() contracts.Snapshot {
	capabilities := map[contracts.Capability]bool{
		contracts.CapabilityRefresh:       true,
		contracts.CapabilitySwitch:        true,
		contracts.CapabilityCreateSession: true,
		contracts.CapabilityCreateWindow:  true,
		contracts.CapabilityRenameSession: true,
		contracts.CapabilityRenameWindow:  true,
		contracts.CapabilityDeleteSession: true,
		contracts.CapabilityDeleteWindow:  true,
	}
	return contracts.Snapshot{
		ActiveNodeID: "window:@1",
		Capabilities: capabilities,
		Nodes: []contracts.Node{
			{
				ID: "session:$1", Kind: contracts.NodeKindSession, Label: "dev", Name: "dev", Target: "$1", Active: true,
				Children: []contracts.Node{
					{ID: "window:@1", ParentID: "session:$1", Kind: contracts.NodeKindWindow, Label: "0 editor", Name: "editor", Target: "$1:@1", Path: "~/code/editor", Active: true},
					{ID: "window:@2", ParentID: "session:$1", Kind: contracts.NodeKindWindow, Label: "1 server", Name: "server", Target: "$1:@2", Path: "~/svc/api"},
				},
			},
			{
				ID: "session:$2", Kind: contracts.NodeKindSession, Label: "prod", Name: "prod", Target: "$2",
				Children: []contracts.Node{
					{ID: "window:@3", ParentID: "session:$2", Kind: contracts.NodeKindWindow, Label: "0 shell", Name: "shell", Target: "$2:@3", Path: "~/prod/app"},
				},
			},
		},
	}
}

func printableKey(text string) tea.KeyPressMsg {
	runes := []rune(text)
	return tea.KeyPressMsg{Code: runes[0], Text: text}
}

func specialKey(code rune) tea.KeyPressMsg { return tea.KeyPressMsg{Code: code} }

func controlKey(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: code, Mod: tea.ModCtrl}
}

func updateModel(t *testing.T, m model, msg tea.Msg) (model, tea.Cmd) {
	t.Helper()
	updated, cmd := m.Update(msg)
	result, ok := updated.(model)
	require.True(t, ok, "Update returned %T", updated)
	return result, cmd
}

func selectNode(t *testing.T, m model, id contracts.NodeID) model {
	t.Helper()
	idx := findRowIndexByID(m.rows, id)
	require.NotEqual(t, -1, idx, "node %q is not visible", id)
	m.cursor = idx
	return m.reflow()
}

func requireQuit(t *testing.T, cmd tea.Cmd) {
	t.Helper()
	require.NotNil(t, cmd)
	_, ok := cmd().(tea.QuitMsg)
	require.True(t, ok, "command did not return tea.QuitMsg")
}

func TestUpdateRoutesKeysByModeAndOverlay(t *testing.T) {
	m := newModel(interactionSnapshot(), nil)
	require.Equal(t, modeFilter, m.mode)

	m, cmd := updateModel(t, m, printableKey("q"))
	require.Nil(t, cmd)
	require.Equal(t, "q", m.filter, "filter must own printable shortcut keys")

	m, _ = updateModel(t, m, specialKey(tea.KeyEscape))
	require.Equal(t, modeBrowse, m.mode)
	require.Equal(t, "q", m.filter, "escape must preserve the query")

	m, _ = updateModel(t, m, printableKey("/"))
	require.Equal(t, modeFilter, m.mode)

	m.mode = modeInput
	m.input = inputState{Value: "name"}
	m, cmd = updateModel(t, m, printableKey("q"))
	require.Nil(t, cmd)
	require.Equal(t, "nameq", m.input.Value)
	m, _ = updateModel(t, m, specialKey(tea.KeyEscape))
	require.Equal(t, modeBrowse, m.mode)

	m.mode = modeConfirm
	m.confirm = confirmState{Intent: contracts.Intent{Type: contracts.IntentDeleteSession, Target: "$1"}}
	m, cmd = updateModel(t, m, printableKey("q"))
	require.Nil(t, cmd, "confirmation must own q instead of quitting")
	require.Equal(t, modeConfirm, m.mode)
	m, _ = updateModel(t, m, printableKey("n"))
	require.Equal(t, modeBrowse, m.mode)

	m.helpOpen = true
	m, cmd = updateModel(t, m, printableKey("q"))
	require.Nil(t, cmd)
	require.True(t, m.helpOpen)
	m, _ = updateModel(t, m, specialKey(tea.KeyEnter))
	require.False(t, m.helpOpen)

	m.busy = true
	m, cmd = updateModel(t, m, printableKey("q"))
	require.Nil(t, cmd)
	require.True(t, m.busy)
	_, cmd = updateModel(t, m, controlKey('c'))
	requireQuit(t, cmd)
}

func TestEnterSwitchesFromFilterAndPreservesTypedTarget(t *testing.T) {
	var got contracts.Intent
	m := newModel(interactionSnapshot(), func(intent contracts.Intent) (contracts.ActionResult, error) {
		got = intent
		return contracts.ActionResult{}, nil
	})
	m.filter = "server"
	m.applyFilter()
	require.Equal(t, "window:@2", m.selectedNodeID())

	m, cmd := updateModel(t, m, specialKey(tea.KeyEnter))
	require.Equal(t, modeBrowse, m.mode)
	require.True(t, m.busy)
	require.NotNil(t, cmd)
	cmd()
	require.Equal(t, contracts.Intent{Type: contracts.IntentSwitch, Target: "$1:@2"}, got)
}

func TestDirectActionKeysKeepTheirTypedIntents(t *testing.T) {
	tests := []struct {
		name       string
		key        tea.KeyPressMsg
		selectedID contracts.NodeID
		value      string
		want       contracts.Intent
	}{
		{name: "switch", key: specialKey(tea.KeyEnter), selectedID: "window:@2", want: contracts.Intent{Type: contracts.IntentSwitch, Target: "$1:@2"}},
		{name: "refresh", key: printableKey("r"), selectedID: "window:@2", want: contracts.Intent{Type: contracts.IntentRefresh}},
		{name: "create session", key: printableKey("s"), selectedID: "session:$1", value: "test", want: contracts.Intent{Type: contracts.IntentCreateSession, Name: "test"}},
		{name: "create window", key: printableKey("a"), selectedID: "window:@2", value: "logs", want: contracts.Intent{Type: contracts.IntentCreateWindow, Session: "$1", Name: "logs"}},
		{name: "rename window", key: printableKey("e"), selectedID: "window:@2", value: "api", want: contracts.Intent{Type: contracts.IntentRenameWindow, NodeID: "window:@2", Target: "$1:@2", Name: "api"}},
		{name: "delete window", key: printableKey("x"), selectedID: "window:@2", want: contracts.Intent{Type: contracts.IntentDeleteWindow, ParentNodeID: "session:$1", Target: "$1:@2"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var got contracts.Intent
			m := newModel(interactionSnapshot(), func(intent contracts.Intent) (contracts.ActionResult, error) {
				got = intent
				return contracts.ActionResult{}, nil
			})
			m.mode = modeBrowse
			m = selectNode(t, m, test.selectedID)

			m, cmd := updateModel(t, m, test.key)
			switch m.mode {
			case modeInput:
				m.input.Value = test.value
				m, cmd = updateModel(t, m, specialKey(tea.KeyEnter))
			case modeConfirm:
				m, cmd = updateModel(t, m, printableKey("y"))
			}
			require.True(t, m.busy)
			require.NotNil(t, cmd)
			cmd()
			require.Equal(t, test.want, got)
		})
	}
}

func TestSelectionUsesStableIDAcrossFilterAndExpansionChanges(t *testing.T) {
	m := newModel(interactionSnapshot(), nil)
	m.mode = modeBrowse
	m = selectNode(t, m, "window:@2")

	m, _ = updateModel(t, m, printableKey("/"))
	m, _ = updateModel(t, m, printableKey("server"))
	require.Equal(t, "window:@2", m.selectedNodeID())
	m, _ = updateModel(t, m, controlKey('l'))
	require.Equal(t, "window:@2", m.selectedNodeID(), "clearing a filter must not select the row at the old index")

	m = selectNode(t, m, "session:$2")
	m, _ = updateModel(t, m, printableKey("H"))
	require.Equal(t, "session:$2", m.selectedNodeID())
	m, _ = updateModel(t, m, printableKey("L"))
	require.Equal(t, "session:$2", m.selectedNodeID(), "expansion must follow node identity, not row index")
}

func TestSelectionUsesStableIDAcrossRefreshAndMutations(t *testing.T) {
	t.Run("refresh", func(t *testing.T) {
		m := newModel(interactionSnapshot(), nil)
		m = selectNode(t, m, "window:@2")
		next := interactionSnapshot()
		next.Nodes[0], next.Nodes[1] = next.Nodes[1], next.Nodes[0]

		m, _ = updateModel(t, m, actionResultMsg{result: contracts.ActionResult{Snapshot: &next}})
		require.Equal(t, "window:@2", m.selectedNodeID())
	})

	t.Run("rename", func(t *testing.T) {
		m := newModel(interactionSnapshot(), nil)
		m = selectNode(t, m, "window:@2")
		m.pending = cloneIntent(contracts.Intent{Type: contracts.IntentRenameWindow, NodeID: "window:@2", Target: "$1:@2", Name: "api"})
		next := interactionSnapshot()
		next.Nodes[0].Children[1].Label = "1 api"
		next.Nodes[0].Children[1].Name = "api"

		m, _ = updateModel(t, m, actionResultMsg{result: contracts.ActionResult{Snapshot: &next}})
		require.Equal(t, "window:@2", m.selectedNodeID())
	})

	t.Run("create and delete window", func(t *testing.T) {
		m := newModel(interactionSnapshot(), nil)
		m.pending = cloneIntent(contracts.Intent{Type: contracts.IntentCreateWindow, Session: "$1", Name: "logs"})
		next := interactionSnapshot()
		next.Nodes[0].Children = append(next.Nodes[0].Children, contracts.Node{ID: "window:@4", ParentID: "session:$1", Kind: contracts.NodeKindWindow, Label: "2 logs", Name: "logs", Target: "$1:@4"})

		m, _ = updateModel(t, m, actionResultMsg{result: contracts.ActionResult{Snapshot: &next}})
		require.Equal(t, "window:@4", m.selectedNodeID())

		m.pending = cloneIntent(contracts.Intent{Type: contracts.IntentDeleteWindow, ParentNodeID: "session:$1", Target: "$1:@4"})
		final := interactionSnapshot()
		m, _ = updateModel(t, m, actionResultMsg{result: contracts.ActionResult{Snapshot: &final}})
		require.Equal(t, "session:$1", m.selectedNodeID())
	})
}
