package cmd_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/internal/vcs"
)

func TestVCSStatusCommitBranch(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if _, err := vcs.Init(dir); err != nil {
		t.Fatalf("init repo: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("x"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	if out := mustRun(t, "vcs", "status"); !strings.Contains(out, "dirty") ||
		!strings.Contains(out, "a.txt") {
		t.Errorf("vcs status (dirty) = %q, want dirty + a.txt", out)
	}
	mustRun(t, "vcs", "-m", "initial", "commit")
	if out := mustRun(t, "vcs", "status"); !strings.Contains(out, "clean") {
		t.Errorf("vcs status after commit = %q, want clean", out)
	}
	mustRun(t, "vcs", "branch", "feature")
	if out := mustRun(t, "vcs", "status"); !strings.Contains(out, "feature") {
		t.Errorf("vcs status after branch = %q, want branch feature", out)
	}
}

func TestVCSOutsideRepoErrors(t *testing.T) {
	t.Chdir(t.TempDir())
	if _, err := run(t, "vcs", "status"); err == nil {
		t.Error("vcs status outside a git repository should return an error")
	}
}

func TestVCSCommitRequiresMessage(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if _, err := vcs.Init(dir); err != nil {
		t.Fatalf("init repo: %v", err)
	}
	if _, err := run(t, "vcs", "commit"); err == nil {
		t.Error("vcs commit without -m should return an error")
	}
}
