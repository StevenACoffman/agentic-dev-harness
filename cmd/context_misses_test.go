package cmd_test

import (
	"strings"
	"testing"
)

// TestRoutingGapRecordsMissAndTemporalGate: each critic routing gap (§19.1, exit 12)
// is appended to the miss log; but two misses in one time stratum do NOT earn a route
// — the temporal-stratum gate requires the pattern to be sustained across strata
// (§10.3), so a same-day burst is recorded yet not proposed.
func TestRoutingGapRecordsMissAndTemporalGate(t *testing.T) {
	t.Chdir(t.TempDir())
	// A populated store that routes nothing for the arc's "auth" label: a gap, not
	// an ungrounded (empty-store) case.
	writeContextUnit(t, "u-billing", []string{"billing"})
	id := seedCriticArc(t, []string{"auth"}, nil)

	// The arc stays at the critic after a gap, so a second step gaps again — two
	// recorded misses for the same label, but stamped in the same (current) stratum.
	for range 2 {
		if _, err := run(t, "step", "--relay", id); err == nil {
			t.Fatal("a routing gap should exit non-zero")
		}
	}

	out := mustRun(t, "context", "misses")
	if !strings.Contains(out, "2 routing miss") {
		t.Errorf("misses output should report 2 recorded misses:\n%s", out)
	}
	// Same-stratum misses are below the temporal gate: recorded, not proposed.
	if strings.Contains(out, "propose") {
		t.Errorf("two same-stratum misses must not propose a route (temporal gate):\n%s", out)
	}
}
