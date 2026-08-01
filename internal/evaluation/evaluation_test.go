package evaluation_test

import (
	"context"
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/critic"
	"github.com/StevenACoffman/agentic-dev-harness/internal/evaluation"
	"github.com/StevenACoffman/agentic-dev-harness/internal/failures"
	"github.com/StevenACoffman/agentic-dev-harness/internal/nfr"
	"github.com/StevenACoffman/agentic-dev-harness/internal/toolreg"
)

// fakeAdjudicator confirms exactly the findings whose kind is failKind.
type fakeAdjudicator struct{ failKind adh.FindingKind }

// fakeRunner reports a fixed (passed, ran) for a check and a fixed (value, measured)
// for a Meter, so an NFR check's disposition is exercised without spawning a process.
type fakeRunner struct {
	passed, ran bool
	value       float64
	measured    bool
}

func (f fakeAdjudicator) Adjudicate(
	_ context.Context,
	finding adh.Finding,
) (ran, failed bool, err error) {
	return true, finding.Kind == f.failKind, nil
}

func (f fakeRunner) RunCheck(_ context.Context, _, _ string) (passed, ran bool) {
	return f.passed, f.ran
}

func (f fakeRunner) Measure(_ context.Context, _, _ string) (value float64, ran bool) {
	return f.value, f.measured
}

func TestAdjudicateSplitsConfirmed(t *testing.T) {
	findings := []adh.Finding{
		{Summary: "real", Kind: adh.FindingDevice},
		{Summary: "spurious", Kind: adh.FindingOracle},
	}
	v, err := evaluation.Adjudicate(
		context.Background(),
		fakeAdjudicator{failKind: adh.FindingDevice},
		findings,
	)
	if err != nil {
		t.Fatalf("Adjudicate: %v", err)
	}
	if len(v.Confirmed) != 1 || v.Confirmed[0].Summary != "real" {
		t.Errorf("confirmed = %+v, want [real]", v.Confirmed)
	}
	if len(v.Unconfirmed) != 1 {
		t.Errorf("unconfirmed = %+v, want 1", v.Unconfirmed)
	}
}

func TestApplyConfirmedReturnsToExecution(t *testing.T) {
	t.Chdir(t.TempDir())
	arc := adh.Arc{
		ID:       "arc-0001",
		Stage:    adh.StageEvaluation,
		Status:   adh.StatusOpen,
		Findings: []adh.Finding{{Kind: adh.FindingDevice}},
	}
	v := critic.Verdict{
		Confirmed: []adh.Finding{{Summary: "screen wrong", Kind: adh.FindingDevice}},
	}
	if err := evaluation.Apply(&arc, &v, true, evaluation.DefaultMaxReworks); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if arc.Stage != adh.StageExecution || arc.Status != adh.StatusOpen {
		t.Errorf("(stage, status) = (%s, %s), want (execution, open)", arc.Stage, arc.Status)
	}
	if arc.Reworks != 1 {
		t.Errorf("reworks = %d, want 1 (a rework within budget)", arc.Reworks)
	}
	if len(arc.Findings) != 0 {
		t.Errorf("findings not cleared: %+v", arc.Findings)
	}
	notes, _ := failures.Load(failures.RegistryFile)
	if len(notes) != 1 {
		t.Errorf("failure registry = %v, want one confirmed entry", notes)
	}
}

// TestApplyFailsTerminallyPastBudget: once an arc has spent its rework budget, a
// still-confirmed finding fails it terminally (StatusFailed) rather than looping —
// the "loop ends" closure (SPEC §4.1, §19.3).
func TestApplyFailsTerminallyPastBudget(t *testing.T) {
	t.Chdir(t.TempDir())
	arc := adh.Arc{
		ID:       "arc-0001",
		Stage:    adh.StageEvaluation,
		Status:   adh.StatusOpen,
		Reworks:  evaluation.DefaultMaxReworks, // budget already spent
		Findings: []adh.Finding{{Kind: adh.FindingDevice}},
	}
	v := critic.Verdict{
		Confirmed: []adh.Finding{{Summary: "still wrong", Kind: adh.FindingDevice}},
	}
	if err := evaluation.Apply(&arc, &v, true, evaluation.DefaultMaxReworks); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if arc.Status != adh.StatusFailed {
		t.Errorf("status = %s, want failed (rework budget spent)", arc.Status)
	}
	if arc.Reworks != evaluation.DefaultMaxReworks {
		t.Errorf("reworks = %d, want it unchanged at the budget on a terminal fail", arc.Reworks)
	}
	if len(arc.Findings) != 0 {
		t.Errorf("findings not cleared: %+v", arc.Findings)
	}
	if notes, _ := failures.Load(failures.RegistryFile); len(notes) != 1 {
		t.Errorf("failure registry = %v, want the terminal failure recorded", notes)
	}
}

// TestDecide covers the three-way disposition and the budget boundary.
func TestDecide(t *testing.T) {
	t.Parallel()
	confirmed := &critic.Verdict{Confirmed: []adh.Finding{{Kind: adh.FindingDevice}}}
	clean := &critic.Verdict{Unconfirmed: []adh.Finding{{Kind: adh.FindingOracle}}}
	structural := &critic.Verdict{Confirmed: []adh.Finding{
		{Kind: adh.FindingContract, Class: adh.StructuralFinding},
	}}
	tests := []struct {
		name    string
		verdict *critic.Verdict
		reworks int
		max     int
		want    evaluation.Disposition
	}{
		{"clean advances", clean, 0, 2, evaluation.AdvanceToOps},
		{"confirmed under budget reworks", confirmed, 0, 2, evaluation.ReturnToExecution},
		{"confirmed one below budget reworks", confirmed, 1, 2, evaluation.ReturnToExecution},
		{"confirmed at budget fails", confirmed, 2, 2, evaluation.Fail},
		{"zero budget fails on first confirm", confirmed, 0, 0, evaluation.Fail},
		{"structural escalates under budget", structural, 0, 2, evaluation.Fail},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := evaluation.Decide(tt.verdict, tt.reworks, tt.max); got != tt.want {
				t.Errorf("Decide(reworks=%d, max=%d) = %d, want %d",
					tt.reworks, tt.max, got, tt.want)
			}
		})
	}
}

func TestApplyUnconfirmedAdvancesAndRecordsLesson(t *testing.T) {
	t.Chdir(t.TempDir())
	arc := adh.Arc{ID: "arc-0001", Stage: adh.StageEvaluation}
	v := critic.Verdict{Unconfirmed: []adh.Finding{{Summary: "hunch", Kind: adh.FindingOracle}}}
	if err := evaluation.Apply(&arc, &v, true, evaluation.DefaultMaxReworks); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if arc.Stage != adh.StageOps {
		t.Errorf("stage = %s, want ops", arc.Stage)
	}
	candidates, _ := failures.Load(failures.CandidatesFile)
	if len(candidates) != 1 {
		t.Errorf("lesson candidates = %v, want one", candidates)
	}
}

func TestApplyUnconfirmedSkipsLessonWhenDisabled(t *testing.T) {
	t.Chdir(t.TempDir())
	arc := adh.Arc{ID: "arc-0001", Stage: adh.StageEvaluation}
	v := critic.Verdict{Unconfirmed: []adh.Finding{{Summary: "hunch", Kind: adh.FindingOracle}}}
	if err := evaluation.Apply(&arc, &v, false, evaluation.DefaultMaxReworks); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if candidates, _ := failures.Load(failures.CandidatesFile); len(candidates) != 0 {
		t.Errorf("recordLessons=false still wrote candidates: %v", candidates)
	}
}

// TestRepoAdjudicatorContractFailsOnMissingProof is the real confirmed path: a
// contract finding naming an absent manifest fails.
func TestRepoAdjudicatorContractFailsOnMissingProof(t *testing.T) {
	t.Chdir(t.TempDir())
	ran, failed, err := (&evaluation.RepoAdjudicator{}).Adjudicate(
		context.Background(),
		adh.Finding{Kind: adh.FindingContract, Ref: ".adh/proof/missing.json"},
	)
	if err != nil {
		t.Fatalf("Adjudicate: %v", err)
	}
	if !ran || !failed {
		t.Errorf("missing-proof contract = (ran %v, failed %v), want (true, true)", ran, failed)
	}
}

func TestRepoAdjudicatorNFR(t *testing.T) {
	t.Parallel()
	reg := toolreg.Registry{Tools: []toolreg.Tool{
		{ID: "nfr-lint", Run: "golangci-lint run", Verifies: "style floor"},
	}}
	tests := []struct {
		name         string
		ref          string
		runner       evaluation.CheckRunner
		wantRan      bool
		wantFailed   bool
		wantConfirms bool
	}{
		{
			"declared check fails",
			"nfr-lint",
			fakeRunner{passed: false, ran: true},
			true,
			true,
			true,
		},
		{
			"declared check passes",
			"nfr-lint",
			fakeRunner{passed: true, ran: true},
			true,
			false,
			false,
		},
		{"check could not start", "nfr-lint", fakeRunner{ran: false}, false, false, false},
		{"undeclared ref", "nfr-absent", fakeRunner{passed: false, ran: true}, false, false, false},
		{"empty ref", "", fakeRunner{passed: false, ran: true}, false, false, false},
		{"no runner wired", "nfr-lint", nil, false, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			adj := evaluation.NewRepoAdjudicator(t.TempDir(), reg, nil, tt.runner)
			ran, failed, err := adj.Adjudicate(
				context.Background(),
				adh.Finding{Summary: "slow path", Kind: adh.FindingNFR, Ref: tt.ref},
			)
			if err != nil {
				t.Fatalf("Adjudicate: %v", err)
			}
			if ran != tt.wantRan || failed != tt.wantFailed {
				t.Errorf("(ran, failed) = (%v, %v), want (%v, %v)",
					ran, failed, tt.wantRan, tt.wantFailed)
			}
			// A finding confirms only when its check ran and failed (critic.Dispose).
			if confirms := ran && failed; confirms != tt.wantConfirms {
				t.Errorf("confirms = %v, want %v", confirms, tt.wantConfirms)
			}
		})
	}
}

// TestRepoAdjudicatorNFRSpec: an NFR finding whose Ref names a Planguage spec gates
// on the spec's Fail threshold — the measured Meter value, not a tool exit code. A
// value that breaches Fail confirms; one that meets it does not; an unmeasurable or
// undeclared Meter is unrunnable (unconfirmed), so the gate drops what it cannot
// measure (§10.5, §19.2).
func TestRepoAdjudicatorNFRSpec(t *testing.T) {
	t.Parallel()
	reg := toolreg.Registry{Tools: []toolreg.Tool{
		{ID: "bench-latency", Run: "bench", Verifies: "p95 latency ms"},
	}}
	spec := nfr.Spec{
		ID: "latency", Tag: "Performance.Latency", Scale: "ms",
		Meter: "bench-latency", Direction: nfr.Lower, Fail: 300, Goal: 200,
	}
	ghost := spec
	ghost.Meter = "undeclared-tool"
	tests := []struct {
		name       string
		spec       nfr.Spec
		runner     fakeRunner
		wantRan    bool
		wantFailed bool
	}{
		{"measured breaches Fail", spec, fakeRunner{value: 350, measured: true}, true, true},
		{"measured meets Fail", spec, fakeRunner{value: 250, measured: true}, true, false},
		{"unmeasurable output", spec, fakeRunner{measured: false}, false, false},
		{"meter not declared", ghost, fakeRunner{value: 350, measured: true}, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			adj := evaluation.NewRepoAdjudicator(t.TempDir(), reg, []nfr.Spec{tt.spec}, tt.runner)
			ran, failed, err := adj.Adjudicate(
				context.Background(),
				adh.Finding{Summary: "too slow", Kind: adh.FindingNFR, Ref: "latency"},
			)
			if err != nil {
				t.Fatalf("Adjudicate: %v", err)
			}
			if ran != tt.wantRan || failed != tt.wantFailed {
				t.Errorf("(ran, failed) = (%v, %v), want (%v, %v)",
					ran, failed, tt.wantRan, tt.wantFailed)
			}
		})
	}
}
