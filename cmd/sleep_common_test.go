package cmd_test

import (
	"strings"
	"testing"
)

// TestSleepStatusReportsCommonPatterns wires the arc-coverage miner end to end:
// two closed arcs share one failure class (100% of arcs), so `sleep status` reports
// it as a common pattern.
func TestSleepStatusReportsCommonPatterns(t *testing.T) {
	t.Chdir(t.TempDir())
	failure := selectionFailure(t)
	seedSleepWorkspace(t, failure)

	out, err := run(t, "sleep", "status")
	if err != nil {
		t.Fatalf("sleep status: %v", err)
	}
	if want := "common failure: " + failure; !strings.Contains(out, want) {
		t.Errorf("sleep status is missing the common-pattern line %q:\n%s", want, out)
	}
}
