package frecency

import "time"

// Record is one exact raw path and session-name history entry.
type Record struct {
	Path     string  `json:"path"`
	Session  string  `json:"session"`
	Rank     float64 `json:"rank"`
	LastUsed int64   `json:"last_used"`
}

type recordKey struct {
	path    string
	session string
}

// Scores contains scores calculated at one instant.
type Scores struct {
	paths   map[string]float64
	records map[recordKey]float64
}

// Score applies the zoxide age bucket for a record.
func Score(record Record, now time.Time) float64 {
	if record.LastUsed >= now.Unix() {
		return record.Rank * 4
	}
	age := uint64(now.Unix()) - uint64(record.LastUsed)
	switch {
	case age < uint64(time.Hour/time.Second):
		return record.Rank * 4
	case age < uint64(24*time.Hour/time.Second):
		return record.Rank * 2
	case age < uint64(7*24*time.Hour/time.Second):
		return record.Rank / 2
	default:
		return record.Rank / 4
	}
}

// NewScores calculates per-record scores and path sums.
func NewScores(records []Record, now time.Time) Scores {
	scores := Scores{
		paths:   make(map[string]float64, len(records)),
		records: make(map[recordKey]float64, len(records)),
	}
	for _, record := range records {
		score := Score(record, now)
		scores.paths[record.Path] += score
		scores.records[recordKey{path: record.Path, session: record.Session}] += score
	}
	return scores
}

// Path returns the sum of independently decayed scores for path.
func (s Scores) Path(path string) float64 {
	return s.paths[path]
}

// Record returns the score for one exact path and session pair.
func (s Scores) Record(path, session string) float64 {
	return s.records[recordKey{path: path, session: session}]
}
