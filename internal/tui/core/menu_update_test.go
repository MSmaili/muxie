package core

import (
	"errors"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/MSmaili/hetki/internal/tui/list"
	"github.com/stretchr/testify/require"
)

func testItemMenu() ItemMenu {
	return ItemMenu{Title: "WINDOW ACTIONS", Entries: []MenuEntry{
		{Action: ActionOpen, Label: "Open window", Activation: 'o'},
		{Action: ActionRename, Label: "Rename window", Activation: 'r'},
		{Action: ActionDelete, Label: "Delete window", Activation: 'd'},
	}}
}

func openTestMenu(t *testing.T, m model) model {
	t.Helper()
	m, cmd := updateModel(t, m, controlKey('k'))
	require.True(t, m.busy)
	require.NotNil(t, cmd)
	m, _ = updateModel(t, m, cmd())
	require.Equal(t, modeMenu, m.mode)
	return m
}

func TestContextMenuCancelRestoresBrowseAndFilterState(t *testing.T) {
	for _, origin := range []uiMode{modeBrowse, modeFilter} {
		t.Run(string(origin), func(t *testing.T) {
			var got ActionRequest
			m := newModel(interactionSnapshot(), func(request ActionRequest) (ActionResult, error) {
				got = request
				return ActionResult{Menu: menuPtr(testItemMenu())}, nil
			})
			m = browseModel(m)
			m.items.SetQuery("server")
			m.height = 5
			m = m.reflow()
			require.True(t, m.items.Select("window-2"))
			m.mode = origin
			query, cursor, offset, selected := m.items.Query(), m.items.Cursor(), m.items.Offset(), selectedNodeID(m)

			m = openTestMenu(t, m)
			require.Equal(t, ActionRequest{ActionID: ActionContextMenu, ItemID: "window-2"}, got)
			require.Equal(t, list.ItemID("window-2"), m.menu.ItemID)
			m, cmd := updateModel(t, m, printableKey("q"))
			require.Nil(t, cmd, "menu must own printable keys")
			require.Equal(t, modeMenu, m.mode)

			m, _ = updateModel(t, m, specialKey(tea.KeyEscape))
			require.Equal(t, origin, m.mode)
			require.Equal(t, query, m.items.Query())
			require.Equal(t, cursor, m.items.Cursor())
			require.Equal(t, offset, m.items.Offset())
			require.Equal(t, selected, selectedNodeID(m))
		})
	}
}

func TestMenuLettersAndArrowEnterUseTheDirectActionRequest(t *testing.T) {
	direct := make(chan ActionRequest, 1)
	directModel := newModel(interactionSnapshot(), func(request ActionRequest) (ActionResult, error) {
		direct <- request
		return ActionResult{}, nil
	})
	directModel = selectNode(t, browseModel(directModel), "window-2")
	_, directCmd := updateModel(t, directModel, printableKey("r"))
	require.NotNil(t, directCmd)
	directCmd()
	want := <-direct

	for _, test := range []struct {
		name string
		act  func(*testing.T, model) (model, tea.Cmd)
	}{
		{name: "letter", act: func(t *testing.T, m model) (model, tea.Cmd) {
			return updateModel(t, m, printableKey("R"))
		}},
		{name: "arrow and enter", act: func(t *testing.T, m model) (model, tea.Cmd) {
			m, _ = updateModel(t, m, specialKey(tea.KeyDown))
			return updateModel(t, m, specialKey(tea.KeyEnter))
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			requests := make(chan ActionRequest, 2)
			m := newModel(interactionSnapshot(), func(request ActionRequest) (ActionResult, error) {
				requests <- request
				if request.ActionID == ActionContextMenu {
					return ActionResult{Menu: menuPtr(testItemMenu())}, nil
				}
				return ActionResult{}, nil
			})
			m = selectNode(t, browseModel(m), "window-2")
			m = openTestMenu(t, m)
			<-requests

			_, cmd := test.act(t, m)
			require.NotNil(t, cmd)
			cmd()
			require.Equal(t, want, <-requests)
		})
	}
}

func TestEveryMenuEntryDispatchesFromTheStableOrigin(t *testing.T) {
	for _, entry := range []MenuEntry{
		{Action: ActionOpen, Label: "Open", Activation: 'o'},
		{Action: ActionCreateSession, Label: "New session", Activation: 's'},
		{Action: ActionCreateWindow, Label: "New window", Activation: 'w'},
		{Action: ActionRename, Label: "Rename", Activation: 'r'},
		{Action: ActionDelete, Label: "Delete", Activation: 'd'},
		{Action: ActionRenameSession, Label: "Rename session", Activation: 'n'},
		{Action: ActionDeleteSession, Label: "Delete session", Activation: 'x'},
		{Action: ActionRefresh, Label: "Refresh", Activation: 'f'},
		{Action: ActionToggleProjection, Label: "Toggle", Activation: 't'},
	} {
		t.Run(string(entry.Action), func(t *testing.T) {
			var got ActionRequest
			m := newModel(interactionSnapshot(), func(request ActionRequest) (ActionResult, error) {
				if request.ActionID == ActionContextMenu {
					return ActionResult{Menu: menuPtr(ItemMenu{Entries: []MenuEntry{entry}})}, nil
				}
				got = request
				return ActionResult{}, nil
			})
			m = selectNode(t, browseModel(m), "window-2")
			m = openTestMenu(t, m)
			_, cmd := updateModel(t, m, printableKey(string(entry.Activation)))
			require.NotNil(t, cmd)
			cmd()
			require.Equal(t, ActionRequest{ActionID: entry.Action, ItemID: "window-2"}, got)
		})
	}
}

func TestMenuInputAndConfirmationHandoffsRestoreFilterState(t *testing.T) {
	failed := errors.New("rename failed")
	m := newModel(interactionSnapshot(), func(request ActionRequest) (ActionResult, error) {
		switch request.ActionID {
		case ActionContextMenu:
			return ActionResult{Menu: menuPtr(testItemMenu())}, nil
		case ActionRename:
			if request.Value == nil {
				return ActionResult{Input: &InputPrompt{Title: "RENAME", Prompt: "Name", InitialValue: "editor"}}, nil
			}
			return ActionResult{}, failed
		case ActionDelete:
			return ActionResult{Confirmation: &Confirmation{Title: "DELETE", Body: "Delete?"}}, nil
		default:
			return ActionResult{}, nil
		}
	})
	m = browseModel(m)
	m.items.SetQuery("server")
	m.height = 5
	m = m.reflow()
	require.True(t, m.items.Select("window-2"))
	m.mode = modeFilter
	query, cursor, offset, selected := m.items.Query(), m.items.Cursor(), m.items.Offset(), selectedNodeID(m)
	requireState := func(t *testing.T, m model) {
		t.Helper()
		require.Equal(t, modeFilter, m.mode)
		require.Equal(t, query, m.items.Query())
		require.Equal(t, cursor, m.items.Cursor())
		require.Equal(t, offset, m.items.Offset())
		require.Equal(t, selected, selectedNodeID(m))
	}

	m = openTestMenu(t, m)
	m, cmd := updateModel(t, m, printableKey("r"))
	m, _ = updateModel(t, m, cmd())
	require.Equal(t, modeInput, m.mode)
	m, _ = updateModel(t, m, specialKey(tea.KeyEscape))
	requireState(t, m)

	m = openTestMenu(t, m)
	m, cmd = updateModel(t, m, printableKey("d"))
	m, _ = updateModel(t, m, cmd())
	require.Equal(t, modeConfirm, m.mode)
	m, _ = updateModel(t, m, specialKey(tea.KeyEscape))
	requireState(t, m)

	m = openTestMenu(t, m)
	m, cmd = updateModel(t, m, printableKey("r"))
	m, _ = updateModel(t, m, cmd())
	m, cmd = updateModel(t, m, specialKey(tea.KeyEnter))
	require.True(t, m.busy)
	m, _ = updateModel(t, m, cmd())
	require.ErrorIs(t, m.err, failed)
	requireState(t, m)
}

func TestDeleteResultSelectsParentThenNextOrPreviousSurvivor(t *testing.T) {
	t.Run("filtered tree window parent", func(t *testing.T) {
		m := browseModel(newModel(interactionSnapshot(), nil))
		m.items.SetQuery("editor")
		m = selectNode(t, m, "window-1")
		m.mode = modeFilter
		m.busy = true
		m.pending = cloneRequest(ActionRequest{ActionID: ActionDelete, ItemID: "window-1", Confirmed: true})
		m.pendingRows = currentRowIDs(m)
		next := interactionSnapshot()
		next.Items[0].Children = next.Items[0].Children[1:]
		next.Items[1].Children[0].Primary = "editor backup"
		next.Items[1].Children[0].SearchFields = []list.SearchField{{Tier: list.SearchPrimary, Text: "editor backup"}}
		next.ActiveItemID = "session-1"
		m, _ = updateModel(t, m, actionResultMsg{result: ActionResult{Snapshot: &next, SelectItemID: "session-1"}})
		require.Equal(t, list.ItemID("session-1"), selectedNodeID(m))
		require.Empty(t, m.items.Query(), "the parent must be revealed when the preserved filter hides it")
	})

	flat := func(ids ...list.ItemID) list.Snapshot {
		items := make([]list.Item, len(ids))
		for i, id := range ids {
			items[i] = list.Item{ID: id, Primary: string(id), SearchFields: []list.SearchField{{Tier: list.SearchPrimary, Text: "place"}}}
		}
		return list.Snapshot{Items: items}
	}
	for _, test := range []struct {
		name     string
		action   ActionID
		before   list.Snapshot
		selectID list.ItemID
		after    list.Snapshot
		want     list.ItemID
	}{
		{name: "owning window chooses next", action: ActionDelete, before: flat("one", "two", "three"), selectID: "two", after: flat("one", "three"), want: "three"},
		{name: "owning session skips removed destinations", action: ActionDeleteSession, before: flat("one", "sibling", "selected", "next", "last"), selectID: "selected", after: flat("one", "next", "last"), want: "next"},
		{name: "owning session chooses previous at end", action: ActionDeleteSession, before: flat("one", "two", "three"), selectID: "three", after: flat("one", "two"), want: "two"},
	} {
		t.Run(test.name, func(t *testing.T) {
			m := browseModel(newModel(test.before, nil))
			m.items.SetQuery("place")
			m = selectNode(t, m, test.selectID)
			m.mode = modeFilter
			m.busy = true
			m.pending = cloneRequest(ActionRequest{ActionID: test.action, ItemID: test.selectID, Confirmed: true})
			m.pendingRows = currentRowIDs(m)
			m, _ = updateModel(t, m, actionResultMsg{result: ActionResult{Snapshot: &test.after}})
			require.Equal(t, test.want, selectedNodeID(m))
		})
	}
}

func TestContextMenuRequiresASelection(t *testing.T) {
	m := browseModel(newModel(list.Snapshot{}, nil))
	m, cmd := updateModel(t, m, controlKey('k'))
	require.Nil(t, cmd)
	require.False(t, m.busy)
	require.Equal(t, statusNoSelection, m.status)
}

func TestContextMenuRejectsEmptyAndInvalidEntries(t *testing.T) {
	for _, test := range []struct {
		name string
		menu ItemMenu
		want string
	}{
		{name: "empty", menu: ItemMenu{}, want: "no available actions"},
		{name: "duplicate action", menu: ItemMenu{Entries: []MenuEntry{{Action: ActionOpen, Label: "Open", Activation: 'o'}, {Action: ActionOpen, Label: "Again", Activation: 'a'}}}, want: "appears more than once"},
		{name: "duplicate activation", menu: ItemMenu{Entries: []MenuEntry{{Action: ActionOpen, Label: "Open", Activation: 'o'}, {Action: ActionRename, Label: "Rename", Activation: 'O'}}}, want: "appears more than once"},
		{name: "control activation", menu: ItemMenu{Entries: []MenuEntry{{Action: ActionOpen, Label: "Open", Activation: '\n'}}}, want: "invalid"},
	} {
		t.Run(test.name, func(t *testing.T) {
			m := newModel(interactionSnapshot(), func(ActionRequest) (ActionResult, error) {
				return ActionResult{Menu: menuPtr(test.menu)}, nil
			})
			m = selectNode(t, browseModel(m), "window-2")
			m, cmd := updateModel(t, m, controlKey('k'))
			m, _ = updateModel(t, m, cmd())
			require.Equal(t, modeBrowse, m.mode)
			require.ErrorContains(t, m.err, test.want)
		})
	}
}

func currentRowIDs(m model) []list.ItemID {
	rows := m.items.Rows()
	ids := make([]list.ItemID, len(rows))
	for i, row := range rows {
		ids[i] = row.Item.ID
	}
	return ids
}

func menuPtr(value ItemMenu) *ItemMenu { return &value }
