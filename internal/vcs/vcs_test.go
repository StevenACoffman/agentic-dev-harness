package vcs_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/StevenACoffman/agentic-dev-harness/internal/vcs"
)

// Both implementations satisfy the point-of-use interface repo (below) — the
// contract the mock exists to honor. Declared as vars (before the type) to keep
// the const→var→type→func order.
var (
	_ repo = (*vcs.Git)(nil)
	_ repo = (*vcs.Mock)(nil)

	who = vcs.Signature{Name: "adh", Email: "adh@example.test"}
	at  = time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)
)

// repo is the version-control surface a consumer needs, declared here in the
// test so production carries no unused interface.
type repo interface {
	CurrentBranch() (string, error)
	Status() (vcs.Status, error)
	CreateBranch(name string) error
	Commit(msg string, who vcs.Signature, when time.Time) (string, error)
}

func TestGitLifecycle(t *testing.T) {
	dir := t.TempDir()
	repo, err := vcs.Init(dir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	dirty, err := repo.Status()
	if err != nil {
		t.Fatalf("Status (dirty): %v", err)
	}
	if dirty.Clean || len(dirty.Changed) != 1 || dirty.Changed[0] != "a.txt" {
		t.Fatalf("dirty status = %+v, want a.txt changed and not clean", dirty)
	}

	hash, err := repo.Commit("initial", who, at)
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if hash == "" {
		t.Error("Commit returned an empty hash")
	}

	clean, err := repo.Status()
	if err != nil {
		t.Fatalf("Status (clean): %v", err)
	}
	if !clean.Clean || len(clean.Changed) != 0 {
		t.Errorf("post-commit status = %+v, want clean", clean)
	}

	if err := repo.CreateBranch("feature/x"); err != nil {
		t.Fatalf("CreateBranch: %v", err)
	}
	branch, err := repo.CurrentBranch()
	if err != nil {
		t.Fatalf("CurrentBranch: %v", err)
	}
	if branch != "feature/x" {
		t.Errorf("current branch = %q, want feature/x", branch)
	}
}

func TestGitOpenNonRepo(t *testing.T) {
	if _, err := vcs.Open(t.TempDir()); err == nil {
		t.Error("Open of a non-repository directory should error")
	}
}

func TestMock(t *testing.T) {
	m := &vcs.Mock{Changed: []string{"x.go"}}
	st, err := m.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if st.Clean || st.Branch != "main" {
		t.Errorf("mock status = %+v, want dirty on main", st)
	}
	if _, err := m.Commit("c", who, at); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	after, _ := m.Status()
	if !after.Clean {
		t.Errorf("mock should be clean after commit, got %+v", after)
	}
}
