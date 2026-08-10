package core

import (
	"testing"

	"github.com/MSmaili/hetki/internal/tui/contracts"
)

func filterTestSnapshot() contracts.Snapshot {
	dev := contracts.Node{
		ID: "session:dev", Kind: contracts.NodeKindSession, Label: "dev",
		Children: []contracts.Node{
			{ID: "window:dev:0", ParentID: "session:dev", Kind: contracts.NodeKindWindow, Label: "0 editor", Path: "~/code/editor"},
			{ID: "window:dev:1", ParentID: "session:dev", Kind: contracts.NodeKindWindow, Label: "1 server", Path: "~/svc/api"},
		},
	}
	prod := contracts.Node{
		ID: "session:prod", Kind: contracts.NodeKindSession, Label: "prod",
		Children: []contracts.Node{
			{ID: "window:prod:0", ParentID: "session:prod", Kind: contracts.NodeKindWindow, Label: "0 shell", Path: "~/prod/app"},
		},
	}
	return contracts.Snapshot{Nodes: []contracts.Node{dev, prod}}
}

func newFilterModel(t *testing.T) model {
	t.Helper()
	return newModel(filterTestSnapshot(), nil)
}

func (m model) selectedNodeID() string {
	if r, ok := m.selectedRow(); ok {
		return string(r.Node.ID)
	}
	return ""
}

func rowIDs(m model) []string {
	ids := make([]string, 0, len(m.rows))
	for _, r := range m.rows {
		ids = append(ids, string(r.Node.ID))
	}
	return ids
}

func contains(ids []string, id string) bool {
	for _, v := range ids {
		if v == id {
			return true
		}
	}
	return false
}

func TestApplyFilterByPathKeepsWindowAndAncestor(t *testing.T) {
	m := newFilterModel(t)
	m.filter = "svc" // only window:dev:1's path contains this
	m.applyFilter()

	ids := rowIDs(m)
	if !contains(ids, "window:dev:1") {
		t.Fatalf("expected path match window:dev:1 to be kept, got %v", ids)
	}
	if !contains(ids, "session:dev") {
		t.Fatalf("expected ancestor session:dev to be kept, got %v", ids)
	}
	if contains(ids, "session:prod") || contains(ids, "window:prod:0") {
		t.Fatalf("did not expect prod rows for query %q, got %v", m.filter, ids)
	}
	if m.selectedNodeID() != "window:dev:1" {
		t.Fatalf("cursor should land on best match window:dev:1, got %q", m.selectedNodeID())
	}
}

func TestApplyFilterByNameStillWorks(t *testing.T) {
	m := newFilterModel(t)
	m.filter = "server"
	m.applyFilter()

	if m.selectedNodeID() != "window:dev:1" {
		t.Fatalf("cursor should land on window:dev:1 for name query, got %q", m.selectedNodeID())
	}
}

func TestJumpMatchMovesOneMatchFromAncestor(t *testing.T) {
	m := newFilterModel(t)
	m.rows = []row{
		{Node: contracts.Node{ID: "session:dev"}},
		{Node: contracts.Node{ID: "window:dev:0"}, score: 10},
		{Node: contracts.Node{ID: "window:dev:1"}, score: 9},
	}
	m.cursor = 0

	if !m.jumpMatch(true) || m.cursor != 1 {
		t.Fatalf("first jump selected row %d, want 1", m.cursor)
	}
	if !m.jumpMatch(true) || m.cursor != 2 {
		t.Fatalf("second jump selected row %d, want 2", m.cursor)
	}
}

func TestApplyFilterEmptyRestoresAllRows(t *testing.T) {
	m := newFilterModel(t)
	m.filter = "svc"
	m.applyFilter()
	m.filter = ""
	m.applyFilter()

	// dev + 2 windows + prod + 1 window = 5 rows.
	if len(m.rows) != 5 {
		t.Fatalf("expected 5 rows when filter cleared, got %d (%v)", len(m.rows), rowIDs(m))
	}
}
