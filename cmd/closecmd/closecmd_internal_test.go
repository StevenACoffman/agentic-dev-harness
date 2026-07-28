package closecmd

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/vcs"
)

// shipAt is a fixed author time so the test reads no clock.
var shipAt = time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)

// seedRepo returns a git repo with one baseline commit on its default branch.
func seedRepo(t *testing.T) (*vcs.Git, string) {
	t.Helper()
	dir := t.TempDir()
	repo, err := vcs.Init(dir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(dir, "base.go"),
		[]byte("package p\n"),
		0o600,
	); err != nil {
		t.Fatalf("write baseline: %v", err)
	}
	who := vcs.Signature{Name: "adh", Email: "adh@example.test"}
	if _, err := repo.Commit("baseline", who, shipAt); err != nil {
		t.Fatalf("baseline Commit: %v", err)
	}
	return repo, dir
}

func TestShipBranchIsolatesTheArc(t *testing.T) {
	repo, _ := seedRepo(t)
	base, err := repo.CurrentBranch()
	if err != nil {
		t.Fatalf("CurrentBranch: %v", err)
	}

	arc := &adh.Arc{ID: "arc-0001", Title: "do a thing"}
	branch := shipBranch(repo, arc)
	if branch != "adh/arc-0001" {
		t.Errorf("shipBranch = %q, want adh/arc-0001", branch)
	}
	// CreateBranch checks the new branch out, so the commit lands there, off base.
	current, err := repo.CurrentBranch()
	if err != nil {
		t.Fatalf("CurrentBranch after ship: %v", err)
	}
	if current != "adh/arc-0001" {
		t.Errorf("checked-out branch = %q, want adh/arc-0001 (base was %q)", current, base)
	}
}

func TestShipBranchFallsBackWithoutACommit(t *testing.T) {
	dir := t.TempDir()
	repo, err := vcs.Init(dir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	// No commit yet: a branch needs one, so ship stays on the current branch.
	current, err := repo.CurrentBranch()
	if err != nil {
		t.Fatalf("CurrentBranch: %v", err)
	}
	arc := &adh.Arc{ID: "arc-0002", Title: "first change"}
	if branch := shipBranch(repo, arc); branch != current {
		t.Errorf("shipBranch = %q, want fallback to current branch %q", branch, current)
	}
}
