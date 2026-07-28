package cmd_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/state"
)

func TestGateListShowsBlockedArcs(t *testing.T) {
	t.Chdir(t.TempDir())
	store := state.Default()
	blocked := adh.Arc{
		ID:      "arc-0001",
		Stage:   adh.StageOps,
		Status:  adh.StatusBlocked,
		History: []string{"blocked: ops is the ship gate"},
	}
	open := adh.Arc{ID: "arc-0002", Stage: adh.StageStrategy, Status: adh.StatusOpen}
	for _, arc := range []adh.Arc{blocked, open} {
		if err := store.Save(&arc); err != nil {
			t.Fatalf("seed arc: %v", err)
		}
	}
	out, err := run(t, "gate", "list")
	if err != nil {
		t.Fatalf("gate list: %v", err)
	}
	if !strings.Contains(out, "arc-0001") || !strings.Contains(out, "ops is the ship gate") {
		t.Errorf("blocked arc + reason missing:\n%s", out)
	}
	if strings.Contains(out, "arc-0002") {
		t.Errorf("open arc should not appear as a pending gate:\n%s", out)
	}
}

func TestGateListEmpty(t *testing.T) {
	t.Chdir(t.TempDir())
	out, err := run(t, "gate", "list")
	if err != nil {
		t.Fatalf("gate list: %v", err)
	}
	if !strings.Contains(out, "no pending gates") {
		t.Errorf("empty gate list = %q, want the empty notice", out)
	}
}

func TestHarnessHashReportsIdentity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "artifact.md")
	if err := os.WriteFile(path, []byte("guiding text\n"), 0o600); err != nil {
		t.Fatalf("write artifact: %v", err)
	}
	first, err := run(t, "harness", "hash", path)
	if err != nil {
		t.Fatalf("harness hash: %v", err)
	}
	if strings.TrimSpace(first) == "" {
		t.Fatal("harness hash printed nothing")
	}
	second, err := run(t, "harness", "hash", path)
	if err != nil {
		t.Fatalf("harness hash (again): %v", err)
	}
	if first != second {
		t.Errorf("hash not stable for identical content: %q vs %q", first, second)
	}
}
