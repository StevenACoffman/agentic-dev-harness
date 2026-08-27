package cmd_test

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/state"
	"github.com/StevenACoffman/agentic-dev-harness/internal/toolreg"
)

// seedGuardedRepo prepares a repository whose only NFR guard is metered by a tool that
// always fails — a planted defect the repository itself declares it will not tolerate.
//
// declareGuard false leaves the same failing tool declared but no guard naming it, which
// is the uncovered case: the defect is present and nothing in the repository speaks for
// it.
func seedGuardedRepo(t *testing.T, declareGuard bool) {
	t.Helper()
	mustRun(t, "init")
	// The meter must emit a parseable measurement, not merely exit non-zero: a tool
	// that prints nothing is unrunnable, and an unrunnable check does not block (§19.2).
	// 999 breaches the guard's fail bar of 300 on a lower-is-better scale.
	reg := toolreg.Registry{Tools: []toolreg.Tool{{
		ID: "planted-defect", Run: "echo 999", Verifies: "the planted defect is absent",
		Adjudicates: []adh.FindingKind{adh.FindingNFR},
	}}}
	data, err := json.Marshal(reg)
	if err != nil {
		t.Fatalf("marshal registry: %v", err)
	}
	if err := os.WriteFile(toolreg.DefaultRegistryFile, data, 0o600); err != nil {
		t.Fatalf("write registry: %v", err)
	}
	if declareGuard {
		writeSpec(t, "planted.json",
			`{"id":"planted-defect","tag":"Performance.Latency","scale":"ms",`+
				`"meter":"planted-defect","direction":"lower","fail":300,"goal":200,`+
				`"guard":true}`)
	}
}

// TestSeededDefectIsCaughtWhenAGuardCoversIt extends the negative control from the gate
// to the whole arc.
//
// `oracle selftest` proves a planted harmful *edit* is rejected by the selection gate.
// This proves a planted *defect* is caught by an arc — and the interesting part is that
// **the critic never mentions it**. `adh run` without --relay completes through
// model.Mock, whose critic reply is fixed text yielding no findings, so this arc runs
// with a silent critic by construction. What catches the defect is the guard, which
// §10.5 adjudicates whether or not the critic raised it.
func TestSeededDefectIsCaughtWhenAGuardCoversIt(t *testing.T) {
	t.Chdir(t.TempDir())
	seedGuardedRepo(t, true)
	id := strings.TrimSpace(mustRun(t, "arc", "new", "ship me"))
	// The arc parks at the ops gate only if nothing blocked it earlier; a confirmed
	// guard finding returns it to Execution instead.
	_, _ = run(t, "run", id)
	arc, err := state.Default().Get(id)
	if err != nil {
		t.Fatalf("get arc: %v", err)
	}
	if arc.Stage == adh.StageOps {
		t.Errorf("a guarded planted defect reached the ops gate; the guard did not fire")
	}
}

// TestSeededDefectPassesASilentCriticWhenNoGuardCoversIt asserts a **limit, not a bug**.
//
// §19.2 states it: artifacts decide what blocks, and narration decides what gets
// examined. A critic that finds nothing leaves nothing to adjudicate, so the arc
// advances — bounded only by the guards, which is what the previous test exercises. The
// artifacts audit (SPEC §9.0) names this as the gap nobody had measured; this is the
// measurement, and it is a test rather than a sentence so that it **breaks if the
// boundary ever moves in either direction**.
//
// Do not "fix" this by making the arc block. Closing the gap is a design change with its
// own decision (a §12 applicability rule, since a decision-resolution arc may have
// nothing runnable), and this test is what would tell you the change worked.
func TestSeededDefectPassesASilentCriticWhenNoGuardCoversIt(t *testing.T) {
	t.Chdir(t.TempDir())
	seedGuardedRepo(t, false)
	id := strings.TrimSpace(mustRun(t, "arc", "new", "ship me"))
	_, _ = run(t, "run", id)
	arc, err := state.Default().Get(id)
	if err != nil {
		t.Fatalf("get arc: %v", err)
	}
	if arc.Stage != adh.StageOps {
		t.Errorf("arc stopped at %s; with no guard and a silent critic it advances, "+
			"which is the documented limit this test exists to pin", arc.Stage)
	}
	if len(arc.Findings) != 0 {
		t.Errorf("findings = %v, want none: the mock critic is silent by construction",
			arc.Findings)
	}
}

// TestSeededDefectLeavesEvidenceOfWhatWasChecked guards the reporting half: an arc that
// advanced past a planted defect should still show that nothing was adjudicated, so a
// reader can tell "checked and clean" from "nothing to check".
func TestSeededDefectLeavesEvidenceOfWhatWasChecked(t *testing.T) {
	t.Chdir(t.TempDir())
	seedGuardedRepo(t, false)
	id := strings.TrimSpace(mustRun(t, "arc", "new", "ship me"))
	_, _ = run(t, "run", id)
	arc, err := state.Default().Get(id)
	if err != nil {
		t.Fatalf("get arc: %v", err)
	}
	var evaluated bool
	for _, entry := range arc.History {
		if strings.HasPrefix(entry, "evaluation:") {
			evaluated = true
			if !strings.Contains(entry, "no findings confirmed") {
				t.Errorf("evaluation history = %q, want it to say nothing confirmed", entry)
			}
		}
	}
	if !evaluated {
		t.Error("the arc advanced with no evaluation entry in its history")
	}
}
