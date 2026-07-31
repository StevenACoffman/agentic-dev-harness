package adh_test

import (
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
)

func TestFindingKindValid(t *testing.T) {
	for _, k := range []adh.FindingKind{
		adh.FindingOracle, adh.FindingInvariant, adh.FindingDevice,
		adh.FindingNFR, adh.FindingContract,
	} {
		if !k.Valid() {
			t.Errorf("FindingKind(%q).Valid() = false, want true", k)
		}
	}
	for _, k := range []adh.FindingKind{"", "bogus", "Oracle"} {
		if k.Valid() {
			t.Errorf("FindingKind(%q).Valid() = true, want false", k)
		}
	}
}

func TestParseResolution(t *testing.T) {
	for _, res := range []adh.Resolution{
		adh.ResolutionChange, adh.ResolutionInvestigation,
		adh.ResolutionExperiment, adh.ResolutionDecision,
	} {
		got, err := adh.ParseResolution(string(res))
		if err != nil || got != res {
			t.Errorf("ParseResolution(%q) = %q, %v; want %q, nil", res, got, err, res)
		}
	}
	if _, err := adh.ParseResolution("bogus"); adh.ErrorCode(err) != adh.EINVALID {
		t.Errorf("ParseResolution(bogus) code = %q, want EINVALID", adh.ErrorCode(err))
	}
}

func TestNextStage(t *testing.T) {
	tests := []struct {
		name  string
		in    adh.Stage
		want  adh.Stage
		wantK bool
	}{
		{
			name:  "strategy to execution",
			in:    adh.StageStrategy,
			want:  adh.StageExecution,
			wantK: true,
		},
		{name: "execution to critic", in: adh.StageExecution, want: adh.StageCritic, wantK: true},
		{name: "critic to evaluation", in: adh.StageCritic, want: adh.StageEvaluation, wantK: true},
		{name: "evaluation to ops", in: adh.StageEvaluation, want: adh.StageOps, wantK: true},
		{name: "ops is terminal", in: adh.StageOps, want: "", wantK: false},
		{name: "unknown", in: adh.Stage("bogus"), want: "", wantK: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := adh.NextStage(tt.in)
			if got != tt.want || ok != tt.wantK {
				t.Errorf(
					"NextStage(%q) = (%q, %v), want (%q, %v)",
					tt.in,
					got,
					ok,
					tt.want,
					tt.wantK,
				)
			}
		})
	}
}

func TestCanClose(t *testing.T) {
	tests := []struct {
		name     string
		res      adh.Resolution
		hasProof bool
		wantCode string
	}{
		{name: "change with proof", res: adh.ResolutionChange, hasProof: true, wantCode: ""},
		{
			name:     "investigation with proof",
			res:      adh.ResolutionInvestigation,
			hasProof: true,
			wantCode: "",
		},
		{
			name:     "change without proof",
			res:      adh.ResolutionChange,
			hasProof: false,
			wantCode: adh.ECONFLICT,
		},
		{name: "no resolution", res: adh.Resolution(""), hasProof: true, wantCode: adh.EINVALID},
		{
			name:     "unknown resolution",
			res:      adh.Resolution("bogus"),
			hasProof: true,
			wantCode: adh.EINVALID,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := adh.CanClose(&adh.Arc{Resolution: tt.res}, tt.hasProof)
			if got := adh.ErrorCode(err); got != tt.wantCode {
				t.Errorf("CanClose code = %q, want %q (err=%v)", got, tt.wantCode, err)
			}
		})
	}
}
