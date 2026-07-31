package cmd_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/state"
)

// TestStepRelayGroundsStrategyWithContextAndTools is the loop-wiring test: a
// strategy emit routes the context unit its label selects (§10) and surfaces the
// tool registry (§13) into the prompt, and records the loaded working set on the
// arc (§10.3).
func TestStepRelayGroundsStrategyWithContextAndTools(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.MkdirAll(filepath.Join(".adh", "context"), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	unit := `{"id":"u-auth","kind":"runbook","labels":["auth"]}`
	if err := os.WriteFile(
		filepath.Join(".adh", "context", "u-auth.json"),
		[]byte(unit),
		0o600,
	); err != nil {
		t.Fatalf("write unit: %v", err)
	}
	tools := `{"tools":[{"id":"nfr-lint","run":"golangci-lint run","verifies":"style floor"}]}`
	if err := os.WriteFile(filepath.Join(".adh", "tools.json"), []byte(tools), 0o600); err != nil {
		t.Fatalf("write tools: %v", err)
	}
	// The arc declares the label the unit routes on (as `arc --label` would seed).
	arc := adh.Arc{
		ID:     "arc-0001",
		Stage:  adh.StageStrategy,
		Status: adh.StatusOpen,
		Labels: []string{"auth"},
	}
	if err := state.Default().Save(&arc); err != nil {
		t.Fatalf("seed arc: %v", err)
	}

	recs := jsonLines(t, mustRun(t, "step", "--relay", "--jsonl", "arc-0001"))
	prompt, ok := okData(t, recs[0])["prompt"].(string)
	if !ok {
		t.Fatalf("emit outcome carried no prompt: %v", recs[0])
	}
	for _, want := range []string{"u-auth", "nfr-lint", "style floor"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("strategy prompt missing %q:\n%s", want, prompt)
		}
	}

	got, err := state.Default().Get("arc-0001")
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if len(got.Context) != 1 || got.Context[0] != "u-auth" {
		t.Errorf("arc.Context = %v, want the loaded working set [u-auth]", got.Context)
	}
}
