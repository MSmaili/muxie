package state

import (
	"github.com/MSmaili/tms/internal/domain"
)

func compareSessions(diff *domain.Diff, desired, actual map[string][]domain.Window) *domain.Diff {

	for session := range desired {
		_, ok := actual[session]
		if !ok {
			diff.MissingSessions = append(diff.MissingSessions, session)
		} else {
			diff.CommonSessions = append(diff.CommonSessions, session)
		}
	}

	for session := range actual {
		_, ok := desired[session]
		if !ok {
			diff.ExtraSessions = append(diff.ExtraSessions, session)
		}
	}

	return diff
}
