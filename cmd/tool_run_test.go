package cmd_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/cmd/root"
)

// seedTools writes a tool registry into the current repo's .adh so `tool run`
// resolves the entries. The commands are POSIX-shell one-liners so the test needs
// no external binary (the real entries name exegesis/skillsaw/modelith).
func seedTools(t *testing.T) {
	t.Helper()
	if err := os.MkdirAll(".adh", 0o750); err != nil {
		t.Fatalf("mkdir .adh: %v", err)
	}
	reg := `{"tools":[
	  {"id":"say","run":"printf hello","verifies":"greeting"},
	  {"id":"boom","run":"echo oops 1>&2; exit 7","verifies":"failure"},
	  {"id":"gone","run":"definitely-not-a-real-binary-xyz","verifies":"absence","repair_hint":"install the thing"}
	]}`
	if err := os.WriteFile(filepath.Join(".adh", "tools.json"), []byte(reg), 0o600); err != nil {
		t.Fatalf("write registry: %v", err)
	}
}

// TestToolRunStreamsOutput: a declared tool's stdout reaches the worker and a clean
// exit is a success.
func TestToolRunStreamsOutput(t *testing.T) {
	t.Chdir(t.TempDir())
	seedTools(t)
	out := mustRun(t, "tool", "run", "say")
	if !strings.Contains(out, "hello") {
		t.Errorf("tool run say output = %q, want it to contain \"hello\"", out)
	}
}

// TestToolRunPropagatesExitCode: a tool that ran and exited non-zero surfaces its
// own exit code, so the worker sees the real result.
func TestToolRunPropagatesExitCode(t *testing.T) {
	t.Chdir(t.TempDir())
	seedTools(t)
	_, err := run(t, "tool", "run", "boom")
	var exit root.ExitError
	if !errors.As(err, &exit) || int(exit) != 7 {
		t.Fatalf("tool run boom = %v, want ExitError(7)", err)
	}
}

// TestToolRunUnknownTool: an id not in the registry is a registry-level problem
// (exit 10), not a crash.
func TestToolRunUnknownTool(t *testing.T) {
	t.Chdir(t.TempDir())
	seedTools(t)
	_, err := run(t, "tool", "run", "nope")
	var exit root.ExitError
	if !errors.As(err, &exit) || int(exit) != 10 {
		t.Fatalf("tool run nope = %v, want ExitError(10)", err)
	}
}

// TestToolRunUnavailableBinary: a declared tool whose binary is not installed is
// reported (exit 10) rather than treated as a failing check.
func TestToolRunUnavailableBinary(t *testing.T) {
	t.Chdir(t.TempDir())
	seedTools(t)
	_, err := run(t, "tool", "run", "gone")
	var exit root.ExitError
	if !errors.As(err, &exit) || int(exit) != 10 {
		t.Fatalf("tool run gone = %v, want ExitError(10)", err)
	}
}

// TestToolRunJSONLCarriesOutput: under --jsonl the run's captured stdout, stderr,
// and exit code ride in one structured outcome the worker can parse.
func TestToolRunJSONLCarriesOutput(t *testing.T) {
	t.Chdir(t.TempDir())
	seedTools(t)
	out, err := run(t, "--jsonl", "tool", "run", "boom")
	var exit root.ExitError
	if !errors.As(err, &exit) || int(exit) != 7 {
		t.Fatalf("tool run boom --jsonl = %v, want ExitError(7)", err)
	}
	recs := jsonLines(t, out)
	if len(recs) != 1 {
		t.Fatalf("want one outcome line, got %d:\n%s", len(recs), out)
	}
	rec := recs[0]
	if rec["status"] != "error" || rec["reason"] != "tool_failed" {
		t.Errorf(
			"outcome status/reason = %v/%v, want error/tool_failed",
			rec["status"],
			rec["reason"],
		)
	}
	data, ok := rec["data"].(map[string]any)
	if !ok {
		t.Fatalf("outcome has no data object: %v", rec)
	}
	if !strings.Contains(data["stderr"].(string), "oops") {
		t.Errorf("captured stderr = %q, want it to contain \"oops\"", data["stderr"])
	}
	if data["exit"].(float64) != 7 {
		t.Errorf("captured exit = %v, want 7", data["exit"])
	}
}
