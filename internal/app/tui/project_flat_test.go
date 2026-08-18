package tui

import (
	"fmt"
	"testing"

	"github.com/MSmaili/hetki/internal/backend"
	"github.com/MSmaili/hetki/internal/tui/list"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProjectFlatBuildsStablePathDestinations(t *testing.T) {
	state := backend.StateResult{
		Sessions: []backend.Session{{
			ID: "$1", Name: "dev", Windows: []backend.Window{
				{ID: "@1", Name: "editor", Index: 1, Active: true, Panes: []backend.Pane{
					{ID: "%5", Index: 5, Path: "/home/me/code"},
					{ID: "%7", Index: 7, Path: "/home/me/code", Active: true},
					{ID: "%2", Index: 2, Path: "/var/log"},
				}},
				{ID: "@2", Name: "tests", Index: 2, Panes: []backend.Pane{{ID: "%8", Index: 0, Path: "/home/me/code", Active: true}}},
			},
		}},
		Active: backend.ActiveContext{PaneID: "%7"},
	}

	snapshot, index, err := projectFlat(state, "/home/me")
	require.NoError(t, err)
	require.Len(t, snapshot.Items, 3)

	codeID := destinationItemID("$1", "@1", "/home/me/code")
	code := index[codeID]
	assert.Equal(t, "%7", code.Target, "the active pane owns a collapsed path destination")
	assert.Equal(t, "$1:@1", code.MutationTarget)
	assert.Equal(t, "~/code", itemByID(snapshot.Items, codeID).Secondary)
	assert.Equal(t, codeID, snapshot.ActiveItemID)
	assert.NotEqual(t, codeID, destinationItemID("$1", "@2", "/home/me/code"))

	fields := itemByID(snapshot.Items, codeID).SearchFields
	assert.Equal(t, []list.SearchField{
		{Tier: list.SearchPrimary, Text: "dev"},
		{Tier: list.SearchPrimary, Text: "editor"},
		{Tier: list.SearchSecondary, Text: "/home/me/code"},
		{Tier: list.SearchSecondary, Text: "~/code"},
	}, fields)
}

func TestProjectFlatKeepsDuplicateAndDelimitedContextsDistinct(t *testing.T) {
	state := backend.StateResult{Sessions: []backend.Session{
		{ID: "$1", Name: "same > name\n", Windows: []backend.Window{
			{ID: "@1", Name: "same:window", Panes: []backend.Pane{{ID: "%1", Path: "/work/a:b\tctx"}}},
			{ID: "@3", Name: "same:window", Panes: []backend.Pane{{ID: "%3", Path: "/work/a:b\tctx"}}},
		}},
		{ID: "$2", Name: "same > name\n", Windows: []backend.Window{{ID: "@2", Name: "same:window", Panes: []backend.Pane{{ID: "%2", Path: "/work/a:b\tctx"}}}}},
	}}

	snapshot, index, err := projectFlat(state, "")
	require.NoError(t, err)
	require.Len(t, snapshot.Items, 3)
	assert.Equal(t, "%1", index[destinationItemID("$1", "@1", "/work/a:b\tctx")].Target)
	assert.Equal(t, "%3", index[destinationItemID("$1", "@3", "/work/a:b\tctx")].Target)
	assert.Equal(t, "%2", index[destinationItemID("$2", "@2", "/work/a:b\tctx")].Target)
}

func TestProjectFlatAllowsWindowsWithoutPanes(t *testing.T) {
	state := backend.StateResult{Sessions: []backend.Session{{
		ID: "$1", Name: "empty", Windows: []backend.Window{{ID: "@1", Name: "empty"}},
	}}}

	snapshot, index, err := projectFlat(state, "")
	require.NoError(t, err)
	assert.Empty(t, snapshot.Items)
	assert.Empty(t, index)
}

func TestProjectFlatUsesLowestPaneIndexWithoutAnActivePane(t *testing.T) {
	state := backend.StateResult{Sessions: []backend.Session{{
		ID: "$1", Name: "dev", Windows: []backend.Window{{
			ID: "@1", Name: "editor", Panes: []backend.Pane{
				{ID: "%9", Index: 9, Path: "/code"},
				{ID: "%2", Index: 2, Path: "/code"},
			},
		}},
	}}}

	_, index, err := projectFlat(state, "")
	require.NoError(t, err)
	assert.Equal(t, "%2", index[destinationItemID("$1", "@1", "/code")].Target)
}

func TestProjectFlatKeepsRawPathsAsIdentity(t *testing.T) {
	state := backend.StateResult{Sessions: []backend.Session{{
		ID: "$1", Name: "dev", Windows: []backend.Window{{
			ID: "@1", Name: "editor", Panes: []backend.Pane{
				{ID: "%1", Index: 0, Path: "/home/me/code"},
				{ID: "%2", Index: 1, Path: "~/code"},
			},
		}},
	}}}

	snapshot, _, err := projectFlat(state, "/home/me")
	require.NoError(t, err)
	require.Len(t, snapshot.Items, 2)
	assert.Equal(t, snapshot.Items[0].Secondary, snapshot.Items[1].Secondary)
	assert.NotEqual(t, snapshot.Items[0].ID, snapshot.Items[1].ID)
}

func TestFlatProjectionSearchesSessionWindowAndPathWithNamePriority(t *testing.T) {
	state := backend.StateResult{Sessions: []backend.Session{
		{ID: "$1", Name: "needle", Windows: []backend.Window{{ID: "@1", Name: "editor", Panes: []backend.Pane{{ID: "%1", Path: "/work/code"}}}}},
		{ID: "$2", Name: "other", Windows: []backend.Window{{ID: "@2", Name: "logs", Panes: []backend.Pane{{ID: "%2", Path: "/work/needle"}}}}},
	}}
	snapshot, _, err := projectFlat(state, "")
	require.NoError(t, err)
	model, err := list.New(snapshot)
	require.NoError(t, err)

	model.SetQuery("needle")
	require.Len(t, model.Rows(), 2)
	assert.Equal(t, destinationItemID("$1", "@1", "/work/code"), model.Rows()[0].Item.ID)
	model.SetQuery("editor")
	require.Len(t, model.Rows(), 1)
	model.SetQuery("work/needle")
	require.Len(t, model.Rows(), 1)
	assert.Equal(t, destinationItemID("$2", "@2", "/work/needle"), model.Rows()[0].Item.ID)
}

func TestProjectFlatRejectsUnstablePaneIDs(t *testing.T) {
	state := backend.StateResult{Sessions: []backend.Session{{
		ID: "$1", Name: "dev", Windows: []backend.Window{{
			ID: "@1", Name: "editor", Panes: []backend.Pane{{ID: "editor.0", Path: "/code"}},
		}},
	}}}

	_, _, err := projectFlat(state, "")
	require.ErrorContains(t, err, "pane must have a stable %N ID")
}

func itemByID(items []list.Item, id list.ItemID) list.Item {
	for _, item := range items {
		if item.ID == id {
			return item
		}
	}
	return list.Item{}
}

func BenchmarkProjectFlatTenThousandPanes(b *testing.B) {
	state := backend.StateResult{}
	windowID, paneID := 0, 0
	for sessionIndex := range 100 {
		session := backend.Session{ID: fmt.Sprintf("$%d", sessionIndex), Name: fmt.Sprintf("session-%d", sessionIndex)}
		for windowIndex := range 20 {
			windowID++
			window := backend.Window{ID: fmt.Sprintf("@%d", windowID), Name: fmt.Sprintf("window-%d", windowIndex), Index: windowIndex}
			for paneIndex := range 5 {
				paneID++
				window.Panes = append(window.Panes, backend.Pane{
					ID: fmt.Sprintf("%%%d", paneID), Index: paneIndex, Path: fmt.Sprintf("/work/%d/%d/%d", sessionIndex, windowIndex, paneIndex),
				})
			}
			session.Windows = append(session.Windows, window)
		}
		state.Sessions = append(state.Sessions, session)
	}

	b.ReportAllocs()
	for b.Loop() {
		_, _, err := projectFlat(state, "/home/me")
		if err != nil {
			b.Fatal(err)
		}
	}
}
