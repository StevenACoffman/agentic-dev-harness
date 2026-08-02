package oracle_test

import (
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/internal/oracle"
)

func TestDiffOutputs(t *testing.T) {
	tests := []struct {
		name      string
		reference string
		candidate string
		wantLine  int // 0 = no divergence
	}{
		{"identical", "a\nb\nc", "a\nb\nc", 0},
		{"trailing newline ignored", "a\nb\n", "a\nb", 0},
		{"both empty", "", "", 0},
		{"differ at line 2", "a\nb\nc", "a\nX\nc", 2},
		{"candidate has an extra line", "a\nb", "a\nb\nc", 3},
		{"reference has an extra line", "a\nb\nc", "a\nb", 3},
		{"differ at first line", "x", "y", 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			div := oracle.DiffOutputs(tt.reference, tt.candidate)
			switch {
			case tt.wantLine == 0 && div != nil:
				t.Errorf("DiffOutputs = %+v, want match (nil)", div)
			case tt.wantLine != 0 && div == nil:
				t.Errorf("DiffOutputs = nil, want a divergence at line %d", tt.wantLine)
			case div != nil && div.Line != tt.wantLine:
				t.Errorf("divergence line = %d, want %d", div.Line, tt.wantLine)
			}
		})
	}
}
