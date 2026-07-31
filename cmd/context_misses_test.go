package cmd_test

import (
	"strings"
	"testing"
)

// TestRoutingGapRecordsMissAndProposes: each critic routing gap (§19.1, exit 12) is
// appended to the miss log, and once a label crosses the threshold `context misses`
// proposes a deterministic route for it — routing learning from its misses (§10.3).
func TestRoutingGapRecordsMissAndProposes(t *testing.T) {
	t.Chdir(t.TempDir())
	// A populated store that routes nothing for the arc's "auth" label: a gap, not
	// an ungrounded (empty-store) case.
	writeContextUnit(t, "u-billing", []string{"billing"})
	id := seedCriticArc(t, []string{"auth"}, nil)

	// The arc stays at the critic after a gap, so a second step gaps again — two
	// recorded misses for the same label, reaching the proposal threshold (2).
	for range 2 {
		if _, err := run(t, "step", "--relay", id); err == nil {
			t.Fatal("a routing gap should exit non-zero")
		}
	}

	out := mustRun(t, "context", "misses")
	if !strings.Contains(out, "2 routing miss") {
		t.Errorf("misses output should report 2 recorded misses:\n%s", out)
	}
	if !strings.Contains(out, "propose") || !strings.Contains(out, "auth") {
		t.Errorf("misses output should propose a route for auth:\n%s", out)
	}
}
