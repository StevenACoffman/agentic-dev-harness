package metrics_test

import (
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/internal/metrics"
)

func TestClassifyHistory(t *testing.T) {
	history := []string{
		"strategy: chose change",            // model
		"execution: wrote the fix",          // model
		"critic: looks good",                // model
		"evaluation: no findings confirmed", // deterministic
		"committed abc123 on adh/arc-0001",  // deterministic
		"closed as change",                  // deterministic
		"gate approved",                     // deterministic
		"some unclassified note",            // ignored
	}
	c := metrics.ClassifyHistory(history)
	if c.Model != 3 || c.Deterministic != 4 {
		t.Fatalf("ClassifyHistory = %+v, want model 3 / deterministic 4", c)
	}
	if got := c.Ratio(); got < 0.57 || got > 0.58 {
		t.Errorf("Ratio = %.3f, want ~0.571 (4 of 7 classified)", got)
	}
	if (metrics.StepClass{}).Ratio() != 0 {
		t.Errorf("empty Ratio should be 0")
	}
}
