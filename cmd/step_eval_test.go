package cmd_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/state"
)

// TestStepDisposesEvaluation checks that a non-relay `step` at the evaluation
// stage runs the deterministic disposition (§19.2), not a mock model step.
func TestStepDisposesEvaluation(t *testing.T) {
	t.Chdir(t.TempDir())
	arc := adh.Arc{ID: "arc-0001", Stage: adh.StageEvaluation, Status: adh.StatusOpen}
	if err := state.Default().Save(&arc); err != nil {
		t.Fatalf("seed arc: %v", err)
	}
	if _, err := run(t, "step", arc.ID); err != nil {
		t.Fatalf("step: %v", err)
	}
	reloaded, err := state.Default().Get(arc.ID)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.Stage != adh.StageOps {
		t.Errorf("stage = %s, want ops (evaluation disposed, no model step)", reloaded.Stage)
	}
	if !strings.Contains(strings.Join(reloaded.History, "\n"), "no findings confirmed") {
		t.Errorf("history missing the disposition entry:\n%s", strings.Join(reloaded.History, "\n"))
	}
}
