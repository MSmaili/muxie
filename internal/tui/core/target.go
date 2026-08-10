package core

import (
	"strings"

	"github.com/MSmaili/hetki/internal/tui/contracts"
)

func findRowIndexByID(rows []row, nodeID contracts.NodeID) int {
	if nodeID == "" {
		return -1
	}
	for i := range rows {
		if rows[i].Node.ID == nodeID {
			return i
		}
	}
	return -1
}

func preferredSelectionID(snapshot contracts.Snapshot, intent *contracts.Intent, previousID contracts.NodeID, existingNodeIDs map[contracts.NodeID]struct{}) contracts.NodeID {
	if intent == nil {
		return previousID
	}
	name := strings.TrimSpace(intent.Name)
	switch intent.Type {
	case contracts.IntentCreateSession:
		if name != "" {
			return findSessionNodeIDByName(snapshot.Nodes, name)
		}
	case contracts.IntentRenameSession, contracts.IntentRenameWindow:
		return intent.NodeID
	case contracts.IntentCreateWindow:
		session := strings.TrimSpace(string(intent.Session))
		if session == "" {
			session = string(sessionFromNodeTarget(intent.Target))
		}
		if session != "" && name != "" {
			if id := findWindowNodeIDByName(snapshot.Nodes, session, name, existingNodeIDs); id != "" {
				return id
			}
		}
	case contracts.IntentDeleteWindow:
		return intent.ParentNodeID
	}
	return previousID
}

func sessionFromNodeTarget(target contracts.BackendTarget) contracts.BackendTarget {
	value := strings.TrimSpace(string(target))
	if value == "" {
		return ""
	}
	session, _, hasWindow := strings.Cut(value, ":")
	if hasWindow {
		return contracts.BackendTarget(strings.TrimSpace(session))
	}
	return contracts.BackendTarget(value)
}

func findSessionNodeIDByName(nodes []contracts.Node, name string) contracts.NodeID {
	for _, session := range nodes {
		if session.Kind == contracts.NodeKindSession && session.Name == name {
			return session.ID
		}
	}
	return ""
}

func findWindowNodeIDByName(nodes []contracts.Node, sessionName, windowName string, excluded map[contracts.NodeID]struct{}) contracts.NodeID {
	var matched *contracts.Node
	for i := range nodes {
		if nodes[i].Kind != contracts.NodeKindSession {
			continue
		}
		if string(nodes[i].Target) == sessionName {
			matched = &nodes[i]
			break
		}
		if matched == nil && nodes[i].Name == sessionName {
			matched = &nodes[i]
		}
	}
	if matched == nil {
		return ""
	}
	for _, window := range matched.Children {
		if _, exists := excluded[window.ID]; window.Name == windowName && !exists {
			return window.ID
		}
	}
	return ""
}

func collectNodeIDs(nodes []contracts.Node) map[contracts.NodeID]struct{} {
	ids := make(map[contracts.NodeID]struct{})
	var collect func([]contracts.Node)
	collect = func(nodes []contracts.Node) {
		for _, node := range nodes {
			ids[node.ID] = struct{}{}
			collect(node.Children)
		}
	}
	collect(nodes)
	return ids
}

func windowDisplayName(label string) string {
	parts := strings.Fields(strings.TrimSpace(label))
	if len(parts) <= 1 {
		return strings.TrimSpace(label)
	}
	return strings.Join(parts[1:], " ")
}
