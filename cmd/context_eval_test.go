package cmd_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/cmd/root"
)

// writeCases seeds the routing-eval fixtures under .adh.
func writeCases(t *testing.T, body string) {
	t.Helper()
	if err := os.MkdirAll(".adh", 0o750); err != nil {
		t.Fatalf("mkdir .adh: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(".adh", "routing-cases.json"),
		[]byte(body),
		0o600,
	); err != nil {
		t.Fatalf("write cases: %v", err)
	}
}

// TestContextEvalPasses: routing that matches every fixture reports a clean eval.
func TestContextEvalPasses(t *testing.T) {
	t.Chdir(t.TempDir())
	writeUnit(t, "sec", "base-rule", "fail secure", "security")
	writeCases(t, `[{"name":"sec routes","labels":["security"],"want":["sec"]}]`)
	out := mustRun(t, "context", "eval")
	if !strings.Contains(out, "1/1 cases pass") {
		t.Errorf("eval output = %q, want 1/1 pass", out)
	}
}

// TestContextEvalFails: a fixture the store no longer satisfies fails the eval
// (exit 12), so a routing regression is caught.
func TestContextEvalFails(t *testing.T) {
	t.Chdir(t.TempDir())
	writeUnit(t, "sec", "base-rule", "fail secure", "security")
	writeCases(t, `[{"name":"expects a missing unit","labels":["security"],"want":["ghost"]}]`)
	_, err := run(t, "context", "eval")
	var exit root.ExitError
	if !errors.As(err, &exit) || int(exit) != 12 {
		t.Fatalf("eval with a failing case = %v, want ExitError(12)", err)
	}
}
