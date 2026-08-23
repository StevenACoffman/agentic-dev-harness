package evaluation

import (
	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/skillet/identity"
)

// BarHash fingerprints an acceptance bar for pre-registration (§19.2).
//
// Requires: nothing; an empty bar hashes like any other string.
// Ensures: identity.Hash, the same fingerprint every other artifact in this family
// carries, so a bar hash is comparable to one written by any other tool. Pure.
func BarHash(bar string) string { return identity.Hash(bar) }

// BarMoved reports whether the acceptance bar in force differs from the one the arc
// was planned against.
//
// Requires: nothing.
// Ensures: **false when the arc records no bar**, which is an arc planned before
// pre-registration shipped or one that never reached Strategy. That case is *unknown*
// and reporting it as drift would fire on every pre-existing arc, which teaches a
// reader to ignore the report — the failure a check like this most easily causes.
// Pure.
func BarMoved(arc *adh.Arc, current string) bool {
	if arc.Bar == "" {
		return false
	}
	return arc.Bar != BarHash(current)
}
