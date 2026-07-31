// Package verdict is the replication-gated outcome-eval verdict (SPEC-ADDITIONS
// §18.2): a pure decision core that layers a trust taxonomy over the cheap strict-`>`
// gate. A single held-out comparison cannot separate signal from noise, so a change
// is ELEVATE only when its primary gain is meaningful AND replicates significantly on
// an independent split; otherwise the verdict refuses to elevate and says why. It
// performs no I/O and reads no clock — the caller supplies the measured outcomes.
package verdict

import (
	"math"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
)

// chiSqCritical is the χ²(1 df, α=.05) critical value; a McNemar statistic above it
// is a significant paired difference.
const chiSqCritical = 3.841

// DefaultMinEffect is the smallest score delta (on the 0..1 rule-judge scale) that
// counts as a meaningful effect for ELEVATE — a smaller gain is real but not worth
// elevating on without more evidence (§18.2).
const DefaultMinEffect = 0.05

// Verdict values: the outcome-eval taxonomy (§18.2). ELEVATE requires a primary gain
// AND an independent replication; the rest refuse to elevate, honestly.
const (
	Elevate            Verdict = "ELEVATE"
	Directional        Verdict = "DIRECTIONAL-NOT-REPLICATED"
	ReplicationMissing Verdict = "REPLICATION-MISSING"
	Kill               Verdict = "KILL"
)

// Verdict is the replication-gated adoption verdict.
type Verdict string

// Outcome is one condition's measured result: the score delta (candidate − baseline)
// and whether a paired significance test found the change significant.
type Outcome struct {
	Delta       float64
	Significant bool
}

// SplitAssignment pairs a task id with the split it was assigned, for the leakage
// guard.
type SplitAssignment struct {
	ID    string
	Split string
}

// Decide maps a primary and a replication outcome to a verdict (§18.2): KILL on a
// primary regression; REPLICATION-MISSING when there was no held-out replication to
// check; ELEVATE only when the primary gain clears minEffect AND the replication
// held without regressing and is significant; DIRECTIONAL otherwise — a primary gain
// that did not replicate. It is pure.
func Decide(primary, replication Outcome, minEffect float64, hasReplication bool) Verdict {
	switch {
	case primary.Delta < 0:
		return Kill
	case !hasReplication:
		return ReplicationMissing
	case primary.Delta >= minEffect && replication.Delta >= 0 && replication.Significant:
		return Elevate
	default:
		return Directional
	}
}

// Replicate is the fresh-replication verdict (§18.2): a single held-out comparison
// (Decide) can still be a fluke, so ELEVATE here requires at least two *independent*
// runs — a primary and a fresh replication — that each clear minEffect and are
// significant. Any run that regressed is KILL; fewer than two runs is
// REPLICATION-MISSING (there is no fresh replication to trust); otherwise
// DIRECTIONAL. It is pure — the caller supplies each independent run's outcome.
func Replicate(runs []Outcome, minEffect float64) Verdict {
	if anyRegressed(runs) {
		return Kill
	}
	if len(runs) < 2 {
		return ReplicationMissing
	}
	if allElevate(runs, minEffect) {
		return Elevate
	}
	return Directional
}

// anyRegressed reports whether any run's delta went negative.
func anyRegressed(runs []Outcome) bool {
	for i := range runs {
		if runs[i].Delta < 0 {
			return true
		}
	}
	return false
}

// allElevate reports whether every run clears minEffect and is significant.
func allElevate(runs []Outcome, minEffect float64) bool {
	for i := range runs {
		if runs[i].Delta < minEffect || !runs[i].Significant {
			return false
		}
	}
	return true
}

// McNemar runs McNemar's test over paired before/after binary outcomes (§18.2):
// improved is the count that flipped fail→pass, regressed the count that flipped
// pass→fail. It returns the continuity-corrected χ² statistic and whether it clears
// the χ²(1, .05) critical value. With no discordant pairs there is no evidence of a
// difference. It is pure.
func McNemar(improved, regressed int) (stat float64, significant bool) {
	discordant := improved + regressed
	if discordant == 0 {
		return 0, false
	}
	corrected := math.Abs(float64(improved-regressed)) - 1
	if corrected < 0 {
		corrected = 0
	}
	stat = corrected * corrected / float64(discordant)
	return stat, stat > chiSqCritical
}

// ValidateSplits guards against split leakage (§18.2): a task id assigned to more
// than one split makes the held-out replication a proxy for the primary rather than
// an independent check. It returns EINVALID naming the first leaked id. It is pure.
func ValidateSplits(assignments []SplitAssignment) error {
	seen := make(map[string]string, len(assignments))
	for i := range assignments {
		id, split := assignments[i].ID, assignments[i].Split
		if prev, ok := seen[id]; ok && prev != split {
			return &adh.Error{
				Code:    adh.EINVALID,
				Message: "split leakage: task " + id + " is in both " + prev + " and " + split,
			}
		}
		seen[id] = split
	}
	return nil
}
