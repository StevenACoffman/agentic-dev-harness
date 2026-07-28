package cmd_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/state"
)

// TestRunDisposesEvaluationDeterministically checks that `run` routes the
// evaluation stage through the deterministic disposition (§19.2), not a model
// step: an arc seeded at evaluation with no findings advances to the ops gate and
// records the disposition in history.
func TestRunDisposesEvaluationDeterministically(t *testing.T) {
	t.Chdir(t.TempDir())
	arc := adh.Arc{ID: "arc-0001", Stage: adh.StageEvaluation, Status: adh.StatusOpen}
	if err := state.Default().Save(&arc); err != nil {
		t.Fatalf("seed arc: %v", err)
	}
	if _, err := run(t, "run", arc.ID); err != nil {
		t.Fatalf("run: %v", err)
	}
	reloaded, err := state.Default().Get(arc.ID)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.Stage != adh.StageOps {
		t.Errorf("stage = %s, want ops (disposition then parked at the ship gate)", reloaded.Stage)
	}
	if reloaded.Status != adh.StatusBlocked {
		t.Errorf("status = %s, want blocked at ops", reloaded.Status)
	}
	joined := strings.Join(reloaded.History, "\n")
	if !strings.Contains(joined, "no findings confirmed") {
		t.Errorf("history missing the deterministic disposition entry:\n%s", joined)
	}
}
