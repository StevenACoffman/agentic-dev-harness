package cmd_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/cmd"
	"github.com/StevenACoffman/agentic-dev-harness/cmd/root"
	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/consolidate"
	"github.com/StevenACoffman/agentic-dev-harness/internal/state"
)

func TestHarnessEvalDispatch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "skill.md")
	doc := "# Skill\nIf the build fails, retry.\n## Boundary\nNot for zero-to-one work.\n"
	if err := os.WriteFile(path, []byte(doc), 0o600); err != nil {
		t.Fatalf("write artifact: %v", err)
	}
	out, err := run(t, "harness", "eval", path)
	if err != nil {
		t.Fatalf("harness eval returned error: %v", err)
	}
	if !strings.Contains(out, "det score: 100.0/100") {
		t.Errorf("harness eval output = %q, want a full det score", out)
	}
}

func TestHarnessEvalMinFloorPasses(t *testing.T) {
	path := filepath.Join(t.TempDir(), "skill.md")
	doc := "# Skill\nIf the build fails, retry.\n## Boundary\nNot for zero-to-one.\n"
	if err := os.WriteFile(path, []byte(doc), 0o600); err != nil {
		t.Fatalf("write artifact: %v", err)
	}
	if _, err := run(t, "harness", "--min", "50", "eval", path); err != nil {
		t.Errorf("eval --min 50 on a 100-scoring doc should pass, got %v", err)
	}
}

func TestHarnessEvalMinFloorFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "skill.md")
	doc := "# Skill\nIf the build fails, retry.\n## Boundary\nNot for zero-to-one.\n"
	if err := os.WriteFile(path, []byte(doc), 0o600); err != nil {
		t.Fatalf("write artifact: %v", err)
	}
	_, err := run(t, "harness", "--min", "101", "eval", path)
	var exit root.ExitError
	if !errors.As(err, &exit) || int(exit) != 1 {
		t.Fatalf("eval below --min floor = %v, want ExitError(1)", err)
	}
}

func TestHarnessUnknownVerb(t *testing.T) {
	if _, err := run(t, "harness", "frobnicate"); err == nil {
		t.Errorf("unknown harness verb should return an error")
	}
}

func run(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var out, errb bytes.Buffer
	err := cmd.Run(context.Background(), args, strings.NewReader(""), &out, &errb)
	return out.String(), err
}

func TestGateAcceptDispatch(t *testing.T) {
	out, err := run(t, "gate", "--candidate", "90", "--current", "84")
	if err != nil {
		t.Fatalf("gate accept returned error: %v", err)
	}
	if !strings.Contains(out, "accept_new_best") {
		t.Errorf("gate accept output = %q, want accept_new_best", out)
	}
}

func TestGateRejectExitCode(t *testing.T) {
	_, err := run(t, "gate", "--candidate", "80", "--current", "84")
	var exit root.ExitError
	if !errors.As(err, &exit) || int(exit) != 1 {
		t.Fatalf("gate reject error = %v, want ExitError(1)", err)
	}
}

func TestOracleSelfTestDispatch(t *testing.T) {
	out, err := run(t, "oracle", "selftest")
	if err != nil {
		t.Fatalf("oracle selftest returned error: %v", err)
	}
	if !strings.Contains(out, "passed") {
		t.Errorf("oracle selftest output = %q, want a pass", out)
	}
}

func TestUnknownVerb(t *testing.T) {
	if _, err := run(t, "arc", "frobnicate"); err == nil {
		t.Errorf("unknown arc verb should return an error")
	}
}

// selectionFailure returns a failure class that hashes to the selection split,
// so a seeded arc's mined task is held out for acceptance. Deterministic.
func selectionFailure(t *testing.T) string {
	t.Helper()
	for i := range 200 {
		class := fmt.Sprintf("missing-thing-%d", i)
		if consolidate.SplitFor(class) == consolidate.SplitSelection {
			return class
		}
	}
	t.Fatal("no synthetic class hashed to the selection split")
	return ""
}

// seedSleepWorkspace writes two closed arcs sharing one failure class and a
// realistically sized managed artifact into the current (temp) directory.
func seedSleepWorkspace(t *testing.T, failure string) {
	t.Helper()
	store := state.Default()
	for _, id := range []string{"arc-0001", "arc-0002"} {
		arc := adh.Arc{ID: id, Status: adh.StatusClosed, History: []string{"critic: " + failure}}
		if err := store.Save(&arc); err != nil {
			t.Fatalf("seed arc %s: %v", id, err)
		}
	}
	if err := os.MkdirAll(".adh/context", 0o750); err != nil {
		t.Fatalf("mkdir context: %v", err)
	}
	art := "# Harness\n\n" + strings.Repeat("Guidance for doing the work well and safely.\n", 6)
	if err := os.WriteFile(".adh/context/harness.md", []byte(art), 0o600); err != nil {
		t.Fatalf("seed artifact: %v", err)
	}
}

func stagedIDs(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(".adh/sleep/staging")
	if err != nil {
		t.Fatalf("read staging: %v", err)
	}
	var ids []string
	for _, entry := range entries {
		if entry.IsDir() {
			ids = append(ids, entry.Name())
		}
	}
	return ids
}

func TestSleepRunStagesProposal(t *testing.T) {
	t.Chdir(t.TempDir())
	seedSleepWorkspace(t, selectionFailure(t))
	_, err := run(t, "sleep", "run")
	var exit root.ExitError
	if !errors.As(err, &exit) || int(exit) != 14 {
		t.Fatalf("sleep run = %v, want ExitError(14) (proposal staged, adoption pending)", err)
	}
	if ids := stagedIDs(t); len(ids) != 1 {
		t.Fatalf("staged %d proposals, want exactly 1", len(ids))
	}
}

func TestSleepAdoptAppliesProposal(t *testing.T) {
	t.Chdir(t.TempDir())
	failure := selectionFailure(t)
	seedSleepWorkspace(t, failure)
	_, _ = run(t, "sleep", "run") // stages (ExitError(14))
	ids := stagedIDs(t)
	if len(ids) != 1 {
		t.Fatalf("expected one staged id, got %v", ids)
	}
	if _, err := run(t, "sleep", "adopt", ids[0]); err != nil {
		t.Fatalf("sleep adopt: %v", err)
	}
	live, err := os.ReadFile(".adh/context/harness.md")
	if err != nil {
		t.Fatalf("read adopted artifact: %v", err)
	}
	if !strings.Contains(string(live), failure) {
		t.Errorf("adopted artifact does not contain the learned class %q", failure)
	}
	if !strings.Contains(string(live), "ADH:LEARNED") {
		t.Errorf("adopted artifact is missing the protected-region marker")
	}
}

func TestSleepRunNoImprovement(t *testing.T) {
	t.Chdir(t.TempDir())
	arc := adh.Arc{
		ID:      "arc-0001",
		Status:  adh.StatusClosed,
		History: []string{"critic: looks good"},
	}
	if err := state.Default().Save(&arc); err != nil {
		t.Fatalf("seed arc: %v", err)
	}
	out, err := run(t, "sleep", "run")
	if err != nil {
		t.Fatalf("sleep run with nothing to learn should exit 0, got %v", err)
	}
	if !strings.Contains(out, "no proposal staged") {
		t.Errorf("output = %q, want a self-explanation", out)
	}
}

func TestSleepRunReportsLongitudinal(t *testing.T) {
	t.Chdir(t.TempDir())
	seedSleepWorkspace(t, selectionFailure(t))
	out, _ := run(t, "sleep", "run") // stages (ExitError(14))
	if !strings.Contains(out, "longitudinal") {
		t.Errorf("run summary = %q, want a longitudinal report", out)
	}
	guidance := filepath.Join(".adh", "sleep", "staging", stagedIDs(t)[0], "longitudinal.md")
	if _, err := os.Stat(guidance); err != nil {
		t.Errorf("slow-update guidance not staged: %v", err)
	}
}

func TestSleepUnknownVerb(t *testing.T) {
	if _, err := run(t, "sleep", "frobnicate"); err == nil {
		t.Errorf("unknown sleep verb should return an error")
	}
}
