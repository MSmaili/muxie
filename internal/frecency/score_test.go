package frecency

import (
	"math"
	"testing"
	"time"
)

func TestScoreDecayBoundariesAndFutureClock(t *testing.T) {
	now := time.Unix(2_000_000, 0)
	tests := []struct {
		name     string
		lastUsed time.Time
		want     float64
	}{
		{name: "under one hour", lastUsed: now.Add(-time.Hour + time.Second), want: 8},
		{name: "one hour", lastUsed: now.Add(-time.Hour), want: 4},
		{name: "under one day", lastUsed: now.Add(-24*time.Hour + time.Second), want: 4},
		{name: "one day", lastUsed: now.Add(-24 * time.Hour), want: 1},
		{name: "under one week", lastUsed: now.Add(-7*24*time.Hour + time.Second), want: 1},
		{name: "one week", lastUsed: now.Add(-7 * 24 * time.Hour), want: 0.5},
		{name: "future", lastUsed: now.Add(24 * time.Hour), want: 8},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Score(Record{Rank: 2, LastUsed: tt.lastUsed.Unix()}, now)
			if got != tt.want {
				t.Fatalf("Score() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestScoresSumDecayPerRecord(t *testing.T) {
	now := time.Unix(2_000_000, 0)
	records := []Record{
		{Path: "/repo", Session: "recent", Rank: 2, LastUsed: now.Unix()},
		{Path: "/repo", Session: "old", Rank: 8, LastUsed: now.Add(-8 * 24 * time.Hour).Unix()},
	}

	scores := NewScores(records, now)
	if got := scores.Path("/repo"); got != 10 { // 2*4 + 8/4
		t.Fatalf("Path() = %v, want 10", got)
	}
	if got := scores.Record("/repo", "recent"); got != 8 {
		t.Fatalf("Record(recent) = %v, want 8", got)
	}
	if got := scores.Record("/repo", "missing"); got != 0 {
		t.Fatalf("Record(missing) = %v, want 0", got)
	}
}

func TestScoresHandleDuplicateInputConsistently(t *testing.T) {
	now := time.Unix(2_000_000, 0)
	records := []Record{
		{Path: "/repo", Session: "work", Rank: 1, LastUsed: now.Unix()},
		{Path: "/repo", Session: "work", Rank: 2, LastUsed: now.Unix()},
	}
	scores := NewScores(records, now)
	if scores.Path("/repo") != 12 || scores.Record("/repo", "work") != 12 {
		t.Fatalf("duplicate scores disagree: path=%v record=%v", scores.Path("/repo"), scores.Record("/repo", "work"))
	}
}

func TestSessionRenameKeepsPathScoreWhileOrphanDecays(t *testing.T) {
	used := time.Unix(2_000_000, 0)
	records := []Record{{Path: "/repo", Session: "before", Rank: 12, LastUsed: used.Unix()}}

	before := NewScores(records, used)
	if before.Path("/repo") != 48 || before.Record("/repo", "after") != 0 {
		t.Fatalf("unexpected scores before rename: path=%v new-session=%v", before.Path("/repo"), before.Record("/repo", "after"))
	}

	after := NewScores(records, used.Add(8*24*time.Hour))
	if after.Path("/repo") != 3 {
		t.Fatalf("orphan path score = %v, want 3", after.Path("/repo"))
	}
	if after.Record("/repo", "after") != 0 {
		t.Fatalf("renamed session inherited secondary score %v", after.Record("/repo", "after"))
	}
}

func TestScoreNeverProducesNaNForValidRecord(t *testing.T) {
	got := Score(Record{Rank: 1, LastUsed: math.MaxInt64}, time.Unix(0, 0))
	if math.IsNaN(got) || math.IsInf(got, 0) || got != 4 {
		t.Fatalf("future-clock Score() = %v, want 4", got)
	}
}
