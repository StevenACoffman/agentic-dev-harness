package cmd_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/cmd/root"
)

// writeContentUnit writes a unit JSON and (when body != "") its content file under
// .adh/context in the current directory.
func writeContentUnit(t *testing.T, unitJSON, contentFile, body string) {
	t.Helper()
	dir := filepath.Join(".adh", "context")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "unit.json"), []byte(unitJSON), 0o600); err != nil {
		t.Fatalf("write unit: %v", err)
	}
	if body != "" {
		if err := os.WriteFile(filepath.Join(dir, contentFile), []byte(body), 0o600); err != nil {
			t.Fatalf("write content: %v", err)
		}
	}
}

// TestContextShowDeliversContent: `context show` returns the unit's text and
// provenance — the content the routing preview points a worker at (§10.4).
func TestContextShowDeliversContent(t *testing.T) {
	t.Chdir(t.TempDir())
	writeContentUnit(t,
		`{"id":"crypto","kind":"domain-note","labels":["security"],`+
			`"content_path":"crypto.md","provenance":"OWASP crypto guide"}`,
		"crypto.md", "Use the approved AEAD library; never hand-roll crypto.")

	out := mustRun(t, "context", "show", "crypto")
	if !strings.Contains(out, "approved AEAD library") {
		t.Errorf("show did not deliver the unit text:\n%s", out)
	}
	if !strings.Contains(out, "OWASP crypto guide") {
		t.Errorf("show did not print provenance:\n%s", out)
	}

	// Under --jsonl the content + provenance are one machine-readable outcome.
	recs := jsonLines(t, mustRun(t, "--jsonl", "context", "show", "crypto"))
	if len(recs) != 1 {
		t.Fatalf("show --jsonl = %d lines, want 1", len(recs))
	}
	data := okData(t, recs[0])
	if !strings.Contains(data["content"].(string), "AEAD") ||
		data["provenance"] != "OWASP crypto guide" {
		t.Errorf("show outcome = %v, want the content + provenance", data)
	}
}

// TestContextShowUnknownID errors on an id that is not in the store.
func TestContextShowUnknownID(t *testing.T) {
	t.Chdir(t.TempDir())
	if _, err := run(t, "context", "show", "missing"); err == nil {
		t.Errorf("show of an unknown unit should error")
	}
}

// TestContextLintFlagsDanglingContent: lint fails when a unit's content_path does
// not resolve — the routing promised text that is not there.
func TestContextLintFlagsDanglingContent(t *testing.T) {
	t.Chdir(t.TempDir())
	// Unit references a content file that is never written.
	writeContentUnit(t,
		`{"id":"dangling","kind":"skill","content_path":"gone.md"}`, "", "")

	_, err := run(t, "context", "lint")
	var exit root.ExitError
	if !errors.As(err, &exit) || int(exit) != 12 {
		t.Errorf("lint with a dangling content_path = %v, want ExitError(12)", err)
	}
}
