package oracle_test

import (
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/internal/oracle"
)

func TestReactNativeAgreeOnCorpus(t *testing.T) {
	rows := oracle.GenerateRows(1234, 3000, 6, 3)
	if div := oracle.Diverges(oracle.React, oracle.Native, rows); div != nil {
		t.Errorf("React and Native diverged on %v", div)
	}
}

func TestResolverRules(t *testing.T) {
	tests := []struct {
		name         string
		row          []int
		wantSpecials int
		wantCleared  int
	}{
		{
			name:         "run of three clears no special",
			row:          []int{1, 1, 1},
			wantSpecials: 0,
			wantCleared:  3,
		},
		{
			name:         "run of four awards special",
			row:          []int{2, 2, 2, 2},
			wantSpecials: 1,
			wantCleared:  4,
		},
		{name: "short run untouched", row: []int{3, 3, 1}, wantSpecials: 0, wantCleared: 0},
		{name: "two runs", row: []int{1, 1, 1, 0, 2, 2, 2, 2}, wantSpecials: 1, wantCleared: 7},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := oracle.React(tt.row)
			if got.Specials != tt.wantSpecials || len(got.Cleared) != tt.wantCleared {
				t.Errorf("React(%v) = specials %d cleared %d, want %d/%d",
					tt.row, got.Specials, len(got.Cleared), tt.wantSpecials, tt.wantCleared)
			}
			if !oracle.Native(tt.row).Equal(got) {
				t.Errorf("Native disagreed with React on %v", tt.row)
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
	row := []int{1, 1, 1} // one run of length 3: no special, cleared {0,1,2}

	// A hand-crafted correct result — no resolver produced it — must pass, which
	// shows the checker recomputes the rules itself rather than deferring.
	correct := oracle.Result{Cleared: map[int]bool{0: true, 1: true, 2: true}, Specials: 0}
	if !oracle.InvariantsHold(row, correct) {
		t.Errorf("independent checker rejected a rule-correct result")
	}

	// A special awarded for a run of 3 violates the special-source rule.
	tooManySpecials := oracle.Result{Cleared: map[int]bool{0: true, 1: true, 2: true}, Specials: 1}
	if oracle.InvariantsHold(row, tooManySpecials) {
		t.Errorf("checker must convict a special awarded for a run of 3")
	}

	// A wrong cleared set violates the cleared-cell rule.
	wrongCleared := oracle.Result{Cleared: map[int]bool{0: true}, Specials: 0}
	if oracle.InvariantsHold(row, wrongCleared) {
		t.Errorf("checker must convict a wrong cleared set")
	}
}

func TestInvariantsCatchBuggy(t *testing.T) {
	row := []int{2, 2, 2} // Buggy awards a special here; the real rule does not
	if oracle.InvariantsHold(row, oracle.Buggy(row)) {
		t.Errorf("independent checker should convict the buggy resolver on a run of 3")
	}
}
