package tui

import (
	"testing"

	"github.com/MSmaili/hetki/internal/tui/list"
	"github.com/stretchr/testify/require"
)

func TestValidateProjectionRequiresExactOwnerIndexMembership(t *testing.T) {
	snapshot := list.Snapshot{Items: []list.Item{{
		ID: "item", Primary: "item", SearchFields: []list.SearchField{{Tier: list.SearchPrimary, Text: "item"}},
	}}}
	require.ErrorContains(t, validateProjection(snapshot, nil), "index has 0")
	require.ErrorContains(t, validateProjection(snapshot, itemIndex{"other": {ID: "other"}}), "missing from owner index")
	require.NoError(t, validateProjection(snapshot, itemIndex{"item": {ID: "item"}}))
}
