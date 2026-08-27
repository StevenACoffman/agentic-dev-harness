package evaluation

import (
	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/nfr"
)

// GuardFindings turns every declared guard into a finding to adjudicate (§10.5, §19.2).
//
// Requires: specs is the repository's NFR set; a nil or guard-free set yields none.
// Ensures: one finding per guard, ordered as declared, each naming its spec so the
// existing NFR adjudicator measures it. Empty rather than nil. Pure.
//
// **Guards are adjudicated whether or not a critic mentioned them, and that is the
// whole point.** A critic's findings say what it thought to look at; a guard says what
// the repository will not trade away regardless. Routing them through the same
// adjudication path means a breached guard is a *confirmed* finding and blocks by the
// existing rule — no second gate, no new verdict, and the same evidence requirement
// every other confirmation meets.
//
// The synthesised summary names the spec rather than describing the breach, because
// the measurement that would describe it has not happened yet: adjudicateSpec measures
// the meter and decides. A summary asserting a breach here would be a finding claiming
// its own conclusion.
func GuardFindings(specs []nfr.Spec) []adh.Finding {
	guards := nfr.Guards(specs)
	out := make([]adh.Finding, 0, len(guards))
	for i := range guards {
		g := &guards[i]
		out = append(out, adh.Finding{
			Summary: "guard " + g.ID + " (" + g.Tag + ") must not be breached by this change",
			Kind:    adh.FindingNFR,
			Class:   adh.FixableFinding,
			Ref:     g.ID,
		})
	}
	return out
}
