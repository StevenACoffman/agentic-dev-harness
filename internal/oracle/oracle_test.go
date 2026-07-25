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
