package cmd_test

import (
	"errors"
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/cmd/root"
	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/failures"
	"github.com/StevenACoffman/agentic-dev-harness/internal/state"
)

func seedEvalArc(t *testing.T, findings []adh.Finding) string {
	t.Helper()
	arc := adh.Arc{
		ID:         "arc-0001",
		Stage:      adh.StageEvaluation,
		Status:     adh.StatusOpen,
		Resolution: adh.ResolutionChange,
		Findings:   findings,
	}
	if err := state.Default().Save(&arc); err != nil {
		t.Fatalf("seed eval arc: %v", err)
	}
	return arc.ID
}

func TestEvalContractFindingConfirmedReturnsToExecution(t *testing.T) {
	t.Chdir(t.TempDir())
	// A contract finding naming a proof manifest that does not exist: the named
	// proof cannot be verified, so the finding is confirmed.
	id := seedEvalArc(t, []adh.Finding{
		{
			Summary: "proof does not cover the new path",
			Kind:    adh.FindingContract,
			Ref:     ".adh/proof/missing.json",
		},
	})

	_, err := run(t, "eval", id)
	var exit root.ExitError
	if !errors.As(err, &exit) || int(exit) != 8 {
		t.Fatalf("confirmed contract finding = %v, want ExitError(8)", err)
	}
	arc, getErr := state.Default().Get(id)
	if getErr != nil {
		t.Fatalf("reload arc: %v", getErr)
	}
	if arc.Stage != adh.StageExecution {
		t.Errorf("stage = %s, want execution (confirmed finding returns the arc)", arc.Stage)
	}
	if len(arc.Findings) != 0 {
		t.Errorf("findings not cleared after disposition: %+v", arc.Findings)
	}
	notes, _ := failures.Load(failures.RegistryFile)
	if len(notes) != 1 {
		t.Errorf("failure registry = %v, want one confirmed entry", notes)
	}
}

func TestEvalUnconfirmedFindingAdvancesToOps(t *testing.T) {
	t.Chdir(t.TempDir())
	// An oracle finding: the real differential oracle agrees (no divergence), so
	// nothing confirms it — it becomes a lesson candidate and the arc advances.
	id := seedEvalArc(t, []adh.Finding{
		{Summary: "suspected clear-set divergence", Kind: adh.FindingOracle, Ref: "corpus"},
	})

	if _, err := run(t, "eval", id); err != nil {
		t.Fatalf("unconfirmed finding should not error: %v", err)
	}
	arc, getErr := state.Default().Get(id)
	if getErr != nil {
		t.Fatalf("reload arc: %v", getErr)
	}
	if arc.Stage != adh.StageOps {
		t.Errorf("stage = %s, want ops (unconfirmed finding does not block)", arc.Stage)
	}
	candidates, _ := failures.Load(failures.CandidatesFile)
	if len(candidates) != 1 {
		t.Errorf("lesson candidates = %v, want one", candidates)
	}
	if _, err := failures.Load(failures.RegistryFile); err != nil {
		t.Fatalf("load registry: %v", err)
	}
	if notes, _ := failures.Load(failures.RegistryFile); len(notes) != 0 {
		t.Errorf("failure registry = %v, want empty (nothing confirmed)", notes)
	}
}

func TestEvalNoFindingsAdvancesToOps(t *testing.T) {
	t.Chdir(t.TempDir())
	id := seedEvalArc(t, nil)
	if _, err := run(t, "eval", id); err != nil {
		t.Fatalf("clean review should not error: %v", err)
	}
	arc, err := state.Default().Get(id)
	if err != nil {
		t.Fatalf("reload arc: %v", err)
	}
	if arc.Stage != adh.StageOps {
		t.Errorf("stage = %s, want ops", arc.Stage)
	}
}

func TestEvalRefusesNonEvaluationStage(t *testing.T) {
	t.Chdir(t.TempDir())
	arc := adh.Arc{ID: "arc-0001", Stage: adh.StageStrategy, Status: adh.StatusOpen}
	if err := state.Default().Save(&arc); err != nil {
		t.Fatalf("seed arc: %v", err)
	}
	if _, err := run(t, "eval", arc.ID); err == nil {
		t.Error("eval on a non-evaluation arc should error")
	}
}
