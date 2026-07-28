package cmd_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/state"
	"github.com/StevenACoffman/agentic-dev-harness/internal/vcs"
)

// TestStepRelayExecutionResumeCapturesPaths checks that resuming a relayed
// execution turn records the working tree's changed code paths into arc.Paths
// (§19.1/§19.3), excluding the harness's own .adh/ state.
func TestStepRelayExecutionResumeCapturesPaths(t *testing.T) {
	t.Chdir(t.TempDir())
	if _, err := vcs.Init("."); err != nil {
		t.Fatalf("git init: %v", err)
	}
	if err := os.WriteFile("widget.go", []byte("package widget\n"), 0o600); err != nil {
		t.Fatalf("write change: %v", err)
	}

	arc := adh.Arc{ID: "arc-0001", Stage: adh.StageExecution, Status: adh.StatusOpen}
	if err := state.Default().Save(&arc); err != nil {
		t.Fatalf("seed arc: %v", err)
	}
	mustRun(t, "step", "--relay", "--json", arc.ID) // open the execution turn

	reply := filepath.Join(t.TempDir(), "reply.txt") // outside the repo tree
	if err := os.WriteFile(reply, []byte("built the widget"), 0o600); err != nil {
		t.Fatalf("write reply: %v", err)
	}
	mustRun(t, "step", "--relay", "--response", reply, "--json", arc.ID)

	reloaded, err := state.Default().Get(arc.ID)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.Stage != adh.StageCritic {
		t.Fatalf("stage = %s, want critic (execution resumed)", reloaded.Stage)
	}
	found := false
	for _, path := range reloaded.Paths {
		if strings.HasPrefix(path, ".adh/") {
			t.Errorf("captured a harness state path, should be filtered: %q", path)
		}
		if path == "widget.go" {
			found = true
		}
	}
	if !found {
		t.Errorf("changed file not captured into arc.Paths: %v", reloaded.Paths)
	}
}
