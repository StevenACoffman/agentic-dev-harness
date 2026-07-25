package oracle_test

import (
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/internal/oracle"
)

func TestReactNativeAgreeOnCorpus(t *testing.T) {
	boards := oracle.GenerateBoards(1234, 3000, 4, 4, 3)
	if div := oracle.Diverges(oracle.React, oracle.Native, boards); div != nil {
		t.Errorf("React and Native diverged on %v", div)
	}
}

func TestResolverRules(t *testing.T) {
	tests := []struct {
		name         string
		board        oracle.Board
		wantSpecials int
		wantCleared  int
	}{
		{
			name:         "horizontal run of three clears no special",
			board:        oracle.Board{{1, 1, 1}},
			wantSpecials: 0,
			wantCleared:  3,
		},
		{
			name:         "horizontal run of four awards special",
			board:        oracle.Board{{2, 2, 2, 2}},
			wantSpecials: 1,
			wantCleared:  4,
		},
		{
			name:         "vertical run of three clears no special",
			board:        oracle.Board{{1}, {1}, {1}},
			wantSpecials: 0,
			wantCleared:  3,
		},
		{
			name:         "short run untouched",
			board:        oracle.Board{{3, 3, 1}},
			wantSpecials: 0,
			wantCleared:  0,
		},
		{
			name:         "horizontal and vertical runs both count",
			board:        oracle.Board{{1, 1, 1}, {2, 0, 0}, {2, 0, 0}, {2, 0, 0}},
			wantSpecials: 0,
			wantCleared:  6,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := oracle.React(tt.board)
			if got.Specials != tt.wantSpecials || len(got.Cleared) != tt.wantCleared {
				t.Errorf("React(%v) = specials %d cleared %d, want %d/%d",
					tt.board, got.Specials, len(got.Cleared), tt.wantSpecials, tt.wantCleared)
			}
			if !oracle.Native(tt.board).Equal(got) {
				t.Errorf("Native disagreed with React on %v", tt.board)
			}
		})
	}
}

func TestSelfTestCatchesPlantedBug(t *testing.T) {
	if err := oracle.SelfTest(7); err != nil {
		t.Errorf("SelfTest should pass (both nets catch the bug): %v", err)
	}
}

func TestInvariantsAreIndependent(t *testing.T) {
	board := oracle.Board{{1, 1, 1}} // one run of length 3: no special, cleared {0,1,2}
	cell := func(r, c int) oracle.Cell { return oracle.Cell{Row: r, Col: c} }

	// A hand-crafted correct result — no resolver produced it — must pass, which
	// shows the checker recomputes the rules itself rather than deferring.
	correct := oracle.Result{
		Cleared:  map[oracle.Cell]bool{cell(0, 0): true, cell(0, 1): true, cell(0, 2): true},
		Specials: 0,
	}
	if !oracle.InvariantsHold(board, correct) {
		t.Errorf("independent checker rejected a rule-correct result")
	}

	// A special awarded for a run of 3 violates the special-source rule.
	tooManySpecials := oracle.Result{
		Cleared:  map[oracle.Cell]bool{cell(0, 0): true, cell(0, 1): true, cell(0, 2): true},
		Specials: 1,
	}
	if oracle.InvariantsHold(board, tooManySpecials) {
		t.Errorf("checker must convict a special awarded for a run of 3")
	}

	// A wrong cleared set violates the cleared-cell rule.
	wrongCleared := oracle.Result{
		Cleared:  map[oracle.Cell]bool{cell(0, 0): true},
		Specials: 0,
	}
	if oracle.InvariantsHold(board, wrongCleared) {
		t.Errorf("checker must convict a wrong cleared set")
	}
}

func TestInvariantsCatchBuggy(t *testing.T) {
	board := oracle.Board{{2, 2, 2}} // Buggy awards a special here; the real rule does not
	if oracle.InvariantsHold(board, oracle.Buggy(board)) {
		t.Errorf("independent checker should convict the buggy resolver on a run of 3")
	}
}
