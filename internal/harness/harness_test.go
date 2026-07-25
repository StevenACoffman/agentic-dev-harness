package harness_test

import (
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/internal/gate"
	"github.com/StevenACoffman/agentic-dev-harness/internal/harness"
)

func TestClassify(t *testing.T) {
	if got := harness.Classify(true); got != harness.Lapse {
		t.Errorf("Classify(ruleExists=true) = %q, want lapse", got)
	}
	if got := harness.Classify(false); got != harness.Defect {
		t.Errorf("Classify(ruleExists=false) = %q, want defect", got)
	}
}

func TestAccept(t *testing.T) {
	tests := []struct {
		name                string
		cand, current, best float64
		want                gate.Action
	}{
		{
			name:    "strict improvement is new best",
			cand:    0.8,
			current: 0.7,
			best:    0.7,
			want:    gate.AcceptNewBest,
		},
		{name: "tie rejects", cand: 0.7, current: 0.7, best: 0.7, want: gate.Reject},
		{name: "regression rejects", cand: 0.6, current: 0.7, best: 0.7, want: gate.Reject},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := harness.Accept(tt.cand, tt.current, tt.best); got.Action != tt.want {
				t.Errorf("Accept = %q, want %q", got.Action, tt.want)
			}
		})
	}
}
