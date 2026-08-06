package cmd_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/internal/evidence"
)

// TestSleepStatusReportsCalibration wires the sleep-optimizer calibration end to
// end: seed the evidence log with two proposed-candidate cycles (one kept, one
// rejected) plus a no-candidate baseline, then confirm `sleep status` reports the
// calibration over exactly the two proposed cycles.
func TestSleepStatusReportsCalibration(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.MkdirAll(filepath.Join(".adh", "sleep"), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	f, err := os.Create(filepath.Join(".adh", "sleep", "evidence.jsonl"))
	if err != nil {
		t.Fatalf("create evidence log: %v", err)
	}
	if appendErr := evidence.Append(
		f,
		evidence.Record{OldScore: 0.5, NewScore: 0.9, Status: evidence.StatusKeep},     // kept
		evidence.Record{OldScore: 0.5, NewScore: 0.6, Status: evidence.StatusBaseline}, // rejected
		evidence.Record{
			OldScore: 0.5,
			NewScore: 0.5,
			Status:   evidence.StatusBaseline,
		}, // no candidate
	); appendErr != nil {
		t.Fatalf("seed evidence: %v", appendErr)
	}
	_ = f.Close()

	out, err := run(t, "sleep", "status")
	if err != nil {
		t.Fatalf("sleep status: %v", err)
	}
	if !strings.Contains(out, "calibration: 2 cycles") {
		t.Errorf("sleep status is missing the calibration line (2 proposed cycles):\n%s", out)
	}
}
