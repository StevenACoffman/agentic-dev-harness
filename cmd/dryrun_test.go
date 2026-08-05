package cmd_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/cmd/root"
	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/state"
)

// TestApproveDryRunNeverSatisfiesGate is the safety invariant: even with the
// correct phrase, --dry-run must not satisfy the human gate (exit 4) and must
// leave the arc blocked. The agent has no dry-run route to self-grant approval.
func TestApproveDryRunNeverSatisfiesGate(t *testing.T) {
	t.Chdir(t.TempDir())
	id := strings.TrimSpace(mustRun(t, "arc", "new", "gate me"))
	mustRun(t, "run", id) // blocked at the ops gate, awaiting approval

	_, err := run(t, "--dry-run", "approve", "--phrase", id, id)
	var exit root.ExitError
	if !errors.As(err, &exit) || int(exit) != 4 {
		t.Fatalf(
			"approve --dry-run with the phrase = %v, want ExitError(4) (gate not satisfied)",
			err,
		)
	}
	arc, err := state.Default().Get(id)
	if err != nil {
		t.Fatalf("reload arc: %v", err)
	}
	if arc.Status != adh.StatusBlocked {
		t.Errorf(
			"approve --dry-run mutated the arc: status %s, want %s",
			arc.Status,
			adh.StatusBlocked,
		)
	}
}

// TestRejectDryRunDoesNotMutate previews a reject without returning the arc to
// Execution or reverting the working tree: the arc stays blocked at its gate.
func TestRejectDryRunDoesNotMutate(t *testing.T) {
	t.Chdir(t.TempDir())
	id := strings.TrimSpace(mustRun(t, "arc", "new", "reject me"))
	mustRun(t, "run", id) // blocked at the ops gate

	out, err := run(t, "--dry-run", "reject", id)
	if err != nil {
		t.Fatalf("reject --dry-run: %v", err)
	}
	if !strings.Contains(out, "would reject") {
		t.Errorf("reject --dry-run output = %q, want a \"would reject\" preview", out)
	}
	arc, err := state.Default().Get(id)
	if err != nil {
		t.Fatalf("reload arc: %v", err)
	}
	if arc.Status != adh.StatusBlocked || arc.Stage != adh.StageOps {
		t.Errorf(
			"reject --dry-run mutated the arc: %s/%s, want %s/%s",
			arc.Stage, arc.Status, adh.StageOps, adh.StatusBlocked,
		)
	}
}

// TestCloseDryRunDoesNotMutate previews a close-ready arc — every gate passes —
// without shipping, saving, or recording a metric: the arc stays open at ops.
func TestCloseDryRunDoesNotMutate(t *testing.T) {
	t.Chdir(t.TempDir())
	id := parkedAtOps(t)
	writeProof(t, id)

	out, err := run(t, "--dry-run", "close", "--proof", "manifest.json", id)
	if err != nil {
		t.Fatalf("close --dry-run: %v", err)
	}
	if !strings.Contains(out, "would close") {
		t.Errorf("close --dry-run output = %q, want a \"would close\" preview", out)
	}
	arc, err := state.Default().Get(id)
	if err != nil {
		t.Fatalf("reload arc: %v", err)
	}
	if arc.Status != adh.StatusOpen {
		t.Errorf("close --dry-run mutated the arc: status %s, want %s", arc.Status, adh.StatusOpen)
	}
	if _, err := os.Stat(filepath.Join(".adh", "metrics.json")); !os.IsNotExist(err) {
		t.Errorf("close --dry-run recorded a metric; the ledger should be absent, got err %v", err)
	}
}

// TestDryRunRefusedByMutationCommands checks that a command outside the
// approve/reject/close trio refuses --dry-run with DryRunUnsupportedError rather
// than silently ignoring the flag and mutating anyway. Each guard fires before
// any state access, so a placeholder arc id is enough to reach it.
func TestDryRunRefusedByMutationCommands(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		args []string
	}{
		{"run", []string{"run", "arc-0001"}},
		{"step", []string{"step", "arc-0001"}},
		{"eval", []string{"eval", "arc-0001"}},
		{"arc", []string{"arc", "new", "x"}},
		{"worker", []string{"worker", "requalify"}},
		{"proof create", []string{"proof", "create", "arc-0001", "f.txt"}},
		{"stage", []string{"strategy", "arc-0001"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			args := append([]string{"--dry-run"}, tc.args...)
			_, err := run(t, args...)
			var refused root.DryRunUnsupportedError
			if !errors.As(err, &refused) {
				t.Fatalf("%v under --dry-run = %v, want DryRunUnsupportedError", tc.args, err)
			}
		})
	}
}

// TestArcNewDryRunCreatesNothing is the no-mutation half of the guard: the refusal
// fires before the arc is written, so the store stays empty.
func TestArcNewDryRunCreatesNothing(t *testing.T) {
	t.Chdir(t.TempDir())
	if _, err := run(t, "--dry-run", "arc", "new", "should not persist"); err == nil {
		t.Fatal("arc new --dry-run should be refused, got nil error")
	}
	if out := mustRun(t, "arc", "list"); strings.Contains(out, "should not persist") {
		t.Errorf("arc new --dry-run created an arc:\n%s", out)
	}
}
