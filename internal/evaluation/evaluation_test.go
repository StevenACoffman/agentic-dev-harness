package evaluation_test

import (
	"context"
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/critic"
	"github.com/StevenACoffman/agentic-dev-harness/internal/evaluation"
	"github.com/StevenACoffman/agentic-dev-harness/internal/failures"
	"github.com/StevenACoffman/agentic-dev-harness/internal/toolreg"
)

// fakeAdjudicator confirms exactly the findings whose kind is failKind.
type fakeAdjudicator struct{ failKind adh.FindingKind }

// fakeRunner reports a fixed (passed, ran) for any command, so an NFR check's
// disposition is exercised without spawning a process.
type fakeRunner struct{ passed, ran bool }

func (f fakeAdjudicator) Adjudicate(
	_ context.Context,
	finding adh.Finding,
) (ran, failed bool, err error) {
	return true, finding.Kind == f.failKind, nil
}

func (f fakeRunner) RunCheck(_ context.Context, _, _ string) (passed, ran bool) {
	return f.passed, f.ran
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
		Findings: []adh.Finding{{Kind: adh.FindingDevice}},
	}
	v := critic.Verdict{
		Confirmed: []adh.Finding{{Summary: "screen wrong", Kind: adh.FindingDevice}},
	}
	if err := evaluation.Apply(&arc, &v, true); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if arc.Stage != adh.StageExecution {
		t.Errorf("stage = %s, want execution", arc.Stage)
	}
	if len(arc.Findings) != 0 {
		t.Errorf("findings not cleared: %+v", arc.Findings)
	}
	notes, _ := failures.Load(failures.RegistryFile)
	if len(notes) != 1 {
		t.Errorf("failure registry = %v, want one confirmed entry", notes)
	}
}

func TestApplyUnconfirmedAdvancesAndRecordsLesson(t *testing.T) {
	t.Chdir(t.TempDir())
	arc := adh.Arc{ID: "arc-0001", Stage: adh.StageEvaluation}
	v := critic.Verdict{Unconfirmed: []adh.Finding{{Summary: "hunch", Kind: adh.FindingOracle}}}
	if err := evaluation.Apply(&arc, &v, true); err != nil {
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
	if err := evaluation.Apply(&arc, &v, false); err != nil {
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
	ran, failed, err := evaluation.RepoAdjudicator{}.Adjudicate(
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
			adj := evaluation.NewRepoAdjudicator(t.TempDir(), reg, tt.runner)
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
