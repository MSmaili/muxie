package state

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompareSessionWindowsDetectsSemanticMismatch(t *testing.T) {
	desired := []*Window{{
		Name:   "editor",
		Path:   "~/code",
		Layout: "2419,80x24,0,0{40x24,0,0,0,39x24,41,0,1}",
		Panes: []*Pane{
			{Path: "~/code", Command: "vim"},
			{Path: "~/api", Command: "npm test", Zoom: true},
		},
	}}
	actual := []*Window{{
		Name:   "editor",
		Path:   "~/code",
		Layout: "18c3,80x24,0,0{40x24,0,0,0,39x24,41,0,1}",
		Panes: []*Pane{
			{Path: "~/code", Command: "vim"},
			{Path: "~/api", Command: "npm run dev"},
		},
	}}

	diff := compareSessionWindows(desired, actual)
	require.Len(t, diff.Mismatched, 1)
	assert.Equal(t, "2419,80x24,0,0{40x24,0,0,0,39x24,41,0,1}", diff.Mismatched[0].Desired.Layout)
	assert.Equal(t, "18c3,80x24,0,0{40x24,0,0,0,39x24,41,0,1}", diff.Mismatched[0].Actual.Layout)
	assert.Empty(t, diff.Missing)
	assert.Empty(t, diff.Extra)
}

func TestCompareSessionWindowsMatchesPanesByPosition(t *testing.T) {
	desired := []*Window{{
		Name: "editor",
		Path: "~/code",
		Panes: []*Pane{
			{Path: "~/code", Command: "vim"},
			{Path: "~/api", Command: "npm test"},
		},
	}}
	actual := []*Window{{
		Name: "editor",
		Path: "~/code",
		Panes: []*Pane{
			{Path: "~/api", Command: "npm test"},
			{Path: "~/code", Command: "vim"},
		},
	}}

	diff := compareSessionWindows(desired, actual)
	require.Len(t, diff.Mismatched, 1)
	assert.Empty(t, diff.Missing)
	assert.Empty(t, diff.Extra)
}

func TestCompareSessionWindowsTreatsWindowPathChangeAsReplacement(t *testing.T) {
	desired := []*Window{{Name: "editor", Path: "~/new", Panes: []*Pane{{Path: "~/new"}}}}
	actual := []*Window{{Name: "editor", Path: "~/old", Panes: []*Pane{{Path: "~/old"}}}}

	diff := compareSessionWindows(desired, actual)
	require.Len(t, diff.Missing, 1)
	require.Len(t, diff.Extra, 1)
	assert.Empty(t, diff.Mismatched)
	assert.Equal(t, "~/new", diff.Missing[0].Path)
	assert.Equal(t, "~/old", diff.Extra[0].Path)
}

func TestCompareSessionWindowsTreatsDuplicatesAsDeterministicMultiset(t *testing.T) {
	desired := []*Window{{Name: "editor", Path: "~/code", Panes: []*Pane{{Path: "~/code"}}}}
	actual := []*Window{
		{ID: "@1", Name: "editor", Path: "~/code", Panes: []*Pane{{Path: "~/different"}}},
		{ID: "@2", Name: "editor", Path: "~/scratch", Panes: []*Pane{{Path: "~/scratch"}}},
	}

	diff := compareSessionWindows(desired, actual)
	require.Len(t, diff.Mismatched, 1)
	assert.Equal(t, "@1", diff.Mismatched[0].Actual.ID)
	require.Len(t, diff.Extra, 1)
	assert.Equal(t, "@2", diff.Extra[0].ID)
}

func TestCompareSessionWindowsPrefersExactDuplicateMatch(t *testing.T) {
	desired := []*Window{{Name: "editor", Path: "~/code", Panes: []*Pane{{Path: "~/code"}}}}
	actual := []*Window{
		{ID: "@1", Name: "editor", Path: "~/code", Panes: []*Pane{{Path: "~/different"}}},
		{ID: "@2", Name: "editor", Path: "~/code", Panes: []*Pane{{Path: "~/code"}}},
	}

	diff := compareSessionWindows(desired, actual)
	assert.Empty(t, diff.Mismatched)
	require.Len(t, diff.Extra, 1)
	assert.Equal(t, "@1", diff.Extra[0].ID)
}

func TestWindowsMatchUsesBestEffortLayoutAndStrictPaneSemantics(t *testing.T) {
	base := &Window{
		Name:   "editor",
		Path:   "~/code",
		Layout: "tiled",
		Panes: []*Pane{
			{Path: "~/code", Command: "vim"},
			{Path: "~/api", Command: "npm test", Zoom: true},
		},
	}

	assert.True(t, windowsMatch(base, &Window{
		Name:   "editor",
		Path:   "~/code",
		Layout: "tiled",
		Panes: []*Pane{
			{Path: "~/code", Command: "vim"},
			{Path: "~/api", Command: "npm test", Zoom: true},
		},
	}))
	assert.True(t, windowsMatch(&Window{Name: "editor", Path: "~/code", Layout: "tiled", Panes: base.Panes}, &Window{Name: "editor", Path: "~/code", Layout: "2419,80x24,0,0{40x24,0,0,0,39x24,41,0,1}", Panes: base.Panes}))
	assert.False(t, windowsMatch(&Window{Name: "editor", Path: "~/code", Layout: "2419,80x24,0,0{40x24,0,0,0,39x24,41,0,1}", Panes: base.Panes}, &Window{Name: "editor", Path: "~/code", Layout: "18c3,80x24,0,0{40x24,0,0,0,39x24,41,0,1}", Panes: base.Panes}))
	assert.False(t, windowsMatch(base, &Window{Name: "editor", Path: "~/code", Layout: "tiled", Panes: []*Pane{{Path: "~/code", Command: "vim"}, {Path: "~/api", Command: "npm run dev", Zoom: true}}}))
	assert.False(t, windowsMatch(base, &Window{Name: "editor", Path: "~/code", Layout: "tiled", Panes: []*Pane{{Path: "~/code", Command: "vim"}, {Path: "~/api", Command: "npm test"}}}))
}
