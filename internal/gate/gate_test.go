package gate_test

import (
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/internal/gate"
)

func TestSelectScore(t *testing.T) {
	tests := []struct {
		name    string
		hard    float64
		soft    float64
		metric  gate.Metric
		weight  float64
		want    float64
		wantErr bool
	}{
		{name: "hard", hard: 0.8, soft: 0.5, metric: gate.Hard, want: 0.8},
		{name: "soft", hard: 0.8, soft: 0.5, metric: gate.Soft, want: 0.5},
		{name: "mixed even", hard: 1.0, soft: 0.0, metric: gate.Mixed, weight: 0.5, want: 0.5},
		{
			name:   "mixed weight clamps high",
			hard:   0.8,
			soft:   0.4,
			metric: gate.Mixed,
			weight: 2,
			want:   0.4,
		},
		{
			name:   "mixed weight clamps low",
			hard:   0.8,
			soft:   0.4,
			metric: gate.Mixed,
			weight: -1,
			want:   0.8,
		},
		{name: "unknown metric", metric: gate.Metric("bogus"), wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := gate.SelectScore(tt.hard, tt.soft, tt.metric, tt.weight)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("SelectScore() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("SelectScore() unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("SelectScore() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEvaluate(t *testing.T) {
	tests := []struct {
		name                 string
		cand, current, best  float64
		bestStep, globalStep int
		wantAction           gate.Action
		wantCurrent          float64
		wantBest             float64
		wantBestStep         int
	}{
		{
			name: "new best", cand: 90, current: 84, best: 84, bestStep: 3, globalStep: 7,
			wantAction: gate.AcceptNewBest, wantCurrent: 90, wantBest: 90, wantBestStep: 7,
		},
		{
			name:         "accept but not best",
			cand:         86,
			current:      84,
			best:         88,
			bestStep:     3,
			globalStep:   7,
			wantAction:   gate.Accept,
			wantCurrent:  86,
			wantBest:     88,
			wantBestStep: 3,
		},
		{
			name:         "reject on tie with current",
			cand:         84,
			current:      84,
			best:         88,
			bestStep:     3,
			globalStep:   7,
			wantAction:   gate.Reject,
			wantCurrent:  84,
			wantBest:     88,
			wantBestStep: 3,
		},
		{
			name:         "reject below current",
			cand:         80,
			current:      84,
			best:         88,
			bestStep:     3,
			globalStep:   7,
			wantAction:   gate.Reject,
			wantCurrent:  84,
			wantBest:     88,
			wantBestStep: 3,
		},
		{
			name:         "tie with best is not a new best",
			cand:         88,
			current:      84,
			best:         88,
			bestStep:     3,
			globalStep:   7,
			wantAction:   gate.Accept,
			wantCurrent:  88,
			wantBest:     88,
			wantBestStep: 3,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := gate.Evaluate(tt.cand, tt.current, tt.best, tt.bestStep, tt.globalStep)
			if got.Action != tt.wantAction {
				t.Errorf("Action = %q, want %q", got.Action, tt.wantAction)
			}
			if got.CurrentScore != tt.wantCurrent {
				t.Errorf("CurrentScore = %v, want %v", got.CurrentScore, tt.wantCurrent)
			}
			if got.BestScore != tt.wantBest {
				t.Errorf("BestScore = %v, want %v", got.BestScore, tt.wantBest)
			}
			if got.BestStep != tt.wantBestStep {
				t.Errorf("BestStep = %v, want %v", got.BestStep, tt.wantBestStep)
			}
		})
	}
}
