package cmd_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/cmd/root"
	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/evaluation"
	"github.com/StevenACoffman/agentic-dev-harness/internal/state"
)

// seedFailingEvalArc puts an arc at Evaluation with one contract finding that
// names an absent proof manifest, so adjudication confirms it (exit 8) — the
// deterministic failing-finding used to exercise the fail-back-to-execution edge.
// reworks seeds the arc's spent rework budget.
func seedFailingEvalArc(t *testing.T, reworks int) string {
	t.Helper()
	arc := adh.Arc{
		ID:      "arc-0001",
		Stage:   adh.StageEvaluation,
		Status:  adh.StatusOpen,
		Reworks: reworks,
		Findings: []adh.Finding{
			{
				Summary: "proof does not hold",
				Kind:    adh.FindingContract,
				Ref:     ".adh/proof/missing.json",
			},
		},
	}
	if err := state.Default().Save(&arc); err != nil {
		t.Fatalf("seed failing eval arc: %v", err)
	}
	return arc.ID
}

// TestEvalConfirmedRework: within budget, a confirmed finding returns the arc to
// Execution (exit 8, the contract gate) and increments Reworks — it does not fail.
func TestEvalConfirmedRework(t *testing.T) {
	t.Chdir(t.TempDir())
	id := seedFailingEvalArc(t, 0)

	out, err := run(t, "eval", "--jsonl", id)
	assertError(t, out, err, 8, string(adh.FindingContract))

	arc, gerr := state.Default().Get(id)
	if gerr != nil {
		t.Fatalf("reload arc: %v", gerr)
	}
	if arc.Stage != adh.StageExecution || arc.Status != adh.StatusOpen {
		t.Errorf("(stage, status) = (%s, %s), want (execution, open) — a rework",
			arc.Stage, arc.Status)
	}
	if arc.Reworks != 1 {
		t.Errorf("reworks = %d, want 1", arc.Reworks)
	}
}

// TestEvalFailsTerminallyPastBudget: with the rework budget already spent, the
// same confirmed finding fails the arc terminally (StatusFailed) instead of
// looping — still exit 8 by kind, the durable status is the machine signal.
func TestEvalFailsTerminallyPastBudget(t *testing.T) {
	t.Chdir(t.TempDir())
	id := seedFailingEvalArc(t, evaluation.DefaultMaxReworks)

	out, err := run(t, "eval", "--jsonl", id)
	assertError(t, out, err, 8, string(adh.FindingContract))

	arc, gerr := state.Default().Get(id)
	if gerr != nil {
		t.Fatalf("reload arc: %v", gerr)
	}
	if arc.Status != adh.StatusFailed {
		t.Errorf("status = %s, want failed (rework budget spent)", arc.Status)
	}
}

// TestEvalHonorsConfiguredMaxReworks: a repo config lowering the rework budget to
// 1 fails an arc terminally at Reworks=1, where the built-in default (2) would
// still rework — proving [evaluation] max_reworks is threaded through eval.
func TestEvalHonorsConfiguredMaxReworks(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.MkdirAll(".adh", 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(".adh", "config.toml"),
		[]byte("[evaluation]\nmax_reworks = 1\n"),
		0o600,
	); err != nil {
		t.Fatalf("write config: %v", err)
	}
	id := seedFailingEvalArc(t, 1) // one rework already spent

	out, err := run(t, "eval", "--jsonl", id)
	assertError(t, out, err, 8, string(adh.FindingContract))

	arc, gerr := state.Default().Get(id)
	if gerr != nil {
		t.Fatalf("reload arc: %v", gerr)
	}
	if arc.Status != adh.StatusFailed {
		t.Errorf("status = %s, want failed at the configured budget of 1", arc.Status)
	}
}

// TestRunDrivesToTerminalFail: a non-relay run of an arc whose rework budget is
// spent drives evaluation, fails the arc terminally, and reports an error outcome
// (reason failed, exit 1) so an autonomous drive stops for a human.
func TestRunDrivesToTerminalFail(t *testing.T) {
	t.Chdir(t.TempDir())
	id := seedFailingEvalArc(t, evaluation.DefaultMaxReworks)

	out, err := run(t, "run", "--jsonl", id)
	assertError(t, out, err, 1, root.ReasonFailed)

	arc, gerr := state.Default().Get(id)
	if gerr != nil {
		t.Fatalf("reload arc: %v", gerr)
	}
	if arc.Status != adh.StatusFailed {
		t.Errorf("status = %s, want failed after the drive gave up", arc.Status)
	}
}
