package core

import (
	"testing"

	"github.com/MSmaili/hetki/internal/tui/contracts"
)

func TestNodeScoreMatching(t *testing.T) {
	nameNode := contracts.Node{Kind: contracts.NodeKindWindow, Label: "0 editor", Path: "~/code"}
	pathNode := contracts.Node{Kind: contracts.NodeKindWindow, Label: "0 shell", Path: "~/svc/api"}

	tests := []struct {
		name    string
		node    contracts.Node
		query   string
		matched bool
	}{
		{name: "empty query never matches", node: nameNode, query: "", matched: false},
		{name: "matches on name", node: nameNode, query: "editor", matched: true},
		{name: "matches on path only", node: pathNode, query: "api", matched: true},
		{name: "fuzzy subsequence on path", node: pathNode, query: "svapi", matched: true},
		{name: "case-insensitive", node: pathNode, query: "API", matched: true},
		{name: "no match", node: nameNode, query: "zzz", matched: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := nodeScore(tt.node, tt.query) > 0
			if got != tt.matched {
				t.Fatalf("nodeScore(%q) matched = %v, want %v", tt.query, got, tt.matched)
			}
		})
	}
}

func TestNodeScoreNameOutranksPath(t *testing.T) {
	byName := contracts.Node{Kind: contracts.NodeKindWindow, Label: "api", Path: "~/unrelated"}
	byPath := contracts.Node{Kind: contracts.NodeKindWindow, Label: "shell", Path: "~/svc/api"}

	if nodeScore(byName, "api") <= nodeScore(byPath, "api") {
		t.Fatalf("expected a name match to outrank a path-only match for query %q", "api")
	}
}

// TestTierGapProtectsPriority guards the invariant that a name match always
// outranks a path match: the gap between tiers must exceed the maximum fuzzy
// quality that can be added within a tier.
func TestTierGapProtectsPriority(t *testing.T) {
	if priorityName-priorityPath <= maxQuality {
		t.Fatalf("tier gap (%d) must exceed maxQuality (%d) to keep name above path",
			priorityName-priorityPath, maxQuality)
	}
}

func TestNodeScoreSlashedQueryMatchesPath(t *testing.T) {
	// A slashed query can only fuzzy-match a field that contains a slash, which
	// in practice means the path. No special-case promotion is needed.
	byPath := contracts.Node{Kind: contracts.NodeKindWindow, Label: "shell", Path: "~/svc/api"}

	if got := nodeScore(byPath, "svc/api"); got <= 0 {
		t.Fatalf("expected path match for slashed query, got %d", got)
	}
}
