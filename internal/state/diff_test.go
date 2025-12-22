package state

import (
	"testing"

	"github.com/MSmaili/tms/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestWindowsEqual(t *testing.T) {
	t.Helper()

	i1 := 1
	i2 := 2

	tests := []struct {
		name string
		a, b domain.Window
		want bool
	}{
		{
			name: "Equal windows",
			a:    domain.Window{Name: "a", Path: "/x", Index: &i1, Layout: "h", Command: "ls"},
			b:    domain.Window{Name: "a", Path: "/x", Index: &i1, Layout: "h", Command: "ls"},
			want: true,
		},
		{
			name: "Different name",
			a:    domain.Window{Name: "a", Path: "/x"},
			b:    domain.Window{Name: "b", Path: "/x"},
			want: false,
		},
		{
			name: "Different path",
			a:    domain.Window{Name: "a", Path: "/x"},
			b:    domain.Window{Name: "a", Path: "/y"},
			want: false,
		},
		{
			name: "Different index pointer values",
			a:    domain.Window{Index: &i1},
			b:    domain.Window{Index: &i2},
			want: false,
		},
		{
			name: "One index nil, one not",
			a:    domain.Window{Index: nil},
			b:    domain.Window{Index: &i1},
			want: false,
		},
		{
			name: "Both index nil",
			a:    domain.Window{Index: nil},
			b:    domain.Window{Index: nil},
			want: true,
		},
		{
			name: "Different layout",
			a:    domain.Window{Layout: "h"},
			b:    domain.Window{Layout: "v"},
			want: false,
		},
		{
			name: "Different command",
			a:    domain.Window{Command: "ls"},
			b:    domain.Window{Command: "top"},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, windowsEqual(tt.a, tt.b, domain.CompareStrict))
		})
	}
}

func TestCompareMixedDiffInSameSession(t *testing.T) {
	desired := map[string][]domain.Window{
		"s": {
			{Name: "A", Path: "/A"}, // MISMATCHED
			{Name: "B", Path: "/B"}, // MISSING
			{Name: "C", Path: "/C"}, // MATCH
		},
	}
	actual := map[string][]domain.Window{
		"s": {
			{Name: "A", Path: "/A", Command: "git status"}, // mismatched
			{Name: "C", Path: "/C"},                        // match
			{Name: "D", Path: "/D"},                        // extra
		},
	}

	diff := Compare(desired, actual, domain.CompareStrict)

	assert.Len(t, diff.MissingWindows["s"], 1)
	assert.Equal(t, "B", diff.MissingWindows["s"][0].Name)

	assert.Len(t, diff.Mismatched["s"], 1)
	assert.Equal(t, "A", diff.Mismatched["s"][0].Desired.Name)
	assert.Equal(t, "A", diff.Mismatched["s"][0].Actual.Name)
	assert.Equal(t, "", diff.Mismatched["s"][0].Desired.Command)

	assert.Len(t, diff.ExtraWindows["s"], 1)
	assert.Equal(t, "D", diff.ExtraWindows["s"][0].Name)
}

func TestCompareKeyCollisionOverridesEarlier(t *testing.T) {
	desired := map[string][]domain.Window{
		"s": {
			{Name: "first", Path: "/collision"},
			{Name: "first", Path: "/collision"}, // overrides first
		},
	}
	actual := map[string][]domain.Window{
		"s": {},
	}

	diff := Compare(desired, actual, domain.CompareStrict)

	// Only the last one should appear due to map override.
	assert.Len(t, diff.MissingWindows["s"], 1)
	assert.Equal(t, "first", diff.MissingWindows["s"][0].Name)
}

func TestCompareMultipleMissingExtra(t *testing.T) {
	desired := map[string][]domain.Window{
		"s": {
			{Name: "A", Path: "/A"},
			{Name: "B", Path: "/B"},
		},
	}
	actual := map[string][]domain.Window{
		"s": {
			{Name: "C", Path: "/C"},
			{Name: "D", Path: "/D"},
		},
	}

	diff := Compare(desired, actual, domain.CompareStrict)

	assert.Len(t, diff.MissingWindows["s"], 2)
	assert.Len(t, diff.ExtraWindows["s"], 2)
}
