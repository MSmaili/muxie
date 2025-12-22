package state

import (
	"github.com/MSmaili/tms/internal/domain"
)

type Diff interface {
	Compare(desired, actual map[string][]domain.Window, mode domain.CompareMode) domain.Diff
}

func Compare(desired, actual map[string][]domain.Window, mode domain.CompareMode) domain.Diff {
	diff := domain.Diff{
		MissingWindows: make(map[string][]domain.Window),
		ExtraWindows:   make(map[string][]domain.Window),
		Mismatched:     make(map[string][]domain.WindowMismatch),
	}

	compareSessions(&diff, desired, actual)
	compareWindows(&diff, desired, actual, mode)

	return diff
}
