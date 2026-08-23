package evaluation_test

import (
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/evaluation"
)

// TestBarMoved. ruflo rejected changes carrying large favourable metrics because a
// named guard moved, under a hypothesis "frozen before evaluation began; not modified
// after seeing results." A bar that can move after the numbers arrive is a note rather
// than a pre-registration.
func TestBarMoved(t *testing.T) {
	t.Parallel()
	const planned = "the proof must show the failing test now passes"
	cases := map[string]struct {
		recorded string
		current  string
		want     bool
	}{
		"unchanged": {evaluation.BarHash(planned), planned, false},
		"changed":   {evaluation.BarHash(planned), planned + " and nothing regressed", true},
		// The case that decides the zero value. An arc planned before
		// pre-registration shipped records no bar, and reporting drift on every one
		// of those teaches a reader to ignore the report.
		"no bar recorded": {"", planned, false},
		// A bar that became empty is still a change, and must not be excused by the
		// same rule that excuses an unrecorded one.
		"the bar emptied": {evaluation.BarHash(planned), "", true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			arc := adh.Arc{ID: "a1", Bar: tc.recorded}
			if got := evaluation.BarMoved(&arc, tc.current); got != tc.want {
				t.Errorf("BarMoved = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestBarHashIsTheFamilyFingerprint, so a bar hash recorded here is comparable to one
// any other tool in the family writes.
func TestBarHashIsTheFamilyFingerprint(t *testing.T) {
	t.Parallel()
	if a, b := evaluation.BarHash("x"), evaluation.BarHash("x"); a != b {
		t.Error("BarHash is not deterministic")
	}
	if evaluation.BarHash("x") == evaluation.BarHash("y") {
		t.Error("BarHash collides on distinct bars")
	}
}
