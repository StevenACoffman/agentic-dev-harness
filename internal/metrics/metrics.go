// Package metrics is effectiveness accounting (SPEC-ADDITIONS §16): it optimizes
// useful outcomes per unit of scarce human attention, not raw activity. A
// regression is more attention per accepted arc, not fewer arcs.
package metrics

import "strings"

// Record is one closed arc's cost.
type Record struct {
	Arc              string `json:"arc"`
	AttentionMinutes int    `json:"attention_minutes"`
	ComputeTokens    int    `json:"compute_tokens"`
	Accepted         bool   `json:"accepted"`
}

// StepClass counts an arc's history transitions resolved deterministically
// (evaluation, gate, commit, close) versus those that took a relayed model turn
// (a strategy/execution/critic reply) — the effectiveness north-star (§16):
// accretion (routing rules, checks, lessons) should drive the model share down over
// time. It is a coarse proxy classified from history text — a direction to watch,
// not a gate.
type StepClass struct {
	Deterministic int `json:"deterministic"`
	Model         int `json:"model"`
}

// Summary aggregates records over a period.
type Summary struct {
	Arcs               int
	Accepted           int
	AttentionMinutes   int
	ComputeTokens      int
	AttentionPerAccept float64
}

// ClassifyHistory classifies each history line as a deterministic step or a relayed
// model turn (§16). A line matching neither taxonomy is ignored, so the ratio counts
// only classified steps. It is pure.
func ClassifyHistory(history []string) StepClass {
	// A relayed model turn is recorded as "<stage>: <reply>"; the deterministic
	// prefixes mark a step adh resolved itself (evaluation, gate, commit, close).
	modelStages := []string{"strategy:", "execution:", "critic:"}
	deterministic := []string{"evaluation:", "committed", "closed", "gate", "reverted", "requalify"}
	var class StepClass
	for _, line := range history {
		switch {
		case hasAnyPrefix(line, modelStages):
			class.Model++
		case hasAnyPrefix(line, deterministic):
			class.Deterministic++
		}
	}
	return class
}

// Add sums two step classes, for aggregating over many arcs.
func (c StepClass) Add(other StepClass) StepClass {
	return StepClass{
		Deterministic: c.Deterministic + other.Deterministic,
		Model:         c.Model + other.Model,
	}
}

// Ratio is the deterministic share of classified steps (0 when none classified) —
// the north-star to watch trend upward as accretion shrinks the model surface.
func (c StepClass) Ratio() float64 {
	total := c.Deterministic + c.Model
	if total == 0 {
		return 0
	}
	return float64(c.Deterministic) / float64(total)
}

// hasAnyPrefix reports whether line begins with any of the prefixes.
func hasAnyPrefix(line string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}
	return false
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
