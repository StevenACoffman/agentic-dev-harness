package cmd_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/cmd/root"
	"github.com/StevenACoffman/agentic-dev-harness/internal/contextstore"
)

// TestDoctorCleanAfterInit: a freshly initialized repo passes the harness-integrity
// self-check.
func TestDoctorCleanAfterInit(t *testing.T) {
	t.Chdir(t.TempDir())
	mustRun(t, "init")
	out := mustRun(t, "doctor")
	if !strings.Contains(out, "harness intact") {
		t.Errorf("doctor after init = %q, want it to report the harness intact", out)
	}
}

// TestDoctorCatchesDanglingIntegrity: a unit whose integrity check names no declared
// tool is a harness-integrity failure (exit 16).
func TestDoctorCatchesDanglingIntegrity(t *testing.T) {
	t.Chdir(t.TempDir())
	mustRun(t, "init")
	unit := contextstore.Unit{ID: "model", Kind: "domain-model", Integrity: "ghost-tool"}
	data, _ := json.MarshalIndent(unit, "", "  ")
	if err := os.WriteFile(
		filepath.Join(contextstore.DefaultStoreDir, "model.json"),
		data,
		0o600,
	); err != nil {
		t.Fatalf("write unit: %v", err)
	}
	_, err := run(t, "doctor")
	var exit root.ExitError
	if !errors.As(err, &exit) || int(exit) != 16 {
		t.Fatalf("doctor with a dangling integrity ref = %v, want ExitError(16)", err)
	}
}
