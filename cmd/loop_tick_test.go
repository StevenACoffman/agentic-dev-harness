package cmd_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	looplib "github.com/StevenACoffman/agentic-dev-harness/internal/loop"
	"github.com/StevenACoffman/agentic-dev-harness/internal/state"
)

// writeLoops seeds the loop registry with shell one-liner sensors so `loop tick`
// runs deterministically without invoking real adh subcommands.
func writeLoops(t *testing.T, body string) {
	t.Helper()
	if err := os.MkdirAll(".adh", 0o750); err != nil {
		t.Fatalf("mkdir .adh: %v", err)
	}
	if err := os.WriteFile(looplib.DefaultRegistryFile, []byte(body), 0o600); err != nil {
		t.Fatalf("write loops: %v", err)
	}
}

// TestLoopTickOpensArcsOnFindings: tick sweeps every loop, opening an arc for each
// departure (a non-zero sensor) and leaving the holding ones alone.
func TestLoopTickOpensArcsOnFindings(t *testing.T) {
	t.Chdir(t.TempDir())
	writeLoops(t, `{"loops":[
	  {"id":"drift","goal":"routed context matches source","sensor":"exit 1","on_finding":"open arc","retire_when":"never","owner":"context"},
	  {"id":"ok","goal":"registry valid","sensor":"exit 0","on_finding":"open arc","retire_when":"never"}
	]}`)
	out := mustRun(t, "loop", "tick")
	if !strings.Contains(out, "opened 1 arc") {
		t.Errorf("tick should open exactly one arc:\n%s", out)
	}
	arcs, err := state.NewStore(filepath.Join(".", state.DefaultArcsDir)).List()
	if err != nil {
		t.Fatalf("list arcs: %v", err)
	}
	if len(arcs) != 1 || arcs[0].Title != "routed context matches source" {
		t.Fatalf("want one arc titled by the drift loop's goal, got %+v", arcs)
	}
}
