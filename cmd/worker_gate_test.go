package cmd_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/cmd/root"
	"github.com/StevenACoffman/agentic-dev-harness/internal/state"
)

// writeStaleEpoch records a worker epoch whose model binding cannot match the
// current config, so the requalification gate (§14) fires.
func writeStaleEpoch(t *testing.T) {
	t.Helper()
	if err := os.MkdirAll(".adh", 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	epoch := `{"id":"stale","models":{"strategy":"a-retired-model"}}`
	if err := os.WriteFile(filepath.Join(".adh", "worker.json"), []byte(epoch), 0o600); err != nil {
		t.Fatalf("write epoch: %v", err)
	}
}

func TestRunRefusesWhenWorkerChanged(t *testing.T) {
	t.Chdir(t.TempDir())
	writeStaleEpoch(t)
	seedOpenArc(t)
	out, err := run(t, "run", "--jsonl", "arc-0001")
	assertBlocked(t, out, err, 9, root.ReasonRequalify)
}

func TestStepRefusesWhenWorkerChanged(t *testing.T) {
	t.Chdir(t.TempDir())
	writeStaleEpoch(t)
	seedOpenArc(t)
	out, err := run(t, "step", "--relay", "--jsonl", "arc-0001")
	assertBlocked(t, out, err, 9, root.ReasonRequalify)
}

func TestRunProceedsAfterRequalify(t *testing.T) {
	t.Chdir(t.TempDir())
	seedOpenArc(t)
	mustRun(t, "worker", "requalify") // records the current worker as the epoch

	// The recorded epoch now matches the current worker, so run is not gated — it
	// drives to a gate rather than refusing with exit 9.
	_, err := run(t, "run", "arc-0001")
	var exit root.ExitError
	if errors.As(err, &exit) && int(exit) == 9 {
		t.Errorf("run refused after requalify: %v", err)
	}
	got, gErr := state.Default().Get("arc-0001")
	if gErr != nil {
		t.Fatalf("reload: %v", gErr)
	}
	if got.Stage == "strategy" {
		t.Errorf("run did not advance the arc past strategy: stage %s", got.Stage)
	}
}
