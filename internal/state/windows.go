package state

import (
	"fmt"

	"github.com/MSmaili/tms/internal/domain"
)

func compareWindows(diff *domain.Diff, desired, actual map[string][]domain.Window, mode domain.CompareMode) *domain.Diff {
	for session, desiredWindows := range desired {
		processSession(diff, session, desiredWindows, actual[session], mode)
	}

	//missing session windows
	for session, actualWindows := range actual {
		_, ok := desired[session]
		if !ok {
			diff.ExtraWindows[session] = append(diff.ExtraWindows[session], actualWindows...)
		}
	}

	return diff
}

func processSession(diff *domain.Diff, session string, desiredWindows []domain.Window, actualWindows []domain.Window, mode domain.CompareMode) {

	desiredMap := windowsKey(desiredWindows)
	actualMap := windowsKey(actualWindows)

	missing := missingWindows(desiredMap, actualMap)
	mismatched := mismatchedWindows(desiredMap, actualMap, mode)
	extra := extraWindows(desiredMap, actualMap)

	if len(missing) > 0 {
		diff.MissingWindows[session] = missing
	}

	if len(mismatched) > 0 {
		diff.Mismatched[session] = mismatched
	}

	if len(extra) > 0 {
		diff.ExtraWindows[session] = extra
	}
}

func missingWindows(desiredMap, actualMap map[string]domain.Window) []domain.Window {
	var missing []domain.Window

	for key, dw := range desiredMap {
		_, exist := actualMap[key]
		if !exist {
			missing = append(missing, dw)
		}
	}
	return missing
}

func mismatchedWindows(desiredMap, actualMap map[string]domain.Window, mode domain.CompareMode) []domain.WindowMismatch {
	var mismatched []domain.WindowMismatch

	for key, dw := range desiredMap {
		aw, ok := actualMap[key]
		if !ok {
			continue // handled as missing
		}
		if !windowsEqual(dw, aw, mode) {
			mismatched = append(mismatched, domain.WindowMismatch{
				Actual:  aw,
				Desired: dw,
			})
		}
	}

	return mismatched
}

func extraWindows(desiredMap, actualMap map[string]domain.Window) []domain.Window {
	var extraWindows []domain.Window

	for key, aw := range actualMap {
		_, exist := desiredMap[key]
		if !exist {
			extraWindows = append(extraWindows, aw)
		}
	}
	return extraWindows
}

func windowsKey(windows []domain.Window) map[string]domain.Window {
	m := make(map[string]domain.Window, len(windows))
	for _, w := range windows {
		key := windowKey(w)
		m[key] = w
	}
	return m
}

func windowKey(w domain.Window) string {
	return fmt.Sprintf("%s|%s", w.Name, w.Path)
}

func windowsEqual(w1, w2 domain.Window, mode domain.CompareMode) bool {
	if mode&domain.CompareIgnoreName == 0 && w1.Name != w2.Name {
		return false
	}
	if mode&domain.CompareIgnorePath == 0 && w1.Path != w2.Path {
		return false
	}
	if mode&domain.CompareIgnoreIndex == 0 && !intPtrEqual(w1.Index, w2.Index) {
		return false
	}
	if mode&domain.CompareIgnoreLayout == 0 && w1.Layout != w2.Layout {
		return false
	}
	if mode&domain.CompareIgnoreCommand == 0 && w1.Command != w2.Command {
		return false
	}
	return true
}

func intPtrEqual(a, b *int) bool {
	if a == b {
		return true // covers both nil
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}
