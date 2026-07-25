package stage_test

import (
	"context"
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/authority"
	"github.com/StevenACoffman/agentic-dev-harness/internal/model"
	"github.com/StevenACoffman/agentic-dev-harness/internal/stage"
)

func TestAutoAdvances(t *testing.T) {
	tests := []struct {
		name  string
		stage adh.Stage
		level authority.Level
		want  bool
	}{
		{
			name:  "strategy at L2 advances",
			stage: adh.StageStrategy,
			level: authority.L2,
			want:  true,
		},
		{name: "strategy at L1 stops", stage: adh.StageStrategy, level: authority.L1, want: false},
		{name: "ops never advances", stage: adh.StageOps, level: authority.L4, want: false},
		{
			name:  "evaluation at L3 advances",
			stage: adh.StageEvaluation,
			level: authority.L3,
			want:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := stage.AutoAdvances(tt.stage, tt.level); got != tt.want {
				t.Errorf("AutoAdvances(%s, %s) = %v, want %v", tt.stage, tt.level, got, tt.want)
			}
		})
	}
}

func TestExecuteAdvances(t *testing.T) {
	arc := adh.Arc{ID: "arc-0001", Stage: adh.StageStrategy, Status: adh.StatusOpen}
	if err := stage.Execute(context.Background(), model.Mock{}, &arc); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if arc.Stage != adh.StageExecution {
		t.Errorf("stage after execute = %s, want execution", arc.Stage)
	}
	if len(arc.History) != 1 {
		t.Errorf("history len = %d, want 1", len(arc.History))
	}
	if arc.Resolution != adh.ResolutionChange {
		t.Errorf("resolution after strategy = %q, want change (strategy chooses)", arc.Resolution)
	}
}

func TestExecuteModelGate(t *testing.T) {
	arc := adh.Arc{ID: "arc-0001", Stage: adh.StageStrategy, Status: adh.StatusOpen}
	err := stage.Execute(context.Background(), model.Mock{Class: authority.ClassFast}, &arc)
	if adh.ErrorCode(err) != adh.EUNAUTHORIZED {
		t.Errorf("strategy on a fast-class model = %v, want EUNAUTHORIZED", err)
	}
	if arc.Stage != adh.StageStrategy {
		t.Errorf("stage advanced past a gated model call: %s", arc.Stage)
	}
}

func TestExecuteRefusesOps(t *testing.T) {
	arc := adh.Arc{ID: "arc-0001", Stage: adh.StageOps, Status: adh.StatusOpen}
	err := stage.Execute(context.Background(), model.Mock{}, &arc)
	if adh.ErrorCode(err) != adh.EINVALID {
		t.Errorf("Execute at ops = %v, want EINVALID (ops ships via close)", err)
	}
	if arc.Status != adh.StatusOpen {
		t.Errorf("status after refused ops = %s, want unchanged (open)", arc.Status)
	}
}
