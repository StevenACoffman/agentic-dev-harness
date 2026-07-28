package cmd_test

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/state"
	"github.com/StevenACoffman/agentic-dev-harness/internal/vcs"
)

func TestRejectRevertsAndReturnsToExecution(t *testing.T) {
	t.Chdir(t.TempDir())
	repo, err := vcs.Init(".")
	if err != nil {
		t.Fatalf("git init: %v", err)
	}
	// Baseline: commit the file the arc will change, so a revert has a HEAD to
	// restore to. Do this before seeding the arc, so .adh is not in the commit.
	const baseline = "package p\nconst v = 1\n"
	if err := os.WriteFile("widget.go", []byte(baseline), 0o600); err != nil {
		t.Fatalf("write baseline: %v", err)
	}
	who := vcs.Signature{Name: "adh", Email: "adh@example.test"}
	at := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)
	if _, err := repo.Commit("baseline", who, at); err != nil {
		t.Fatalf("baseline commit: %v", err)
	}

	// A blocked arc carrying a change, with the footprint a rejected attempt leaves.
	arc := adh.Arc{
		ID:       "arc-0001",
		Title:    "widen it",
		Stage:    adh.StageOps,
		Status:   adh.StatusBlocked,
		Paths:    []string{"widget.go"},
		Labels:   []string{"widget"},
		Findings: []adh.Finding{{}},
		Pending:  &adh.Pending{Stage: adh.StageEvaluation, Prompt: "review"},
	}
	if err := state.Default().Save(&arc); err != nil {
		t.Fatalf("seed arc: %v", err)
	}
	// The uncommitted change the reject must undo.
	if err := os.WriteFile("widget.go", []byte("package p\nconst v = 999\n"), 0o600); err != nil {
		t.Fatalf("write change: %v", err)
	}

	out := mustRun(t, "reject", "--reason", "not ready", arc.ID)
	if !strings.Contains(out, "returned to execution") {
		t.Errorf("reject output missing the return line:\n%s", out)
	}

	// The working tree is restored to HEAD.
	restored, err := os.ReadFile("widget.go")
	if err != nil || string(restored) != baseline {
		t.Errorf("widget.go not reverted to HEAD: %q (err %v)", restored, err)
	}

	got, err := state.Default().Get(arc.ID)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got.Stage != adh.StageExecution || got.Status != adh.StatusOpen {
		t.Errorf("arc at %s/%s, want execution/open", got.Stage, got.Status)
	}
	if got.Pending != nil || got.Findings != nil || got.Paths != nil || got.Labels != nil {
		t.Errorf("footprint not cleared: pending=%v findings=%v paths=%v labels=%v",
			got.Pending, got.Findings, got.Paths, got.Labels)
	}
	if !strings.Contains(strings.Join(got.History, "\n"), "not ready") {
		t.Errorf("history missing the reason: %v", got.History)
	}
}

func TestRejectRequiresABlockedArc(t *testing.T) {
	t.Chdir(t.TempDir())
	arc := adh.Arc{ID: "arc-0002", Title: "open one", Stage: adh.StageOps, Status: adh.StatusOpen}
	if err := state.Default().Save(&arc); err != nil {
		t.Fatalf("seed arc: %v", err)
	}
	if _, err := run(t, "reject", arc.ID); err == nil {
		t.Error("rejecting a non-blocked arc should error")
	}
}
