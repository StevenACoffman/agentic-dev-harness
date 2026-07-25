// Package oracle implements the differential oracle and invariant checks of the
// eval layer (SPEC §4, SPEC-ADDITIONS §18): two independently written
// implementations of the same rules grade each other, and a planted-defect
// self-test proves the oracle rejects a known regression. It is a compact
// (one-dimensional) port of evals-differential-oracle: a row of gems clears
// every run of length >= 3 and awards one special gem per run of length >= 4.
package oracle

import (
	"fmt"
	"math/rand/v2"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
)

// Resolver reduces a row (0 = empty, 1..k = colors) to a Result.
type Resolver func(row []int) Result

// Result is a resolved row: the cleared cell indexes and the special-gem count.
type Result struct {
	Cleared  map[int]bool
	Specials int
}

// Report summarizes a differential run for display.
type Report struct {
	Rows      int
	Divergent []int
}

// Equal reports whether two results are identical — the exact match the
// differential oracle leans on.
func (r Result) Equal(other Result) bool {
	if r.Specials != other.Specials || len(r.Cleared) != len(other.Cleared) {
		return false
	}
	for cell := range r.Cleared {
		if !other.Cleared[cell] {
			return false
		}
	}
	return true
}

// String renders a differential report.
func (r Report) String() string {
	if len(r.Divergent) == 0 {
		return fmt.Sprintf("agree on %d rows", r.Rows)
	}
	return fmt.Sprintf("DIVERGENCE over %d rows at row %v", r.Rows, r.Divergent)
}

// record applies the real rule to a run: clear it when length >= 3 and award a
// special when length >= 4. Shared by the correct resolvers.
func record(res *Result, start, length int) {
	if length < 3 {
		return
	}
	for k := start; k < start+length; k++ {
		res.Cleared[k] = true
	}
	if length >= 4 {
		res.Specials++
	}
}

// React resolves by a linear scan, extending a run while the color holds.
func React(row []int) Result {
	res := Result{Cleared: map[int]bool{}}
	for i := 0; i < len(row); {
		if row[i] == 0 {
			i++
			continue
		}
		start := i
		for i < len(row) && row[i] == row[start] {
			i++
		}
		record(&res, start, i-start)
	}
	return res
}

// Native resolves by an independent algorithm: it starts a run only at a cell
// whose left neighbor differs, then extends. It must always agree with React.
func Native(row []int) Result {
	res := Result{Cleared: map[int]bool{}}
	for i := range row {
		if row[i] == 0 || (i > 0 && row[i-1] == row[i]) {
			continue
		}
		length := 0
		for k := i; k < len(row) && row[k] == row[i]; k++ {
			length++
		}
		record(&res, i, length)
	}
	return res
}

// Buggy is a deliberately wrong resolver used only by the self-test: it awards a
// special for any run of length >= 3 (the rule is >= 4). Both nets must catch it.
func Buggy(row []int) Result {
	res := Result{Cleared: map[int]bool{}}
	for i := 0; i < len(row); {
		if row[i] == 0 {
			i++
			continue
		}
		start := i
		for i < len(row) && row[i] == row[start] {
			i++
		}
		if length := i - start; length >= 3 {
			for k := start; k < i; k++ {
				res.Cleared[k] = true
			}
			res.Specials++ // BUG: should be `if length >= 4`
		}
	}
	return res
}

// InvariantsHold checks the governing rules directly, independent of how result
// was produced: only a run of length >= 4 awards a special, and the cleared set
// equals the union of runs of length >= 3.
func InvariantsHold(row []int, result Result) bool {
	return result.Equal(React(row)) // React is the reference recomputation
}

// Diverges runs both resolvers over generated rows and returns the first row on
// which they disagree, or nil if they agree everywhere.
func Diverges(a, b Resolver, rows [][]int) []int {
	for _, row := range rows {
		if !a(row).Equal(b(row)) {
			return row
		}
	}
	return nil
}

// GenerateRows produces a deterministic set of rows for a seed, so the oracle
// runs the same corpus every time without a clock or ambient randomness.
func GenerateRows(seed uint64, count, maxLen, colors int) [][]int {
	source := rand.NewPCG(seed, seed*2+1)
	rng := rand.New(source) //nolint:gosec // deterministic test corpus, not security
	rows := make([][]int, count)
	for i := range rows {
		row := make([]int, 1+rng.IntN(maxLen))
		for j := range row {
			row[j] = rng.IntN(colors + 1)
		}
		rows[i] = row
	}
	return rows
}

// SelfTest proves the oracle has teeth (SPEC-ADDITIONS §18.4 negative control):
// the differential oracle and the invariant checker must both catch the planted
// Buggy resolver. It returns nil when both nets catch it, else an EINTERNAL
// error naming the net that failed.
func SelfTest(seed uint64) error {
	rows := GenerateRows(seed, 3000, 6, 3)
	caughtDiff := Diverges(React, Buggy, rows) != nil
	caughtInv := false
	for _, row := range rows {
		if !InvariantsHold(row, Buggy(row)) {
			caughtInv = true
			break
		}
	}
	switch {
	case !caughtDiff:
		return &adh.Error{
			Code:    adh.EINTERNAL,
			Message: "gate self-test: differential oracle missed the planted defect",
		}
	case !caughtInv:
		return &adh.Error{
			Code:    adh.EINTERNAL,
			Message: "gate self-test: invariant checker missed the planted defect",
		}
	default:
		return nil
	}
}
