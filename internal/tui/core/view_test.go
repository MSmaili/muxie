package core

import (
	"testing"

	"github.com/MSmaili/hetki/internal/tui/list"
	"github.com/stretchr/testify/require"
)

func TestHeaderShowsNonFatalSnapshotNotice(t *testing.T) {
	m := newModel(list.Snapshot{Notice: "frecency state is corrupt"}, nil)
	require.Equal(t, "frecency state is corrupt", headerRight(m))
	m.busy = true
	m.status = "refreshing..."
	require.Equal(t, "refreshing...", headerRight(m))
}

func TestHeaderTracksMode(t *testing.T) {
	m := model{}
	for _, tt := range []struct {
		mode       uiMode
		wantPrompt string
		wantHint   string
	}{
		{mode: modeBrowse, wantPrompt: " NORMAL ", wantHint: "press / to filter or ; to jump"},
		{mode: modeFilter, wantPrompt: " FILTER "},
		{mode: modeJump, wantPrompt: " JUMP ", wantHint: "type a label to jump or / to filter"},
		{mode: modeMenu},
	} {
		m.mode = tt.mode
		require.Equal(t, tt.wantPrompt, headerPrompt(m))
		require.Equal(t, tt.wantHint, headerHint(m))
	}

	m = newModel(viewSnapshot(), nil)
	m.mode = modeFilter
	m.items.SetQuery("dev")
	require.Empty(t, headerHint(m))
}

func TestRootCountLabelShowsFilteredRootsOverTotal(t *testing.T) {
	snapshot := list.Snapshot{Items: []list.Item{
		{
			ID: "session-1", Primary: "dev", SearchFields: []list.SearchField{{Tier: list.SearchPrimary, Text: "dev"}},
			Children: []list.Item{{ID: "window-1", Primary: "editor", SearchFields: []list.SearchField{{Tier: list.SearchPrimary, Text: "editor"}}}},
		},
		{
			ID: "session-2", Primary: "prod", SearchFields: []list.SearchField{{Tier: list.SearchPrimary, Text: "prod"}},
			Children: []list.Item{{ID: "window-2", Primary: "logs", SearchFields: []list.SearchField{{Tier: list.SearchPrimary, Text: "logs"}}}},
		},
	}}
	model, err := list.New(snapshot)
	require.NoError(t, err)
	require.Equal(t, "2", rootCountLabel(snapshot, model.ShownRoots(), model.Query()))

	model.SetQuery("logs")
	require.Equal(t, "1/2", rootCountLabel(snapshot, model.ShownRoots(), model.Query()))
	model.SetQuery("missing")
	require.Equal(t, "0/2", rootCountLabel(snapshot, model.ShownRoots(), model.Query()))
}
