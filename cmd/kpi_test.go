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
