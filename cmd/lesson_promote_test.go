package cmd_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/cmd/root"
)

// seedFailures writes a failure registry so a class is a promotable candidate.
func seedFailures(t *testing.T, notesJSON string) {
	t.Helper()
	if err := os.MkdirAll(".adh", 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(".adh", "failures.json"),
		[]byte(notesJSON),
		0o600,
	); err != nil {
		t.Fatalf("seed failures: %v", err)
	}
}

// seedRecords writes the stamped failure-record log so a class clears the §11
// temporal gate and carries its routing scope and root cause.
func seedRecords(t *testing.T, recordsJSON string) {
	t.Helper()
	if err := os.MkdirAll(".adh", 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(".adh", "failure-records.json"),
		[]byte(recordsJSON),
		0o600,
	); err != nil {
		t.Fatalf("seed records: %v", err)
	}
}

// TestLessonPromoteMaterializesRoutableUnit: promoting a class that recurred across
// ≥2 strata to a reversible owner writes a real §10 context unit that `context
// show`/`route` then see, tagged with the scope it recurred under — the correction
// becomes accretive (§11.2, §19.2), not a printed intent.
func TestLessonPromoteMaterializesRoutableUnit(t *testing.T) {
	t.Chdir(t.TempDir())
	seedFailures(t, `["oracle: clears differ from reference", "oracle: seed drift"]`)
	seedRecords(t, `[
		{"class":"oracle","stratum":"2026-06","labels":["ui"],"paths":["board.go"],"root_cause":"ungrounded"},
		{"class":"oracle","stratum":"2026-07","labels":["ui"],"root_cause":"grounded-miss"}
	]`)

	out := mustRun(t, "lesson", "--to", "decision", "promote", "oracle")
	if !strings.Contains(out, "oracle-decision") {
		t.Fatalf("promote did not report the written unit:\n%s", out)
	}
	if !strings.Contains(out, "grounded-miss") || !strings.Contains(out, "ungrounded") {
		t.Errorf("promote did not surface the root-cause triage:\n%s", out)
	}
	// The unit is routable and shows its ADR content.
	show := mustRun(t, "context", "show", "oracle-decision")
	for _, want := range []string{"# Decision: oracle", "## Context", "clears differ", "### Easier", "### Harder"} {
		if !strings.Contains(show, want) {
			t.Errorf("promoted decision unit missing %q:\n%s", want, show)
		}
	}
	// It routes both by its class label and the scope it recurred under (§19.2).
	for _, footprint := range []string{"oracle", "ui"} {
		if routed := mustRun(
			t,
			"context",
			"route",
			footprint,
		); !strings.Contains(
			routed,
			"oracle-decision",
		) {
			t.Errorf("promoted unit did not route by %q: %q", footprint, routed)
		}
	}
}

// TestLessonPromoteBlockedByStrataGate: a class that recurred in only one time
// stratum cannot promote (exit 19) — accretion requires a pattern across time (§11).
func TestLessonPromoteBlockedByStrataGate(t *testing.T) {
	t.Chdir(t.TempDir())
	seedFailures(t, `["oracle: once"]`)
	seedRecords(t, `[{"class":"oracle","stratum":"2026-07"}]`)
	_, err := run(t, "lesson", "--to", "context", "promote", "oracle")
	var exit root.ExitError
	if !errors.As(err, &exit) || int(exit) != 19 {
		t.Fatalf("single-stratum promotion = %v, want ExitError(19)", err)
	}
	if _, statErr := os.Stat(filepath.Join(".adh", "context")); !os.IsNotExist(statErr) {
		t.Errorf("gated promotion wrote to the context store: %v", statErr)
	}
}

// TestLessonPromoteExecutableGates: an executable owner still needs approval
// (exit 13) and writes nothing.
func TestLessonPromoteExecutableGates(t *testing.T) {
	t.Chdir(t.TempDir())
	seedFailures(t, `["invariant: board must clear"]`)
	seedRecords(t, `[
		{"class":"invariant","stratum":"2026-06"},
		{"class":"invariant","stratum":"2026-07"}
	]`)

	_, err := run(t, "lesson", "--to", "check", "promote", "invariant")
	var exit root.ExitError
	if !errors.As(err, &exit) || int(exit) != 13 {
		t.Fatalf("executable promotion = %v, want ExitError(13)", err)
	}
	if _, statErr := os.Stat(filepath.Join(".adh", "context")); !os.IsNotExist(statErr) {
		t.Errorf("gated promotion wrote to the context store: %v", statErr)
	}
}

// TestLessonPromoteUnknownClass errors on a class with no candidate.
func TestLessonPromoteUnknownClass(t *testing.T) {
	t.Chdir(t.TempDir())
	seedFailures(t, `["oracle: x"]`)
	if _, err := run(t, "lesson", "--to", "context", "promote", "nope"); err == nil {
		t.Errorf("promoting an unknown class should error")
	}
}
