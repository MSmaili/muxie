package frecency

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func BenchmarkLoad10K(b *testing.B) {
	path := filepath.Join(b.TempDir(), "frecency.json")
	writeStateFixture(b, path, benchmarkRecords(1))
	store := NewStore(path)

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, err := store.Load(); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkScore10K(b *testing.B) {
	records := benchmarkRecords(1)
	now := time.Unix(2_000_000, 0)

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		benchmarkScores = NewScores(records, now)
	}
}

func BenchmarkMergeWrite10K(b *testing.B) {
	path := filepath.Join(b.TempDir(), "frecency.json")
	writeStateFixture(b, path, benchmarkRecords(2))
	seed, err := os.ReadFile(path)
	if err != nil {
		b.Fatal(err)
	}
	store := NewStore(path)
	store.now = func() time.Time { return time.Unix(2_000_000, 0) }

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		b.StopTimer()
		if err := os.WriteFile(path, seed, 0600); err != nil {
			b.Fatal(err)
		}
		b.StartTimer()
		if err := store.Record(context.Background(), "/repo/00000", "session-00000"); err != nil {
			b.Fatal(err)
		}
	}
}

func benchmarkRecords(rank float64) []Record {
	const count = 10_000
	records := make([]Record, count)
	for i := range records {
		records[i] = Record{
			Path:     fmt.Sprintf("/repo/%05d", i),
			Session:  fmt.Sprintf("session-%05d", i),
			Rank:     rank,
			LastUsed: 1,
		}
	}
	return records
}

var benchmarkScores Scores
