package cmd_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/state"
)

func TestStageCommandRunsAtItsStage(t *testing.T) {
	t.Chdir(t.TempDir())
	arc := adh.Arc{ID: "arc-0001", Stage: adh.StageStrategy, Status: adh.StatusOpen}
	if err := state.Default().Save(&arc); err != nil {
		t.Fatalf("seed arc: %v", err)
	}
	if _, err := run(t, "strategy", arc.ID); err != nil {
		t.Fatalf("strategy: %v", err)
	}
	reloaded, err := state.Default().Get(arc.ID)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.Stage != adh.StageExecution {
		t.Errorf("stage after `strategy` = %s, want execution", reloaded.Stage)
	}
}

func TestStageCommandRefusesWrongStage(t *testing.T) {
	t.Chdir(t.TempDir())
	arc := adh.Arc{ID: "arc-0001", Stage: adh.StageStrategy, Status: adh.StatusOpen}
	if err := state.Default().Save(&arc); err != nil {
		t.Fatalf("seed arc: %v", err)
	}
	// `execute` on an arc still at strategy must refuse.
	if _, err := run(t, "execute", arc.ID); err == nil {
		t.Error("`execute` on a strategy-stage arc should error")
	}
	reloaded, err := state.Default().Get(arc.ID)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.Stage != adh.StageStrategy {
		t.Errorf("refused stage command still advanced the arc: %s", reloaded.Stage)
	}
}

func TestOpsReportsShipGate(t *testing.T) {
	t.Chdir(t.TempDir())
	arc := adh.Arc{ID: "arc-0001", Stage: adh.StageOps, Status: adh.StatusOpen}
	if err := state.Default().Save(&arc); err != nil {
		t.Fatalf("seed arc: %v", err)
	}
	out, err := run(t, "ops", arc.ID)
	if err != nil {
		t.Fatalf("ops: %v", err)
	}
	if !strings.Contains(out, "approve") || !strings.Contains(out, "close") {
		t.Errorf("ops report missing the approve/close guidance:\n%s", out)
	}
}
