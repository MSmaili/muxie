package core

import (
	"strings"

	"github.com/MSmaili/hetki/internal/tui/contracts"
	"github.com/sahilm/fuzzy"
)

// Match scoring for the filter. A node is scored by fuzzy-matching its name and
// (for windows) its path; the best field wins. The score is a field tier plus
// the fuzzy quality within it. The tier gap must stay wider than maxQuality so a
// name match always outranks a path match (see TestTierGapProtectsPriority).
//
// Frecency will later add a term on top, in keepMatchesWithAncestors.
const (
	priorityName = 2000
	priorityPath = 1000
	maxQuality   = 999
)

// nodeScore reports how well node matches query; 0 means no match. query is
// expected to be already normalized (lowercased, trimmed).
func nodeScore(n contracts.Node, query string) int {
	if query == "" {
		return 0
	}

	best := 0
	if q, ok := fieldQuality(n.Label, query); ok {
		best = max(best, priorityName+q)
	}
	if path := strings.TrimSpace(n.Path); path != "" {
		if q, ok := fieldQuality(path, query); ok {
			best = max(best, priorityPath+q)
		}
	}
	return best
}

func fieldQuality(value, query string) (int, bool) {
	// FindNoSort avoids sorting a single-element result.
	matches := fuzzy.FindNoSort(query, []string{value})
	if len(matches) == 0 {
		return 0, false
	}
	return clampQuality(matches[0].Score), true
}

func clampQuality(score int) int {
	if score < 0 {
		return 0
	}
	if score > maxQuality {
		return maxQuality
	}
	return score
}
