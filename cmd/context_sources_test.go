package cmd_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/cmd/root"
	"github.com/StevenACoffman/agentic-dev-harness/internal/contextstore"
)

// writeSourcedUnit writes a unit that cites a repo-relative provenance source.
func writeSourcedUnit(t *testing.T, id, source string) {
	t.Helper()
	if err := os.MkdirAll(contextstore.DefaultStoreDir, 0o750); err != nil {
		t.Fatalf("mkdir store: %v", err)
	}
	unit := contextstore.Unit{
		ID:      id,
		Kind:    "base-rule",
		Labels:  []string{"x"},
		Sources: []string{source},
	}
	data, _ := json.MarshalIndent(unit, "", "  ")
	if err := os.WriteFile(
		filepath.Join(contextstore.DefaultStoreDir, id+".json"),
		data,
		0o600,
	); err != nil {
		t.Fatalf("write unit: %v", err)
	}
}

// TestContextLintDanglingSource: a unit citing a source path that does not exist
// fails lint (exit 12) — provenance receipt verification.
func TestContextLintDanglingSource(t *testing.T) {
	t.Chdir(t.TempDir())
	writeSourcedUnit(t, "rule", "docs/nope.md")
	_, err := run(t, "context", "lint")
	var exit root.ExitError
	if !errors.As(err, &exit) || int(exit) != 12 {
		t.Fatalf("lint with a dangling source = %v, want ExitError(12)", err)
	}
	// A present source lints clean.
	t.Chdir(t.TempDir())
	if err := os.MkdirAll("docs", 0o750); err != nil {
		t.Fatalf("mkdir docs: %v", err)
	}
	if err := os.WriteFile("docs/real.md", []byte("x"), 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}
	writeSourcedUnit(t, "rule", "docs/real.md")
	mustRun(t, "context", "lint")
}

// TestDoctorCatchesDanglingSource: doctor reports a dangling provenance source (exit 16).
func TestDoctorCatchesDanglingSource(t *testing.T) {
	t.Chdir(t.TempDir())
	mustRun(t, "init")
	writeSourcedUnit(t, "rule", "docs/ghost.md")
	_, err := run(t, "doctor")
	var exit root.ExitError
	if !errors.As(err, &exit) || int(exit) != 16 {
		t.Fatalf("doctor with a dangling source = %v, want ExitError(16)", err)
	}
}
