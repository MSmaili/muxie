package core

import (
	"testing"

	"github.com/MSmaili/hetki/internal/tui/contracts"
	"github.com/stretchr/testify/assert"
)

func TestPreferredSelectionUsesNodeIdentityNotBackendTargetSyntax(t *testing.T) {
	snapshot := contracts.Snapshot{Nodes: []contracts.Node{
		{
			ID: "session:$2", Kind: contracts.NodeKindSession, Name: "$1", Target: "$2",
			Children: []contracts.Node{{ID: "window:@9", ParentID: "session:$2", Kind: contracts.NodeKindWindow, Name: "logs", Target: "$2:@9"}},
		},
		{
			ID: "session:$1", Kind: contracts.NodeKindSession, Name: "core", Target: "$1",
			Children: []contracts.Node{
				{ID: "window:@2", ParentID: "session:$1", Kind: contracts.NodeKindWindow, Name: "logs", Target: "$1:@2"},
				{ID: "window:@3", ParentID: "session:$1", Kind: contracts.NodeKindWindow, Name: "logs", Target: "$1:@3"},
			},
		},
	}}

	assert.Equal(t, contracts.NodeID("session:$1"), preferredSelectionID(snapshot, &contracts.Intent{Type: contracts.IntentCreateSession, Name: "core"}, "", nil))
	assert.Equal(t, contracts.NodeID("window:@2"), preferredSelectionID(snapshot, &contracts.Intent{Type: contracts.IntentCreateWindow, Session: "$1", Name: "logs"}, "", nil))
	assert.Equal(t, contracts.NodeID("window:@3"), preferredSelectionID(snapshot, &contracts.Intent{Type: contracts.IntentCreateWindow, Session: "$1", Name: "logs"}, "", map[contracts.NodeID]struct{}{"window:@2": {}}))
	assert.Equal(t, contracts.NodeID("window:@2"), preferredSelectionID(snapshot, &contracts.Intent{Type: contracts.IntentRenameWindow, NodeID: "window:@2", Target: "$1:@2"}, "", nil))
	assert.Equal(t, contracts.NodeID("session:$1"), preferredSelectionID(snapshot, &contracts.Intent{Type: contracts.IntentDeleteWindow, ParentNodeID: "session:$1", Target: "$1:@2"}, "", nil))
}
