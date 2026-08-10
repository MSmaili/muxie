package core

import (
	"strings"

	"github.com/MSmaili/hetki/internal/tui/contracts"
)

func flatten(nodes []contracts.Node, expanded map[contracts.NodeID]bool, includeAll bool) []row {
	out := make([]row, 0)
	flattenAtDepth(nodes, nil, expanded, includeAll, &out)
	return out
}

func flattenAtDepth(nodes []contracts.Node, ancestors []bool, expanded map[contracts.NodeID]bool, includeAll bool, out *[]row) {
	for i, n := range nodes {
		hasNext := i < len(nodes)-1
		depth := len(ancestors)
		isExpanded := includeAll || expanded[n.ID]
		*out = append(*out, row{
			Node:       n,
			Depth:      depth,
			TreePrefix: treePrefix(ancestors, hasNext),
			Expanded:   isExpanded,
			Branch:     len(n.Children) > 0,
		})
		if len(n.Children) > 0 {
			nextAncestors := append(append([]bool(nil), ancestors...), hasNext)
			if isExpanded {
				flattenAtDepth(n.Children, nextAncestors, expanded, includeAll, out)
			}
		}
	}
}

func defaultExpanded(nodes []contracts.Node, activeNodeID contracts.NodeID) map[contracts.NodeID]bool {
	expanded := make(map[contracts.NodeID]bool)
	markAllExpanded(nodes, expanded)
	if activeNodeID != "" {
		markActivePathExpanded(nodes, activeNodeID, expanded)
	}
	return expanded
}

func markActivePathExpanded(nodes []contracts.Node, activeNodeID contracts.NodeID, expanded map[contracts.NodeID]bool) bool {
	for _, n := range nodes {
		if n.ID == activeNodeID {
			expanded[n.ID] = true
			return true
		}
		if markActivePathExpanded(n.Children, activeNodeID, expanded) {
			expanded[n.ID] = true
			return true
		}
	}
	return false
}

func markAllExpanded(nodes []contracts.Node, expanded map[contracts.NodeID]bool) {
	for _, n := range nodes {
		if len(n.Children) > 0 {
			expanded[n.ID] = true
			markAllExpanded(n.Children, expanded)
		}
	}
}

func treePrefix(ancestors []bool, hasNext bool) string {
	if len(ancestors) == 0 {
		return ""
	}

	var b strings.Builder
	for _, ancestorHasNext := range ancestors[:len(ancestors)-1] {
		if ancestorHasNext {
			b.WriteString(" │  ")
		} else {
			b.WriteString("    ")
		}
	}
	b.WriteString(" │  ")
	return b.String()
}
