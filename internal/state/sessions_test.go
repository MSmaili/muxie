package state

import (
	"testing"

	"github.com/MSmaili/tms/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestCompareSessions(t *testing.T) {

	tests := []struct {
		name    string
		desired map[string][]domain.Window
		actual  map[string][]domain.Window
		test    func(t *testing.T, diff *domain.Diff)
	}{
		{
			name: "Adds the desired to missing when nothing is in actual",
			desired: map[string][]domain.Window{
				"session1": {
					{
						Name: "window1",
						Path: "~/",
					},
				},
			},
			actual: map[string][]domain.Window{},
			test: func(t *testing.T, diff *domain.Diff) {
				assert.Equal(t, 1, 1)
				assert.Equal(t, len(diff.MissingSessions), 1)
			},
		},
		{
			name: "Adds extra from actual",
			desired: map[string][]domain.Window{
				"session1": {},
			},
			actual: map[string][]domain.Window{
				"session2": {},
			},
			test: func(t *testing.T, diff *domain.Diff) {
				assert.Equal(t, 1, 1)
				assert.Equal(t, len(diff.MissingSessions), 1)
				assert.Equal(t, len(diff.ExtraSessions), 1)
			},
		},
		{
			name: "Adds as common for same session names",
			desired: map[string][]domain.Window{
				"session1": {},
			},
			actual: map[string][]domain.Window{
				"session1": {},
			},
			test: func(t *testing.T, diff *domain.Diff) {
				assert.Equal(t, 1, 1)
				assert.Equal(t, len(diff.MissingSessions), 0)
				assert.Equal(t, len(diff.ExtraSessions), 0)
				assert.Equal(t, len(diff.CommonSessions), 1)
			},
		},
		{
			name:    "Adds as extra when desired is empty",
			desired: map[string][]domain.Window{},
			actual: map[string][]domain.Window{
				"session1": {},
			},
			test: func(t *testing.T, diff *domain.Diff) {
				assert.Equal(t, 1, 1)
				assert.Equal(t, len(diff.MissingSessions), 0)
				assert.Equal(t, len(diff.ExtraSessions), 1)
				assert.Equal(t, len(diff.CommonSessions), 0)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diff := domain.Diff{}
			tt.test(t, compareSessions(&diff, tt.desired, tt.actual))
		})
	}
}
