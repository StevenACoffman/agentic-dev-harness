package cmd_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLessonListSurfacesEvalCandidates checks that lesson list shows the
// candidate classes the Evaluation loop wrote (§11.1, §19.2), not only the
// confirmed failure registry.
func TestLessonListSurfacesEvalCandidates(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.MkdirAll(".adh", 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// A candidate the eval loop would write for an unconfirmed critic finding.
	candidates := `["oracle: clears differ from the reference"]`
	if err := os.WriteFile(
		filepath.Join(".adh", "lesson-candidates.json"),
		[]byte(candidates),
		0o600,
	); err != nil {
		t.Fatalf("write candidates: %v", err)
	}

	out := mustRun(t, "lesson", "list")
	if !strings.Contains(out, "oracle") {
		t.Errorf("lesson list did not surface the eval candidate class:\n%s", out)
	}
}
