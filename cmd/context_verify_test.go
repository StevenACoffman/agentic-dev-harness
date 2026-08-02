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
	"github.com/StevenACoffman/agentic-dev-harness/internal/toolreg"
	"github.com/StevenACoffman/agentic-dev-harness/internal/toolrun"
)

// seedIntegrity writes one context unit that declares an integrity check plus a
// tool registry whose entry is a shell one-liner, so `context verify` runs
// deterministically without any external binary. checkRun is the tool's command.
func seedIntegrity(t *testing.T, checkRun string) {
	t.Helper()
	if err := os.MkdirAll(contextstore.DefaultStoreDir, 0o750); err != nil {
		t.Fatalf("mkdir store: %v", err)
	}
	unit := contextstore.Unit{
		ID:        "model",
		Kind:      "domain-model",
		Labels:    []string{"internal"},
		Integrity: "drift-check",
	}
	data, err := json.MarshalIndent(unit, "", "  ")
	if err != nil {
		t.Fatalf("marshal unit: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(contextstore.DefaultStoreDir, "model.json"),
		data,
		0o600,
	); err != nil {
		t.Fatalf("write unit: %v", err)
	}
	reg := toolreg.Registry{Tools: []toolreg.Tool{
		{ID: "drift-check", Run: checkRun, Verifies: "model matches source"},
	}}
	regData, err := toolreg.Marshal(reg)
	if err != nil {
		t.Fatalf("marshal registry: %v", err)
	}
	if err := os.MkdirAll(".adh", 0o750); err != nil {
		t.Fatalf("mkdir .adh: %v", err)
	}
	if err := os.WriteFile(toolreg.DefaultRegistryFile, regData, 0o600); err != nil {
		t.Fatalf("write registry: %v", err)
	}
}

// TestContextVerifyClean: an integrity check that passes leaves the store verified.
func TestContextVerifyClean(t *testing.T) {
	t.Chdir(t.TempDir())
	seedIntegrity(t, "exit 0")
	out := mustRun(t, "context", "verify")
	if !strings.Contains(out, "ok") || !strings.Contains(out, "model") {
		t.Errorf("verify output = %q, want an ok line for model", out)
	}
}

// TestContextVerifyDrift: an integrity check that fails is drift — exit 14 — so a
// stage that routed a stale unit stops.
func TestContextVerifyDrift(t *testing.T) {
	t.Chdir(t.TempDir())
	seedIntegrity(t, "exit 1")
	_, err := run(t, "context", "verify")
	var exit root.ExitError
	if !errors.As(err, &exit) || int(exit) != 18 {
		t.Fatalf("verify with drift = %v, want ExitError(18)", err)
	}
}

// TestContextVerifyLogsToolRun: a verify run of an integrity tool is recorded to the
// tool-run log (§16/§18) — a drift is a failed run for that tool's KPI — so `adh kpi`
// can measure the tool over time.
func TestContextVerifyLogsToolRun(t *testing.T) {
	t.Chdir(t.TempDir())
	seedIntegrity(t, "exit 1") // drift → ran and failed
	if _, err := run(t, "context", "verify"); err == nil {
		t.Fatal("expected drift to exit non-zero")
	}
	records, err := toolrun.Load(toolrun.RunFile)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(records) != 1 || records[0].Tool != "drift-check" || !records[0].Ran || !records[0].Failed {
		t.Fatalf("tool-run log = %+v, want one ran+failed drift-check run", records)
	}
}

// TestContextVerifyUnverified: an integrity tool that is not installed is reported
// unverified but does not fail the gate (best-effort, unrunnable = unconfirmed).
func TestContextVerifyUnverified(t *testing.T) {
	t.Chdir(t.TempDir())
	seedIntegrity(t, "definitely-not-a-real-binary-xyz")
	out := mustRun(t, "context", "verify")
	if !strings.Contains(out, "unverified") {
		t.Errorf("verify output = %q, want an unverified line", out)
	}
}

// TestContextVerifyUnknownIntegrityTool: a unit whose integrity names a tool the
// registry does not declare is a store misconfiguration, not a silent pass.
func TestContextVerifyUnknownIntegrityTool(t *testing.T) {
	t.Chdir(t.TempDir())
	seedIntegrity(t, "exit 0")
	// Point the unit at an undeclared tool.
	unit := contextstore.Unit{ID: "model", Kind: "domain-model", Integrity: "no-such-tool"}
	data, _ := json.MarshalIndent(unit, "", "  ")
	if err := os.WriteFile(
		filepath.Join(contextstore.DefaultStoreDir, "model.json"),
		data,
		0o600,
	); err != nil {
		t.Fatalf("rewrite unit: %v", err)
	}
	if _, err := run(t, "context", "verify"); err == nil {
		t.Errorf("verify with an undeclared integrity tool should error")
	}
}

// TestContextLintDuplicateID: lint catches two units answering to the same id — a
// cross-unit consistency defect that makes routing ambiguous (§10.4).
func TestContextLintDuplicateID(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.MkdirAll(contextstore.DefaultStoreDir, 0o750); err != nil {
		t.Fatalf("mkdir store: %v", err)
	}
	for _, name := range []string{"a", "b"} {
		unit := contextstore.Unit{ID: "dup", Kind: "runbook"}
		data, _ := json.MarshalIndent(unit, "", "  ")
		if err := os.WriteFile(
			filepath.Join(contextstore.DefaultStoreDir, name+".json"),
			data,
			0o600,
		); err != nil {
			t.Fatalf("write unit %s: %v", name, err)
		}
	}
	_, err := run(t, "context", "lint")
	var exit root.ExitError
	if !errors.As(err, &exit) || int(exit) != 12 {
		t.Fatalf("lint with duplicate id = %v, want ExitError(12)", err)
	}
}
