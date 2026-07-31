package cmd_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestHarnessEvalSkillsaw: `harness eval --skillsaw` decodes a skillsaw eval and
// reports its score beside adh's floor — skillsaw as the cheap floor under adh's bar.
func TestHarnessEvalSkillsaw(t *testing.T) {
	dir := t.TempDir()
	artifact := filepath.Join(dir, "guide.md")
	if err := os.WriteFile(
		artifact,
		[]byte("# Guide\n\n## Failures\n\nRoll back.\n"),
		0o600,
	); err != nil {
		t.Fatalf("write artifact: %v", err)
	}
	saw := filepath.Join(dir, "s.json")
	if err := os.WriteFile(
		saw,
		[]byte(`{"score":81.0,"dimensions":[{"name":"clarity","score":0.7,"needs_judge":true}]}`),
		0o600,
	); err != nil {
		t.Fatalf("write skillsaw: %v", err)
	}
	out := mustRun(t, "harness", "eval", "--skillsaw", saw, artifact)
	if !strings.Contains(out, "skillsaw: score 81.00") || !strings.Contains(out, "clarity") {
		t.Errorf("eval --skillsaw output missing the skillsaw line:\n%s", out)
	}
}
