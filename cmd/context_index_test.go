package cmd_test

import (
	"strings"
	"testing"
)

// TestContextIndex: `context index` renders the read-first catalog over the store.
func TestContextIndex(t *testing.T) {
	t.Chdir(t.TempDir())
	writeUnit(t, "rule", "base-rule", "fail secure", "security")
	out := mustRun(t, "context", "index")
	if !strings.Contains(out, "Context index") || !strings.Contains(out, "rule (base-rule)") {
		t.Errorf("index output missing catalog row:\n%s", out)
	}
}
