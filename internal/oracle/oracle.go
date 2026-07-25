// Package oracle implements the differential oracle and invariant checks of the
// eval layer (SPEC §4, SPEC-ADDITIONS §18): two independently written
// implementations of the same match rules grade each other, and a
// planted-defect self-test proves the oracle rejects a known regression. It is a
// two-dimensional port of evals-differential-oracle: on a board of gems every
// maximal horizontal or vertical run of length >= 3 clears, and a run of length
// >= 4 awards one special gem.
//
// Two nets cover two failure modes. The differential net (React vs Native) uses
// independent run-enumeration strategies, so an enumeration bug in one surfaces
// as a disagreement. The invariant net recomputes the rule (special only for a
// run of >= 4) directly, so a rule bug is convicted even when both resolvers
// agree.
package oracle

import (
	"fmt"
	"math/rand/v2"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
)

// selfTestBoards is the negative-control corpus size; small boards over many
// samples reliably contain runs of exactly 3 (where the planted bug shows).
const selfTestBoards = 3000

// Board is a grid of gems: 0 is empty, 1..k are colors. Boards are rectangular.
type Board [][]int

// Cell is a board coordinate, used as the cleared-set key.
type Cell struct {
	Row int
	Col int
}

// Resolver reduces a board to a Result.
type Resolver func(board Board) Result

// Result is a resolved board: the cleared cells and the special-gem count.
type Result struct {
	Cleared  map[Cell]bool
	Specials int
}

// Report summarizes a differential run for display.
type Report struct {
	Boards    int
	Divergent Board
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
	if r.Divergent == nil {
		return fmt.Sprintf("agree on %d boards", r.Boards)
	}
	return fmt.Sprintf("DIVERGENCE over %d boards at %v", r.Boards, r.Divergent)
}

// React resolves by a while-scan over rows then columns, awarding a special for
// a run of length >= 4 (the real rule).
func React(board Board) Result {
	return scanFrom(board, 4)
}

// Buggy is a deliberately wrong resolver used only by the self-test: it awards a
// special for any run of length >= 3 (the rule is >= 4). Both nets must catch it.
func Buggy(board Board) Result {
	return scanFrom(board, 3)
}

// Native resolves by an independent algorithm: it starts a run only at a cell
// whose left (or upper) neighbor differs, then extends. It must always agree
// with React.
func Native(board Board) Result {
	res := Result{Cleared: map[Cell]bool{}}
	for r := range board {
		for c := range board[r] {
			nativeCell(&res, board, r, c)
		}
	}
	return res
}

// InvariantsHold checks the governing rules directly, recomputing the runs
// itself and applying the special rule independently of any resolver, so it can
// convict a result both implementations agree on. It asserts three rules (a port
// of evals-differential-oracle's check_all): a special is awarded only by a run
// of length >= 4, the cleared set equals the union of runs of length >= 3, and
// the special count stays within [0, number of runs].
func InvariantsHold(board Board, result Result) bool {
	runs := scanRuns(board)
	special4 := 0
	cleared := make(map[Cell]bool)
	for _, run := range runs {
		if len(run) >= 4 {
			special4++
		}
		for _, cell := range run {
			cleared[cell] = true
		}
	}
	if result.Specials != special4 || !equalCellSet(result.Cleared, cleared) {
		return false
	}
	return result.Specials >= 0 && result.Specials <= len(runs)
}

// Diverges runs both resolvers over the boards and returns the first board on
// which they disagree, or nil if they agree everywhere.
func Diverges(a, b Resolver, boards []Board) Board {
	for _, board := range boards {
		if !a(board).Equal(b(board)) {
			return board
		}
	}
	return nil
}

// GenerateBoards produces a deterministic set of rectangular boards for a seed,
// so the oracle runs the same corpus every time without a clock or ambient
// randomness.
func GenerateBoards(seed uint64, count, maxRows, maxCols, colors int) []Board {
	source := rand.NewPCG(seed, seed*2+1)
	rng := rand.New(source) //nolint:gosec // deterministic test corpus, not security
	boards := make([]Board, count)
	for i := range boards {
		rows := 1 + rng.IntN(maxRows)
		cols := 1 + rng.IntN(maxCols)
		board := make(Board, rows)
		for r := range board {
			board[r] = make([]int, cols)
			for c := range board[r] {
				board[r][c] = rng.IntN(colors + 1)
			}
		}
		boards[i] = board
	}
	return boards
}

// SelfTest proves the oracle has teeth (SPEC-ADDITIONS §18.4 negative control):
// the differential oracle and the invariant checker must both catch the planted
// Buggy resolver. It returns nil when both nets catch it, else an EINTERNAL
// error naming the net that failed.
func SelfTest(seed uint64) error {
	boards := GenerateBoards(seed, selfTestBoards, 4, 4, 3)
	caughtDiff := Diverges(React, Buggy, boards) != nil
	caughtInv := false
	for _, board := range boards {
		if !InvariantsHold(board, Buggy(board)) {
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

// scanFrom clears every maximal run of length >= 3 and awards a special for runs
// of length >= specialMin. React passes 4 (the rule); Buggy passes 3.
func scanFrom(board Board, specialMin int) Result {
	res := Result{Cleared: map[Cell]bool{}}
	for _, run := range scanRuns(board) {
		for _, cell := range run {
			res.Cleared[cell] = true
		}
		if len(run) >= specialMin {
			res.Specials++
		}
	}
	return res
}

// scanRuns enumerates every maximal same-color run of length >= 3, horizontal
// then vertical, by a linear while-scan.
func scanRuns(board Board) [][]Cell {
	var runs [][]Cell
	for r := range board {
		runs = appendRowRuns(runs, board, r)
	}
	for c := range cols(board) {
		runs = appendColRuns(runs, board, c)
	}
	return runs
}

func appendRowRuns(runs [][]Cell, board Board, r int) [][]Cell {
	row := board[r]
	for c := 0; c < len(row); {
		if row[c] == 0 {
			c++
			continue
		}
		color := row[c]
		var cells []Cell
		for c < len(row) && row[c] == color {
			cells = append(cells, Cell{Row: r, Col: c})
			c++
		}
		if len(cells) >= 3 {
			runs = append(runs, cells)
		}
	}
	return runs
}

func appendColRuns(runs [][]Cell, board Board, c int) [][]Cell {
	for r := 0; r < len(board); {
		if board[r][c] == 0 {
			r++
			continue
		}
		color := board[r][c]
		var cells []Cell
		for r < len(board) && board[r][c] == color {
			cells = append(cells, Cell{Row: r, Col: c})
			r++
		}
		if len(cells) >= 3 {
			runs = append(runs, cells)
		}
	}
	return runs
}

// nativeCell starts and extends a horizontal and a vertical run at (r, c) only
// when the cell begins one (its left or upper neighbor differs) — the
// independent enumeration the differential oracle checks against React.
func nativeCell(res *Result, board Board, r, c int) {
	color := board[r][c]
	if color == 0 {
		return
	}
	if c == 0 || board[r][c-1] != color {
		clearRun(res, horizontalRun(board, r, c), 4)
	}
	if r == 0 || board[r-1][c] != color {
		clearRun(res, verticalRun(board, r, c), 4)
	}
}

func horizontalRun(board Board, r, c int) []Cell {
	color := board[r][c]
	var cells []Cell
	for k := c; k < len(board[r]) && board[r][k] == color; k++ {
		cells = append(cells, Cell{Row: r, Col: k})
	}
	return cells
}

func verticalRun(board Board, r, c int) []Cell {
	color := board[r][c]
	var cells []Cell
	for k := r; k < len(board) && board[k][c] == color; k++ {
		cells = append(cells, Cell{Row: k, Col: c})
	}
	return cells
}

func clearRun(res *Result, cells []Cell, specialMin int) {
	if len(cells) < 3 {
		return
	}
	for _, cell := range cells {
		res.Cleared[cell] = true
	}
	if len(cells) >= specialMin {
		res.Specials++
	}
}

func cols(board Board) int {
	if len(board) == 0 {
		return 0
	}
	return len(board[0])
}

func equalCellSet(a, b map[Cell]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for cell := range a {
		if !b[cell] {
			return false
		}
	}
	return true
}
