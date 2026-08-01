package cmd_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/cmd/root"
)

// TestHarnessCalibrateAgrees: a fixture set the judge scores as labeled passes.
func TestHarnessCalibrateAgrees(t *testing.T) {
	cases := filepath.Join(t.TempDir(), "cases.json")
	body := `[
	  {"name":"good","output":"met the deadline","checks":[{"op":"contains","arg":"deadline"}],"want_pass":true},
	  {"name":"bad","output":"unrelated","checks":[{"op":"contains","arg":"deadline"}],"want_pass":false}
	]`
	if err := os.WriteFile(cases, []byte(body), 0o600); err != nil {
		t.Fatalf("write cases: %v", err)
	}
	out := mustRun(t, "harness", "calibrate", "--cases", cases)
	if !strings.Contains(out, "2/2 cases agree") {
		t.Errorf("calibrate output = %q, want 2/2 agree", out)
	}
}

// TestHarnessCalibrateDisagrees: a mislabeled fixture makes calibration fail (exit 1).
func TestHarnessCalibrateDisagrees(t *testing.T) {
	cases := filepath.Join(t.TempDir(), "cases.json")
	body := `[{"name":"mislabeled","output":"unrelated","checks":[{"op":"contains","arg":"deadline"}],"want_pass":true}]`
	if err := os.WriteFile(cases, []byte(body), 0o600); err != nil {
		t.Fatalf("write cases: %v", err)
	}
	_, err := run(t, "harness", "calibrate", "--cases", cases)
	var exit root.ExitError
	if !errors.As(err, &exit) || int(exit) != 1 {
		t.Fatalf("calibrate with a mislabeled fixture = %v, want ExitError(1)", err)
	}
}
