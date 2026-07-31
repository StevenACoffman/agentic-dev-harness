package cmd_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/cmd/root"
	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/state"
)

func TestRunRelayEmitsAndParks(t *testing.T) {
	t.Chdir(t.TempDir())
	arc := adh.Arc{ID: "arc-0001", Stage: adh.StageStrategy, Status: adh.StatusOpen}
	if err := state.Default().Save(&arc); err != nil {
		t.Fatalf("seed: %v", err)
	}
	// No pending turn, no reply: run --relay emits the current stage and parks.
	recs := jsonLines(t, mustRun(t, "run", "--relay", "--jsonl", "arc-0001"))
	d := okData(t, recs[len(recs)-1])
	if d["status"] != "awaiting" || d["stage"] != "strategy" || d["prompt"] == "" {
		t.Errorf("emit outcome data = %v, want awaiting a strategy prompt", d)
	}
	got, err := state.Default().Get("arc-0001")
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got.Pending == nil || got.Pending.Stage != adh.StageStrategy {
		t.Errorf("run --relay did not park a strategy turn: %+v", got.Pending)
	}
}

func TestRunRelayResumeChainsThroughEvalToOpsGate(t *testing.T) {
	t.Chdir(t.TempDir())
	// A critic turn is parked; resuming it with a clean review advances the arc to
	// evaluation, which run adjudicates inline, reaching the ops gate — all in one
	// call. This is the collapse of emit → resume → eval the skill otherwise juggles.
	arc := adh.Arc{
		ID:      "arc-0001",
		Stage:   adh.StageCritic,
		Status:  adh.StatusOpen,
		Pending: &adh.Pending{Stage: adh.StageCritic, Prompt: "review the change"},
	}
	if err := state.Default().Save(&arc); err != nil {
		t.Fatalf("seed: %v", err)
	}
	reply := filepath.Join(t.TempDir(), "findings.json")
	if err := os.WriteFile(reply, []byte(`{"findings":[]}`), 0o600); err != nil {
		t.Fatalf("write reply: %v", err)
	}

	out := mustRun(t, "run", "--relay", "--jsonl", "--response", reply, "arc-0001")
	recs := jsonLines(t, out)
	last := recs[len(recs)-1]
	// run parks at the ops gate (exit 0), so the terminal outcome is blocked/at_ops.
	if last["status"] != root.StatusBlocked || last["reason"] != root.ReasonAtOps {
		t.Errorf("terminal outcome = %v, want blocked at_ops", last)
	}
	got, err := state.Default().Get("arc-0001")
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got.Stage != adh.StageOps || got.Status != adh.StatusBlocked {
		t.Errorf("arc at %s/%s, want ops/blocked after the chained relay", got.Stage, got.Status)
	}
}

func TestRunRelayResumeWithoutPendingErrors(t *testing.T) {
	t.Chdir(t.TempDir())
	arc := adh.Arc{ID: "arc-0001", Stage: adh.StageStrategy, Status: adh.StatusOpen}
	if err := state.Default().Save(&arc); err != nil {
		t.Fatalf("seed: %v", err)
	}
	reply := filepath.Join(t.TempDir(), "r.txt")
	if err := os.WriteFile(reply, []byte("a reply"), 0o600); err != nil {
		t.Fatalf("write reply: %v", err)
	}
	// A reply with no parked turn is refused — it cannot advance the wrong stage.
	if _, err := run(t, "run", "--relay", "--response", reply, "arc-0001"); err == nil {
		t.Error("run --relay --response with no pending turn should error")
	}
}
