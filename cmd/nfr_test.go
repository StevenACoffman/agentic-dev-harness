package cmd_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/cmd/root"
	"github.com/StevenACoffman/agentic-dev-harness/internal/nfr"
)

const objectiveSpec = `{"id":"latency","tag":"Performance.Latency","scale":"ms",` +
	`"meter":"bench","direction":"lower","fail":300,"goal":200}`

const guardSpec = `{"id":"footprint","tag":"Performance.Memory","scale":"MB",` +
	`"meter":"bench","direction":"lower","fail":512,"goal":256,"guard":true}`

// writeSpec writes one NFR spec into .adh/nfr for the CLI to load.
func writeSpec(t *testing.T, name, body string) {
	t.Helper()
	if err := os.MkdirAll(nfr.DefaultDir, 0o750); err != nil {
		t.Fatalf("mkdir nfr: %v", err)
	}
	if err := os.WriteFile(filepath.Join(nfr.DefaultDir, name), []byte(body), 0o600); err != nil {
		t.Fatalf("write spec: %v", err)
	}
}

// TestNFRLintValid: a well-formed Planguage spec lints clean.
func TestNFRLintValid(t *testing.T) {
	t.Chdir(t.TempDir())
	writeSpec(
		t,
		"latency.json",
		`{"id":"latency","tag":"Performance.Latency","scale":"ms","meter":"bench","direction":"lower","fail":300,"goal":200}`,
	)
	out := mustRun(t, "nfr", "lint")
	if !strings.Contains(out, "1 NFR spec(s), all valid") {
		t.Errorf("lint output = %q, want all valid", out)
	}
}

// TestNFRLintInvalid: a mis-ordered or mis-tagged spec fails lint (exit 17).
func TestNFRLintInvalid(t *testing.T) {
	t.Chdir(t.TempDir())
	writeSpec(
		t,
		"bad.json",
		`{"id":"bad","tag":"Vibes.Speed","scale":"ms","meter":"bench","direction":"lower","fail":100,"goal":200}`,
	)
	_, err := run(t, "nfr", "lint")
	var exit root.ExitError
	if !errors.As(err, &exit) || int(exit) != 17 {
		t.Fatalf("lint of an invalid spec = %v, want ExitError(17)", err)
	}
}

// TestNFRListShowsWhichAreGuards. nfr.Spec.Guard shipped with no reader, so the answer
// to "what will this change be held to?" was a grep over the spec files — and the
// failure a guard exists to prevent is an author optimising an objective without
// knowing what it must not cost. The column is always shown for that reason: a filter
// would only reach someone who already suspected guards mattered.
func TestNFRListShowsWhichAreGuards(t *testing.T) {
	t.Chdir(t.TempDir())
	writeSpec(t, "latency.json", objectiveSpec)
	writeSpec(t, "footprint.json", guardSpec)

	out, err := run(t, "nfr", "list")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if !strings.Contains(out, "GUARD") {
		t.Errorf("the listing does not mark the guard:\n%s", out)
	}
	if !strings.Contains(out, "objective") {
		t.Errorf("the listing does not mark the objective:\n%s", out)
	}
}

// TestNFRListGuardsFilters, for the other question — "show me only what constrains this
// change" — which is worth asking once the list is long.
func TestNFRListGuardsFilters(t *testing.T) {
	t.Chdir(t.TempDir())
	writeSpec(t, "latency.json", objectiveSpec)
	writeSpec(t, "footprint.json", guardSpec)

	out, err := run(t, "nfr", "--guards", "list")
	if err != nil {
		t.Fatalf("list --guards: %v", err)
	}
	if !strings.Contains(out, "footprint") {
		t.Errorf("the guard is missing from --guards:\n%s", out)
	}
	if strings.Contains(out, "latency") {
		t.Errorf("--guards listed an objective:\n%s", out)
	}
}

// TestNFRRefusesAFlagAfterTheVerb. ff stops parsing flags at the first positional, so
// `nfr list --guards` left the flag unset and listed everything — the caller believing
// a filter applied. A silently-ignored flag is worse than an unsupported one. Found by
// running the command rather than by a test, which is why there is now a test.
func TestNFRRefusesAFlagAfterTheVerb(t *testing.T) {
	t.Chdir(t.TempDir())
	writeSpec(t, "latency.json", objectiveSpec)

	_, err := run(t, "nfr", "list", "--guards")
	if err == nil {
		t.Fatal("a flag after the verb was silently ignored")
	}
	// The message names the form that works, because that is what a reader hitting
	// this wants.
	if !strings.Contains(err.Error(), "nfr --guards list") {
		t.Errorf("the error does not give the working form: %v", err)
	}
}
