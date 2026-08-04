package cmd_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/cmd/root"
)

// TestOracleDiffCommandsMatch: two commands with identical output are a clean oracle —
// exit 0.
func TestOracleDiffCommandsMatch(t *testing.T) {
	t.Chdir(t.TempDir())
	out := mustRun(
		t,
		"oracle",
		"--reference",
		"printf 'a\\nb\\n'",
		"--candidate",
		"printf 'a\\nb\\n'",
		"diff",
	)
	if !strings.Contains(out, "match") {
		t.Errorf("matching commands = %q, want a match line", out)
	}
}

// TestOracleDiffCommandsDiverge: two commands whose output differs confirm a divergence
// with the oracle gate code (5), so a §13 tool wrapping it confirms an oracle finding.
func TestOracleDiffCommandsDiverge(t *testing.T) {
	t.Chdir(t.TempDir())
	_, err := run(
		t,
		"oracle",
		"--reference",
		"printf 'a\\nb\\n'",
		"--candidate",
		"printf 'a\\nX\\n'",
		"diff",
	)
	var exit root.ExitError
	if !errors.As(err, &exit) || int(exit) != 5 {
		t.Fatalf("diverging commands = %v, want ExitError(5)", err)
	}
}

// TestOracleDiffRequiresBothCommands: command mode needs both --reference and
// --candidate; one alone is a usage error, not a silent fall-through to the board oracle.
func TestOracleDiffRequiresBothCommands(t *testing.T) {
	t.Chdir(t.TempDir())
	if _, err := run(t, "oracle", "--reference", "echo x", "diff"); err == nil {
		t.Error("diff with only --reference should error")
	}
}
