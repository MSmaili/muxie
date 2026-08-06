package core

import "strings"

func (m *model) applyFilter() {
	query := m.filterQuery()
	if query == "" {
		m.rows = flatten(m.snapshot.Nodes, m.expanded, false)
		m.cursor = clampCursor(m.cursor, len(m.rows))
		return
	}

	allRows := flatten(m.snapshot.Nodes, m.expanded, true)
	m.rows = keepMatchesWithAncestors(allRows, query)
	m.cursor = bestMatchCursor(m.rows)
}

// keepMatchesWithAncestors returns matching rows plus the ancestors needed to
// keep the tree connected, storing each row's score so it is computed once.
func keepMatchesWithAncestors(rows []row, query string) []row {
	parentByID := make(map[string]string, len(rows))
	scoreByID := make(map[string]int, len(rows))
	keep := make(map[string]bool, len(rows))

	for _, r := range rows {
		parentByID[r.Node.ID] = r.Node.ParentID
		if s := nodeScore(r.Node, query); s > 0 {
			scoreByID[r.Node.ID] = s
			keep[r.Node.ID] = true
		}
	}

	for id := range keep {
		for parent := parentByID[id]; parent != ""; parent = parentByID[parent] {
			keep[parent] = true
		}
	}

	filtered := make([]row, 0, len(rows))
	for _, r := range rows {
		if keep[r.Node.ID] {
			r.score = scoreByID[r.Node.ID]
			filtered = append(filtered, r)
		}
	}
	return filtered
}

// bestMatchCursor returns the index of the highest-scoring row, resolving ties
// to the first one. Returns 0 when nothing matches.
func bestMatchCursor(rows []row) int {
	bestIdx, bestScore := 0, 0
	for i, r := range rows {
		if r.score > bestScore {
			bestScore, bestIdx = r.score, i
		}
	}
	return bestIdx
}

func (m model) filterQuery() string {
	return strings.ToLower(strings.TrimSpace(m.filter))
}

// matchIndices lists matching rows in display order, reading the precomputed
// score instead of re-matching.
func (m model) matchIndices() []int {
	indices := make([]int, 0, len(m.rows))
	for i, r := range m.rows {
		if r.score > 0 {
			indices = append(indices, i)
		}
	}
	return indices
}

func (m *model) jumpMatch(forward bool) bool {
	indices := m.matchIndices()
	if len(indices) == 0 {
		return false
	}

	next := indices[0]
	if forward {
		for _, idx := range indices {
			if idx > m.cursor {
				next = idx
				break
			}
		}
	} else {
		next = indices[len(indices)-1]
		for i := len(indices) - 1; i >= 0; i-- {
			if indices[i] < m.cursor {
				next = indices[i]
				break
			}
		}
	}
	m.cursor = next
	*m = m.reflow()
	return true
}

func (m model) filterMatchPosition() (int, int) {
	indices := m.matchIndices()
	if len(indices) == 0 {
		return 0, 0
	}
	for i, idx := range indices {
		if idx == m.cursor {
			return i + 1, len(indices)
		}
	}
	return 1, len(indices)
}
