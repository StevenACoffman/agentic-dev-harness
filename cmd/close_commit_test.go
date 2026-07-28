package cmd_test

import (
	"os"
	"strings"
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/state"
	"github.com/StevenACoffman/agentic-dev-harness/internal/vcs"
)

// seedApprovedChangeArc writes an approved (open at ops) change arc with a proof
// packet created over the given artifact, returning its id.
func seedApprovedChangeArc(t *testing.T, artifact string) string {
	t.Helper()
	arc := adh.Arc{
		ID:         "arc-0001",
		Title:      "widen it",
		Stage:      adh.StageOps,
		Status:     adh.StatusOpen,
		Resolution: adh.ResolutionChange,
	}
	if err := state.Default().Save(&arc); err != nil {
		t.Fatalf("seed arc: %v", err)
	}
	if err := os.WriteFile(artifact, []byte("the change"), 0o600); err != nil {
		t.Fatalf("write artifact: %v", err)
	}
	mustRun(t, "proof", "create", arc.ID, artifact) // records Arc.Proof
	return arc.ID
}

func TestCloseCommitsChangeArc(t *testing.T) {
	t.Chdir(t.TempDir())
	if _, err := vcs.Init("."); err != nil {
		t.Fatalf("git init: %v", err)
	}
	id := seedApprovedChangeArc(t, "widget.go")

	out := mustRun(t, "close", id) // no --proof: falls back to Arc.Proof
	if !strings.Contains(out, "committed") {
		t.Errorf("close output missing the commit line:\n%s", out)
	}
	arc, err := state.Default().Get(id)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if arc.Status != adh.StatusClosed {
		t.Errorf("status = %s, want closed", arc.Status)
	}
	committed := false
	for _, h := range arc.History {
		if strings.HasPrefix(h, "committed ") {
			committed = true
		}
	}
	if !committed {
		t.Errorf("history has no commit record: %v", arc.History)
	}
}

func TestCloseWithoutRepoStillCloses(t *testing.T) {
	t.Chdir(t.TempDir()) // no git repo here
	id := seedApprovedChangeArc(t, "widget.go")

	if _, err := run(t, "close", id); err != nil {
		t.Fatalf("close without a repo should still succeed: %v", err)
	}
	arc, err := state.Default().Get(id)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if arc.Status != adh.StatusClosed {
		t.Errorf("status = %s, want closed", arc.Status)
	}
	for _, h := range arc.History {
		if strings.HasPrefix(h, "committed ") {
			t.Errorf("no repo, but a commit was recorded: %q", h)
		}
	}
}
