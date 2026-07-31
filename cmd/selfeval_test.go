package cmd_test

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/internal/failures"
	"github.com/StevenACoffman/agentic-dev-harness/internal/metrics"
)

func TestSelfevalReportsHealthAndTaxonomy(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.MkdirAll(".adh", 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	ledger, err := json.Marshal([]metrics.Record{
		{Arc: "arc-0001", AttentionMinutes: 30, ComputeTokens: 1000, Accepted: true},
		{Arc: "arc-0002", AttentionMinutes: 10, ComputeTokens: 500, Accepted: false},
	})
	if err != nil {
		t.Fatalf("marshal ledger: %v", err)
	}
	if err := os.WriteFile(metrics.LedgerFile, ledger, 0o600); err != nil {
		t.Fatalf("write ledger: %v", err)
	}
	if err := failures.Append(failures.RegistryFile, "critic: cold review missed X"); err != nil {
		t.Fatalf("seed failures: %v", err)
	}

	out, err := run(t, "selfeval")
	if err != nil {
		t.Fatalf("selfeval: %v", err)
	}
	if !strings.Contains(out, "arcs 2, accepted 1") {
		t.Errorf("health line wrong:\n%s", out)
	}
	if !strings.Contains(out, "critic") {
		t.Errorf("failure taxonomy missing the critic class:\n%s", out)
	}
}

func TestSelfevalEmptyWorkspace(t *testing.T) {
	t.Chdir(t.TempDir())
	out, err := run(t, "selfeval")
	if err != nil {
		t.Fatalf("selfeval: %v", err)
	}
	if !strings.Contains(out, "none recorded") {
		t.Errorf("empty selfeval should note no failures:\n%s", out)
	}
}
