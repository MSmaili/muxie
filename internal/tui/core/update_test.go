package core

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/MSmaili/hetki/internal/tui/list"
	"github.com/stretchr/testify/require"
)

func interactionSnapshot() list.Snapshot {
	return list.Snapshot{ActiveItemID: "window-1", Items: []list.Item{
		{
			ID: "session-1", Primary: "dev", SearchFields: []list.SearchField{{Tier: list.SearchPrimary, Text: "dev"}},
			Children: []list.Item{
				{ID: "window-1", Primary: "editor", Secondary: "~/code/editor", SearchFields: []list.SearchField{{Tier: list.SearchPrimary, Text: "editor"}, {Tier: list.SearchSecondary, Text: "~/code/editor"}}},
				{ID: "window-2", Primary: "server", Secondary: "~/svc/api", SearchFields: []list.SearchField{{Tier: list.SearchPrimary, Text: "server"}, {Tier: list.SearchSecondary, Text: "~/svc/api"}}},
			},
		},
		{
			ID: "session-2", Primary: "prod", SearchFields: []list.SearchField{{Tier: list.SearchPrimary, Text: "prod"}},
			Children: []list.Item{{ID: "window-3", Primary: "shell", SearchFields: []list.SearchField{{Tier: list.SearchPrimary, Text: "shell"}}}},
		},
	}}
}

func printableKey(text string) tea.KeyPressMsg {
	runes := []rune(text)
	return tea.KeyPressMsg{Code: runes[0], Text: text}
}

func specialKey(code rune) tea.KeyPressMsg { return tea.KeyPressMsg{Code: code} }

func controlKey(code rune) tea.KeyPressMsg { return tea.KeyPressMsg{Code: code, Mod: tea.ModCtrl} }

func updateModel(t *testing.T, m model, msg tea.Msg) (model, tea.Cmd) {
	t.Helper()
	updated, cmd := m.Update(msg)
	result, ok := updated.(model)
	require.True(t, ok, "Update returned %T", updated)
	return result, cmd
}

func selectNode(t *testing.T, m model, id list.ItemID) model {
	t.Helper()
	require.True(t, m.items.Select(id), "item %q is not visible", id)
	return m
}

func selectedNodeID(m model) list.ItemID {
	if selected, ok := m.selectedRow(); ok {
		return selected.Item.ID
	}
	return ""
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
	require.Equal(t, "q", m.items.Query(), "filter must own printable shortcut keys")
	m, _ = updateModel(t, m, printableKey("?"))
	require.Equal(t, "q?", m.items.Query(), "removed help key must remain searchable text")
	m, _ = updateModel(t, m, specialKey(tea.KeyEscape))
	require.Equal(t, modeBrowse, m.mode)
	require.Equal(t, "q?", m.items.Query(), "escape must preserve the query")

	m.mode = modeInput
	m.input = inputState{Value: "name"}
	m, cmd = updateModel(t, m, printableKey("q"))
	require.Nil(t, cmd)
	require.Equal(t, "nameq", m.input.Value)
	m, _ = updateModel(t, m, specialKey(tea.KeyEscape))
	require.Equal(t, modeBrowse, m.mode)

	m.mode = modeConfirm
	m.confirm = confirmState{Request: ActionRequest{ActionID: ActionDelete, ItemID: "session-1"}}
	m, cmd = updateModel(t, m, printableKey("q"))
	require.Nil(t, cmd)
	require.Equal(t, modeConfirm, m.mode)
	m, _ = updateModel(t, m, printableKey("n"))
	require.Equal(t, modeBrowse, m.mode)

	m.busy = true
	m, cmd = updateModel(t, m, printableKey("q"))
	require.Nil(t, cmd)
	require.True(t, m.busy)
	_, cmd = updateModel(t, m, controlKey('c'))
	requireQuit(t, cmd)
}

func TestEnterOpensSelectedItemFromFilter(t *testing.T) {
	var got ActionRequest
	m := newModel(interactionSnapshot(), func(request ActionRequest) (ActionResult, error) {
		got = request
		return ActionResult{}, nil
	})
	m.items.SetQuery("server")
	require.Equal(t, list.ItemID("window-2"), selectedNodeID(m))

	m, cmd := updateModel(t, m, specialKey(tea.KeyEnter))
	require.Equal(t, modeBrowse, m.mode)
	require.True(t, m.busy)
	require.NotNil(t, cmd)
	cmd()
	require.Equal(t, ActionRequest{ActionID: ActionOpen, ItemID: "window-2"}, got)
}

func TestDirectActionKeysEmitStableActionAndItemIDs(t *testing.T) {
	for _, test := range []struct {
		name       string
		key        tea.KeyPressMsg
		selectedID list.ItemID
		want       ActionRequest
	}{
		{name: "open", key: specialKey(tea.KeyEnter), selectedID: "window-2", want: ActionRequest{ActionID: ActionOpen, ItemID: "window-2"}},
		{name: "refresh", key: printableKey("r"), selectedID: "window-2", want: ActionRequest{ActionID: ActionRefresh}},
		{name: "create session", key: printableKey("s"), selectedID: "session-1", want: ActionRequest{ActionID: ActionCreateSession}},
		{name: "create window", key: printableKey("a"), selectedID: "window-2", want: ActionRequest{ActionID: ActionCreateWindow, ItemID: "window-2"}},
		{name: "rename", key: printableKey("e"), selectedID: "window-2", want: ActionRequest{ActionID: ActionRename, ItemID: "window-2"}},
		{name: "delete", key: printableKey("x"), selectedID: "window-2", want: ActionRequest{ActionID: ActionDelete, ItemID: "window-2"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			var got ActionRequest
			m := newModel(interactionSnapshot(), func(request ActionRequest) (ActionResult, error) {
				got = request
				return ActionResult{}, nil
			})
			m.mode = modeBrowse
			m = selectNode(t, m, test.selectedID)
			m, cmd := updateModel(t, m, test.key)
			require.True(t, m.busy)
			require.NotNil(t, cmd)
			cmd()
			require.Equal(t, test.want, got)
		})
	}
}

func TestPromptAndConfirmationKeepTheOriginatingItemID(t *testing.T) {
	var requests []ActionRequest
	m := newModel(interactionSnapshot(), func(request ActionRequest) (ActionResult, error) {
		requests = append(requests, request)
		if request.ActionID == ActionRename && request.Value == nil {
			return ActionResult{Input: &InputPrompt{Title: "RENAME", Prompt: "Name"}}, nil
		}
		if request.ActionID == ActionDelete && !request.Confirmed {
			return ActionResult{Confirmation: &Confirmation{Title: "DELETE", Body: "Delete?"}}, nil
		}
		return ActionResult{}, nil
	})
	m.mode = modeBrowse
	m = selectNode(t, m, "window-2")

	m, cmd := updateModel(t, m, printableKey("e"))
	m, _ = updateModel(t, m, cmd())
	require.Equal(t, modeInput, m.mode)
	require.Equal(t, list.ItemID("window-2"), m.input.Request.ItemID)
	m.input.Value = "api"
	m, cmd = updateModel(t, m, specialKey(tea.KeyEnter))
	m, _ = updateModel(t, m, cmd())
	require.Equal(t, list.ItemID("window-2"), requests[len(requests)-1].ItemID)
	require.Equal(t, "api", *requests[len(requests)-1].Value)

	m, cmd = updateModel(t, m, printableKey("x"))
	m, _ = updateModel(t, m, cmd())
	require.Equal(t, modeConfirm, m.mode)
	require.Equal(t, list.ItemID("window-2"), m.confirm.Request.ItemID)
	m, cmd = updateModel(t, m, printableKey("y"))
	cmd()
	require.Equal(t, list.ItemID("window-2"), requests[len(requests)-1].ItemID)
	require.True(t, requests[len(requests)-1].Confirmed)
}

func TestInvalidRefreshRetainsSnapshotAndSelection(t *testing.T) {
	m := newModel(interactionSnapshot(), nil)
	m = selectNode(t, m, "window-2")
	before := m.items.Snapshot()
	invalid := interactionSnapshot()
	invalid.Items[1].ID = "session-1"

	m, _ = updateModel(t, m, actionResultMsg{result: ActionResult{Snapshot: &invalid}})
	require.Error(t, m.err)
	require.Equal(t, before, m.items.Snapshot())
	require.Equal(t, list.ItemID("window-2"), selectedNodeID(m))
}
