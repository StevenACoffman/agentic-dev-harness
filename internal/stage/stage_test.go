package stage_test

import (
	"context"
	"errors"
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/authority"
	"github.com/StevenACoffman/agentic-dev-harness/internal/critic"
	"github.com/StevenACoffman/agentic-dev-harness/internal/model"
	"github.com/StevenACoffman/agentic-dev-harness/internal/stage"
)

// fakePrompter renders a trivial prompt, or returns err to prove the caller
// consults it (or does not, when a gate should short-circuit first). It ignores
// the critic grounding; a dedicated prompt test covers grounded rendering.
type fakePrompter struct{ err error }

func (f fakePrompter) Render(arc *adh.Arc, _ *critic.Grounding) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return "prompt for " + arc.ID + " at " + string(arc.Stage), nil
}

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
	judgment := authority.DefaultJudgmentRoles()
	if err := stage.Execute(
		context.Background(),
		model.Mock{},
		fakePrompter{},
		&arc,
		judgment,
	); err != nil {
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
	fast := model.Mock{Class: authority.ClassFast}
	err := stage.Execute(
		context.Background(),
		fast,
		fakePrompter{},
		&arc,
		authority.DefaultJudgmentRoles(),
	)
	if adh.ErrorCode(err) != adh.EUNAUTHORIZED {
		t.Errorf("strategy on a fast-class model = %v, want EUNAUTHORIZED", err)
	}
	if arc.Stage != adh.StageStrategy {
		t.Errorf("stage advanced past a gated model call: %s", arc.Stage)
	}
	// A judgment set that excludes strategy lets the same fast model through.
	loose := authority.JudgmentRoles{adh.StageCritic: true}
	if err := stage.Execute(context.Background(), fast, fakePrompter{}, &arc, loose); err != nil {
		t.Errorf("strategy off the judgment set on a fast model = %v, want nil", err)
	}
}

func TestExecuteRefusesOps(t *testing.T) {
	arc := adh.Arc{ID: "arc-0001", Stage: adh.StageOps, Status: adh.StatusOpen}
	err := stage.Execute(
		context.Background(),
		model.Mock{},
		fakePrompter{},
		&arc,
		authority.DefaultJudgmentRoles(),
	)
	if adh.ErrorCode(err) != adh.EINVALID {
		t.Errorf("Execute at ops = %v, want EINVALID (ops ships via close)", err)
	}
	if arc.Status != adh.StatusOpen {
		t.Errorf("status after refused ops = %s, want unchanged (open)", arc.Status)
	}
}

func TestRequestRefusesOps(t *testing.T) {
	arc := adh.Arc{ID: "arc-0001", Stage: adh.StageOps, Status: adh.StatusOpen}
	_, err := stage.Request(
		fakePrompter{},
		&arc,
		nil,
		authority.ClassReasoning,
		authority.DefaultJudgmentRoles(),
	)
	if adh.ErrorCode(err) != adh.EINVALID {
		t.Errorf("Request at ops = %v, want EINVALID", err)
	}
}

// TestRequestGatesBeforeRendering proves the model-gate short-circuits before the
// prompt is rendered: a fast model on a judgment role fails EUNAUTHORIZED, never
// reaching the prompter (whose Render would otherwise return errBoom).
func TestRequestGatesBeforeRendering(t *testing.T) {
	errBoom := errors.New("render must not run")
	arc := adh.Arc{ID: "arc-0001", Stage: adh.StageStrategy, Status: adh.StatusOpen}
	_, err := stage.Request(
		fakePrompter{err: errBoom},
		&arc,
		nil,
		authority.ClassFast,
		authority.DefaultJudgmentRoles(),
	)
	if adh.ErrorCode(err) != adh.EUNAUTHORIZED {
		t.Fatalf("Request on a fast judgment role = %v, want EUNAUTHORIZED", err)
	}
	if errors.Is(err, errBoom) {
		t.Error("Request rendered the prompt despite a gate failure")
	}
}

func TestApplyAdvances(t *testing.T) {
	arc := adh.Arc{ID: "arc-0001", Stage: adh.StageStrategy, Status: adh.StatusOpen}
	stage.Apply(&arc, model.Response{Text: "chose change"})
	if arc.Stage != adh.StageExecution {
		t.Errorf("stage after Apply = %s, want execution", arc.Stage)
	}
	if arc.Resolution != adh.ResolutionChange {
		t.Errorf("resolution after Apply = %q, want change", arc.Resolution)
	}
	if len(arc.History) != 1 {
		t.Errorf("history len = %d, want 1", len(arc.History))
	}
}
