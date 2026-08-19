package core

import (
	"testing"

	"github.com/MSmaili/hetki/internal/tui/list"
	"github.com/stretchr/testify/require"
)

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
	require.Equal(t, "2/2", rootCountLabel(snapshot, model.Rows()))

	model.SetQuery("logs")
	require.Equal(t, "1/2", rootCountLabel(snapshot, model.Rows()))
	model.SetQuery("missing")
	require.Equal(t, "0/2", rootCountLabel(snapshot, model.Rows()))
}
