package state

import "strings"

type windowKey struct {
	Name string
	Path string
}

func compareWindows(diff *Diff, desired, actual *State) {
	common := CommonSessions(desired, actual)

	for _, sessionName := range common {
		desiredSession := desired.Sessions[sessionName]
		actualSession := actual.Sessions[sessionName]

		windowDiff := compareSessionWindows(desiredSession.Windows, actualSession.Windows)
		if !windowDiff.IsEmpty() {
			diff.Windows[sessionName] = windowDiff
		}
	}
}

func compareSessionWindows(desired, actual []*Window) ItemDiff[Window] {
	actualMap := windowsByKey(actual)
	actualNameCounts := windowNameCounts(actual)

	windowDiff := ItemDiff[Window]{
		Missing:    make([]Window, 0, len(desired)),
		Extra:      make([]Window, 0, len(actual)),
		Mismatched: make([]Mismatch[Window], 0),
	}

	for _, desiredWindow := range desired {
		key := keyForWindow(desiredWindow)
		actualWindow, exists := actualMap[key]
		if !exists {
			windowDiff.Missing = append(windowDiff.Missing, *desiredWindow)
		} else {
			if !windowsMatch(desiredWindow, actualWindow) && actualNameCounts[actualWindow.Name] == 1 {
				windowDiff.Mismatched = append(windowDiff.Mismatched, Mismatch[Window]{
					Desired: *desiredWindow,
					Actual:  *actualWindow,
				})
			}
			delete(actualMap, key)
		}
	}

	for _, actualWindow := range actual {
		key := keyForWindow(actualWindow)
		if _, exists := actualMap[key]; exists {
			if actualNameCounts[actualWindow.Name] == 1 {
				windowDiff.Extra = append(windowDiff.Extra, *actualWindow)
			}
			delete(actualMap, key)
		}
	}

	return windowDiff
}

func windowNameCounts(windows []*Window) map[string]int {
	counts := make(map[string]int, len(windows))
	for _, window := range windows {
		counts[window.Name]++
	}
	return counts
}

func windowsMatch(desired, actual *Window) bool {
	if !layoutMatches(desired.Layout, actual.Layout) {
		return false
	}

	if len(desired.Panes) != len(actual.Panes) {
		return false
	}

	// Panes are matched by stable positional order after window matching by name+path.
	for i := range desired.Panes {
		if !panesMatch(desired.Panes[i], actual.Panes[i]) {
			return false
		}
	}

	return true
}

func panesMatch(desired, actual *Pane) bool {
	return desired.Path == actual.Path && desired.Command == actual.Command && desired.Zoom == actual.Zoom
}

func layoutMatches(desired, actual string) bool {
	if desired == "" || desired == actual {
		return true
	}

	// Best effort: manifests commonly use preset names like "tiled", while tmux
	// reports the live layout as a serialized layout string that will not match
	// the preset name directly.
	if isNamedLayout(desired) {
		return true
	}

	return false
}

func isNamedLayout(layout string) bool {
	switch strings.TrimSpace(layout) {
	case "even-horizontal", "even-vertical", "main-horizontal", "main-vertical", "tiled":
		return true
	default:
		return false
	}
}

func windowsByKey(windows []*Window) map[windowKey]*Window {
	m := make(map[windowKey]*Window, len(windows))
	for _, w := range windows {
		m[keyForWindow(w)] = w
	}
	return m
}

func keyForWindow(w *Window) windowKey {
	return windowKey{Name: w.Name, Path: w.Path}
}
