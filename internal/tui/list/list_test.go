package list

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func item(id, primary, secondary string) Item {
	fields := []SearchField{{Tier: SearchPrimary, Text: primary}}
	if secondary != "" {
		fields = append(fields, SearchField{Tier: SearchSecondary, Text: secondary})
	}
	return Item{ID: ItemID(id), Primary: primary, Secondary: secondary, SearchFields: fields}
}

func flatFixture() Snapshot {
	return Snapshot{Items: []Item{
		item("editor", "editor", "~/code/editor"),
		item("server", "server", "~/svc/api"),
		item("shell", "shell", "~/prod/app"),
	}, ActiveItemID: "editor"}
}

func nestedFixture() Snapshot {
	dev := item("dev", "dev", "")
	dev.Children = []Item{item("editor", "editor", "~/code/editor"), item("server", "server", "~/svc/api")}
	prod := item("prod", "prod", "")
	prod.Children = []Item{item("shell", "shell", "~/prod/app")}
	return Snapshot{Items: []Item{dev, prod}, ActiveItemID: "editor"}
}

func selectedID(m Model) ItemID {
	selected, ok := m.Selected()
	if !ok {
		return ""
	}
	return selected.Item.ID
}

func rowIDs(rows []Row) []ItemID {
	ids := make([]ItemID, len(rows))
	for i, row := range rows {
		ids[i] = row.Item.ID
	}
	return ids
}

func TestFlatAndNestedListsShareMovementFilterAndViewport(t *testing.T) {
	for _, test := range []struct {
		name     string
		snapshot Snapshot
		wantRows []ItemID
	}{
		{name: "flat", snapshot: flatFixture(), wantRows: []ItemID{"editor", "server", "shell"}},
		{name: "nested", snapshot: nestedFixture(), wantRows: []ItemID{"dev", "editor", "server", "prod", "shell"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			m, err := New(test.snapshot)
			require.NoError(t, err)
			require.Equal(t, test.wantRows, rowIDs(m.Rows()))

			m.Resize(2)
			m.Bottom()
			require.Equal(t, test.wantRows[len(test.wantRows)-1], selectedID(m))
			require.Len(t, m.VisibleRows(), 2)
			require.Greater(t, m.Offset(), 0)
			m.Top()
			require.Equal(t, test.wantRows[0], selectedID(m))
			require.Zero(t, m.Offset())

			m.SetQuery("api")
			require.Equal(t, ItemID("server"), selectedID(m))
			require.Contains(t, rowIDs(m.Rows()), ItemID("server"))
			if test.name == "nested" {
				require.Contains(t, rowIDs(m.Rows()), ItemID("dev"), "a matching child must retain its ancestor")
			}
			m.SetQuery("")
			require.Equal(t, ItemID("server"), selectedID(m))
			m.SetQuery("no-such-item")
			require.Empty(t, m.Rows())
		})
	}
}

func TestPrimarySearchTierOutranksSecondaryWithoutReorderingRows(t *testing.T) {
	snapshot := Snapshot{Items: []Item{
		item("path", "shell", "~/svc/api"),
		item("name", "api", "~/other"),
	}}
	m, err := New(snapshot)
	require.NoError(t, err)
	m.SetQuery("api")

	require.Equal(t, []ItemID{"name", "path"}, rowIDs(m.Rows()), "stronger fuzzy matches must sort first")
	require.Greater(t, m.Rows()[0].Score, m.Rows()[1].Score, "primary matches must outrank secondary matches")
}

func TestNestedFilteringRanksPrimaryTierBeforeSecondaryAndKeepsAncestors(t *testing.T) {
	secondary := item("secondary-root", "first", "")
	secondary.Children = []Item{item("secondary", "shell", "~/svc/api")}
	primary := item("primary-root", "second", "")
	primary.Children = []Item{item("primary", "api", "~/other")}
	m, err := New(Snapshot{Items: []Item{secondary, primary}})
	require.NoError(t, err)
	m.SetQuery("api")

	require.Equal(t, []ItemID{"primary-root", "primary", "secondary-root", "secondary"}, rowIDs(m.Rows()))
	require.Equal(t, 0, m.Rows()[0].Score, "retained ancestors are not direct matches")
	require.Greater(t, m.Rows()[1].Score, m.Rows()[3].Score)
}

func TestEqualFilterScoresPreserveIncomingOrder(t *testing.T) {
	m, err := New(Snapshot{Items: []Item{item("first", "api", ""), item("second", "api", "")}})
	require.NoError(t, err)
	m.SetQuery("api")
	require.Equal(t, []ItemID{"first", "second"}, rowIDs(m.Rows()))
}

func TestActiveItemMarksItsVisibleAncestors(t *testing.T) {
	m, err := New(nestedFixture())
	require.NoError(t, err)
	require.True(t, m.IsActive("dev"))
	require.True(t, m.IsActive("editor"))
	require.False(t, m.IsActive("server"))
	require.False(t, m.IsActive("prod"))
}

func TestExpansionAndReplacementPreserveStableSelection(t *testing.T) {
	m, err := New(nestedFixture())
	require.NoError(t, err)
	require.True(t, m.Select("prod"))
	require.True(t, m.CollapseAll())
	require.Equal(t, ItemID("prod"), selectedID(m))
	require.True(t, m.ExpandAll())
	require.Equal(t, ItemID("prod"), selectedID(m))

	m.expanded["removed"] = true
	reordered := nestedFixture()
	reordered.Items[0], reordered.Items[1] = reordered.Items[1], reordered.Items[0]
	require.NoError(t, m.Replace(reordered, ""))
	require.Equal(t, ItemID("prod"), selectedID(m))
	require.NotContains(t, m.expanded, ItemID("removed"))
}

func TestInvalidReplacementRetainsAllListState(t *testing.T) {
	m, err := New(nestedFixture())
	require.NoError(t, err)
	m.Resize(2)
	require.True(t, m.Select("server"))
	m.SetQuery("server")
	beforeRows := rowIDs(m.Rows())
	beforeOffset := m.Offset()

	invalid := nestedFixture()
	invalid.Items[1].ID = "dev"
	require.Error(t, m.Replace(invalid, ""))
	require.Equal(t, beforeRows, rowIDs(m.Rows()))
	require.Equal(t, beforeOffset, m.Offset())
	require.Equal(t, ItemID("server"), selectedID(m))
	require.Equal(t, "server", m.Query())
}

func TestEmptyListUsesTheSameSafePipeline(t *testing.T) {
	m, err := New(Snapshot{})
	require.NoError(t, err)
	m.Resize(3)
	m.Move(1)
	m.Page(1)
	m.SetQuery("missing")
	require.Empty(t, m.Rows())
	require.Empty(t, m.VisibleRows())
	_, ok := m.Selected()
	require.False(t, ok)
	require.Zero(t, m.Offset())
}

func TestMatchNavigationUsesOnlyMatchingRows(t *testing.T) {
	snapshot := Snapshot{Items: []Item{
		item("first", "api one", ""),
		item("second", "not a match", ""),
		item("third", "api two", ""),
	}}
	m, err := New(snapshot)
	require.NoError(t, err)
	m.SetQuery("api")
	require.Equal(t, ItemID("first"), selectedID(m))
	require.True(t, m.JumpMatch(true))
	require.Equal(t, ItemID("third"), selectedID(m))
	current, total := m.MatchPosition()
	require.Equal(t, 2, current)
	require.Equal(t, 2, total)
}
