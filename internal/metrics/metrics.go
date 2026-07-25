// Package metrics is effectiveness accounting (SPEC-ADDITIONS §16): it optimizes
// useful outcomes per unit of scarce human attention, not raw activity. A
// regression is more attention per accepted arc, not fewer arcs.
package metrics

// Record is one closed arc's cost.
type Record struct {
	Arc              string `json:"arc"`
	AttentionMinutes int    `json:"attention_minutes"`
	ComputeTokens    int    `json:"compute_tokens"`
	Accepted         bool   `json:"accepted"`
}

// Summary aggregates records over a period.
type Summary struct {
	Arcs               int
	Accepted           int
	AttentionMinutes   int
	ComputeTokens      int
	AttentionPerAccept float64
}

// Summarize aggregates records. AttentionPerAccept — the headline effectiveness
// figure — divides attention by accepted arcs (never by zero).
func Summarize(records []Record) Summary {
	var s Summary
	for i := range records {
		s.Arcs++
		s.AttentionMinutes += records[i].AttentionMinutes
		s.ComputeTokens += records[i].ComputeTokens
		if records[i].Accepted {
			s.Accepted++
		}
	}
	denom := s.Accepted
	if denom == 0 {
		denom = 1
	}
	s.AttentionPerAccept = float64(s.AttentionMinutes) / float64(denom)
	return s
}

// AttentionDelta returns this period's attention-per-accept minus the prior
// period's. A positive delta is a regression (more attention per outcome).
func (s Summary) AttentionDelta(prev Summary) float64 {
	return s.AttentionPerAccept - prev.AttentionPerAccept
}
