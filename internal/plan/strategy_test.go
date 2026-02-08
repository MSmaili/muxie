package plan

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMergeStrategyPlan(t *testing.T) {
	tests := []struct {
		name string
		diff Diff
		want []Action
	}{
		{
			name: "creates missing session",
			diff: Diff{
				Sessions: ItemDiff[Session]{
					Missing: []Session{{Name: "dev", Windows: []Window{{Name: "editor", Path: "~/code"}}}},
				},
				Windows: make(map[string]ItemDiff[Window]),
			},
			want: []Action{CreateSessionAction{Name: "dev", WindowName: "editor", Path: "~/code"}},
		},
		{
			name: "creates missing window",
			diff: Diff{
				Sessions: ItemDiff[Session]{},
				Windows: map[string]ItemDiff[Window]{
					"dev": {Missing: []Window{{Name: "server", Path: "~/api"}}},
				},
			},
			want: []Action{CreateWindowAction{Session: "dev", Name: "server", Path: "~/api"}},
		},
		{
			name: "ignores extra",
			diff: Diff{
				Sessions: ItemDiff[Session]{Extra: []Session{{Name: "old"}}},
				Windows:  map[string]ItemDiff[Window]{"dev": {Extra: []Window{{Name: "unused"}}}},
			},
			want: []Action{},
		},
		{
			name: "ignores mismatched",
			diff: Diff{
				Sessions: ItemDiff[Session]{},
				Windows: map[string]ItemDiff[Window]{
					"dev": {Mismatched: []Mismatch[Window]{{Desired: Window{Name: "editor"}, Actual: Window{Name: "editor"}}}},
				},
			},
			want: []Action{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := (&MergeStrategy{}).Plan(tt.diff)
			assert.Equal(t, tt.want, plan.Actions)
		})
	}
}

func TestForceStrategyPlan(t *testing.T) {
	tests := []struct {
		name string
		diff Diff
		want []Action
	}{
		{
			name: "kills extra session",
			diff: Diff{
				Sessions: ItemDiff[Session]{Extra: []Session{{Name: "old"}}},
				Windows:  make(map[string]ItemDiff[Window]),
			},
			want: []Action{KillSessionAction{Name: "old"}},
		},
		{
			name: "kills extra window",
			diff: Diff{
				Sessions: ItemDiff[Session]{},
				Windows:  map[string]ItemDiff[Window]{"dev": {Extra: []Window{{Name: "unused"}}}},
			},
			want: []Action{KillWindowAction{Session: "dev", Window: "unused"}},
		},
		{
			name: "recreates mismatched",
			diff: Diff{
				Sessions: ItemDiff[Session]{},
				Windows: map[string]ItemDiff[Window]{
					"dev": {Mismatched: []Mismatch[Window]{{Desired: Window{Name: "editor", Path: "~/new"}, Actual: Window{Name: "editor", Path: "~/old"}}}},
				},
			},
			want: []Action{
				KillWindowAction{Session: "dev", Window: "editor"},
				CreateWindowAction{Session: "dev", Name: "editor", Path: "~/new"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := (&ForceStrategy{}).Plan(tt.diff)
			assert.Equal(t, tt.want, plan.Actions)
		})
	}
}

func TestPlanValidate(t *testing.T) {
	tests := []struct {
		name    string
		actions []Action
		wantErr bool
	}{
		{"valid session", []Action{CreateSessionAction{Name: "dev", Path: "~"}}, false},
		{"empty session name", []Action{CreateSessionAction{Name: "", Path: "~"}}, true},
		{"empty window name", []Action{CreateWindowAction{Session: "dev", Name: ""}}, true},
		{"empty window session", []Action{CreateWindowAction{Session: "", Name: "win"}}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := &Plan{Actions: tt.actions}
			err := plan.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
