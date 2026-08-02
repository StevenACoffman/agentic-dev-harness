package cmd_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/contextstore"
)

// writeKPIUnit writes a unit that declares a grounded_miss KPI with the given threshold.
func writeKPIUnit(t *testing.T, id string, labels []string, threshold float64) {
	t.Helper()
	if err := os.MkdirAll(contextstore.DefaultStoreDir, 0o750); err != nil {
		t.Fatalf("mkdir store: %v", err)
	}
	unit := contextstore.Unit{
		ID:     id,
		Kind:   "base-rule",
		Labels: labels,
		KPIs: []adh.KPI{
			{Metric: "grounded_miss", Threshold: threshold, Direction: adh.WorseWhenAbove},
		},
	}
	data, _ := json.MarshalIndent(unit, "", "  ")
	if err := os.WriteFile(
		filepath.Join(contextstore.DefaultStoreDir, id+".json"), data, 0o600,
	); err != nil {
		t.Fatalf("write unit: %v", err)
	}
}

// TestKPIProposesDegradedUnit: a unit whose scope keeps failing while grounded, across
// ≥2 strata, breaches its grounded_miss KPI and earns a proposal (§16/§18). Below the
// replication bar, nothing is proposed.
func TestKPIProposesDegradedUnit(t *testing.T) {
	t.Chdir(t.TempDir())
	writeKPIUnit(t, "ui-rule", []string{"ui"}, 1)
	seedRecords(t, `[
		{"class":"oracle","stratum":"2026-06","labels":["ui"],"root_cause":"grounded-miss"},
		{"class":"oracle","stratum":"2026-07","labels":["ui"],"root_cause":"grounded-miss"}
	]`)
	out := mustRun(t, "kpi")
	if !strings.Contains(out, "ui-rule") || !strings.Contains(out, "grounded_miss") {
		t.Fatalf("kpi did not propose the degraded unit:\n%s", out)
	}

	// One stratum is below the §18.2 replication bar — no proposal.
	t.Chdir(t.TempDir())
	writeKPIUnit(t, "ui-rule", []string{"ui"}, 1)
	seedRecords(t, `[
		{"class":"oracle","stratum":"2026-07","labels":["ui"],"root_cause":"grounded-miss"},
		{"class":"oracle","stratum":"2026-07","labels":["ui"],"root_cause":"grounded-miss"}
	]`)
	clean := mustRun(t, "kpi")
	if !strings.Contains(clean, "no KPI degradations") {
		t.Errorf("single-stratum breach should not propose:\n%s", clean)
	}
}

// TestKPIProposesDegradedTool: a declared tool that ran and failed across ≥2 strata
// breaches its run_failure KPI and earns a proposal naming the tool (§16/§18).
func TestKPIProposesDegradedTool(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.MkdirAll(".adh", 0o750); err != nil {
		t.Fatalf("mkdir .adh: %v", err)
	}
	if err := os.WriteFile(filepath.Join(".adh", "tools.json"), []byte(
		`{"tools":[{"id":"skillsaw-eval","run":"x","verifies":"y","kpis":`+
			`[{"metric":"run_failure","threshold":1,"direction":"above"}]}]}`,
	), 0o600); err != nil {
		t.Fatalf("seed tools: %v", err)
	}
	if err := os.WriteFile(filepath.Join(".adh", "tool-runs.json"), []byte(
		`[{"tool":"skillsaw-eval","stratum":"2026-06","ran":true,"failed":true},`+
			`{"tool":"skillsaw-eval","stratum":"2026-07","ran":true,"failed":true}]`,
	), 0o600); err != nil {
		t.Fatalf("seed tool-runs: %v", err)
	}
	out := mustRun(t, "kpi")
	if !strings.Contains(out, "tool skillsaw-eval") || !strings.Contains(out, "run_failure") {
		t.Fatalf("kpi did not propose the degraded tool:\n%s", out)
	}
}

// TestKPIProposesSlowTool: a tool whose mean run duration exceeds its run_duration_ms
// threshold across ≥2 strata earns a latency proposal (§16/§18).
func TestKPIProposesSlowTool(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.MkdirAll(".adh", 0o750); err != nil {
		t.Fatalf("mkdir .adh: %v", err)
	}
	if err := os.WriteFile(filepath.Join(".adh", "tools.json"), []byte(
		`{"tools":[{"id":"bench","run":"x","verifies":"y","kpis":`+
			`[{"metric":"run_duration_ms","threshold":500,"direction":"above"}]}]}`,
	), 0o600); err != nil {
		t.Fatalf("seed tools: %v", err)
	}
	if err := os.WriteFile(filepath.Join(".adh", "tool-runs.json"), []byte(
		`[{"tool":"bench","stratum":"2026-06","ran":true,"failed":false,"duration_ms":800},`+
			`{"tool":"bench","stratum":"2026-07","ran":true,"failed":false,"duration_ms":900}]`,
	), 0o600); err != nil {
		t.Fatalf("seed tool-runs: %v", err)
	}
	out := mustRun(t, "kpi")
	if !strings.Contains(out, "tool bench") || !strings.Contains(out, "run_duration_ms") {
		t.Fatalf("kpi did not propose the slow tool:\n%s", out)
	}
}
