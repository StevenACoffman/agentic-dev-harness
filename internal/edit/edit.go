// Package edit holds the pure guards an optimize/apply step enforces before
// committing a harness edit (SPEC-ADDITIONS §18.2): the size budget and the
// hash-based no-op check. Ported from skillsaw's edit; both are pure, no I/O.
package edit

import "github.com/StevenACoffman/skillet/identity"

// WithinSizeBudget reports whether an edited artifact of newBytes stays within
// ratio × origBytes (adh's default ratio is 1.5). A non-positive origBytes
// admits only an empty result, since any growth from nothing is unbounded.
func WithinSizeBudget(origBytes, newBytes int, ratio float64) bool {
	if origBytes <= 0 {
		return newBytes == 0
	}
	return float64(newBytes) <= float64(origBytes)*ratio
}

// IsNoOp reports whether before and after are the same artifact under the
// content identity used across adh (identity.Hash). An edit that leaves the hash
// unchanged did nothing and need not be re-evaluated.
func IsNoOp(before, after string) bool {
	return identity.Hash(before) == identity.Hash(after)
}
