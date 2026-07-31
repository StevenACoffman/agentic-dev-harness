package cmd_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/cmd/root"
	"github.com/StevenACoffman/agentic-dev-harness/internal/nfr"
)

// writeSpec writes one NFR spec into .adh/nfr for the CLI to load.
func writeSpec(t *testing.T, name, body string) {
	t.Helper()
	if err := os.MkdirAll(nfr.DefaultDir, 0o750); err != nil {
		t.Fatalf("mkdir nfr: %v", err)
	}
	if err := os.WriteFile(filepath.Join(nfr.DefaultDir, name), []byte(body), 0o600); err != nil {
		t.Fatalf("write spec: %v", err)
	}
}

// TestNFRLintValid: a well-formed Planguage spec lints clean.
func TestNFRLintValid(t *testing.T) {
	t.Chdir(t.TempDir())
	writeSpec(
		t,
		"latency.json",
		`{"id":"latency","tag":"Performance.Latency","scale":"ms","meter":"bench","direction":"lower","fail":300,"goal":200}`,
	)
	out := mustRun(t, "nfr", "lint")
	if !strings.Contains(out, "1 NFR spec(s), all valid") {
		t.Errorf("lint output = %q, want all valid", out)
	}
}

// TestNFRLintInvalid: a mis-ordered or mis-tagged spec fails lint (exit 17).
func TestNFRLintInvalid(t *testing.T) {
	t.Chdir(t.TempDir())
	writeSpec(
		t,
		"bad.json",
		`{"id":"bad","tag":"Vibes.Speed","scale":"ms","meter":"bench","direction":"lower","fail":100,"goal":200}`,
	)
	_, err := run(t, "nfr", "lint")
	var exit root.ExitError
	if !errors.As(err, &exit) || int(exit) != 17 {
		t.Fatalf("lint of an invalid spec = %v, want ExitError(17)", err)
	}
}
