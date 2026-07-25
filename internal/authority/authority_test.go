package authority_test

import (
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/authority"
)

func TestParseLevelRoundTrip(t *testing.T) {
	for _, s := range []string{"L0", "L1", "L2", "L3", "L4"} {
		t.Run(s, func(t *testing.T) {
			lvl, err := authority.ParseLevel(s)
			if err != nil {
				t.Fatalf("ParseLevel(%q): %v", s, err)
			}
			if lvl.String() != s {
				t.Errorf("round trip = %q, want %q", lvl.String(), s)
			}
		})
	}
}

func TestParseLevelInvalid(t *testing.T) {
	if _, err := authority.ParseLevel("L9"); adh.ErrorCode(err) != adh.EINVALID {
		t.Errorf("ParseLevel(L9) code = %q, want invalid", adh.ErrorCode(err))
	}
}

func TestRaiseIsGated(t *testing.T) {
	if !authority.RaiseIsGated(authority.L2, authority.L3) {
		t.Errorf("raising L2->L3 must be gated")
	}
	if authority.RaiseIsGated(authority.L3, authority.L1) {
		t.Errorf("lowering L3->L1 must not be gated")
	}
	if authority.RaiseIsGated(authority.L2, authority.L2) {
		t.Errorf("staying at L2 must not be gated")
	}
}

func TestModelGate(t *testing.T) {
	tests := []struct {
		name     string
		role     adh.Stage
		class    authority.ModelClass
		wantCode string
	}{
		{
			name:     "critic on reasoning",
			role:     adh.StageCritic,
			class:    authority.ClassReasoning,
			wantCode: "",
		},
		{
			name:     "critic on fast",
			role:     adh.StageCritic,
			class:    authority.ClassFast,
			wantCode: adh.EUNAUTHORIZED,
		},
		{
			name:     "strategy on fast",
			role:     adh.StageStrategy,
			class:    authority.ClassFast,
			wantCode: adh.EUNAUTHORIZED,
		},
		{
			name:     "execution on fast",
			role:     adh.StageExecution,
			class:    authority.ClassFast,
			wantCode: "",
		},
		{name: "ops on fast", role: adh.StageOps, class: authority.ClassFast, wantCode: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := adh.ErrorCode(authority.ModelGate(tt.role, tt.class)); got != tt.wantCode {
				t.Errorf("ModelGate = %q, want %q", got, tt.wantCode)
			}
		})
	}
}

func TestGateSatisfied(t *testing.T) {
	tests := []struct {
		name               string
		required, provided string
		dryRun             bool
		want               bool
	}{
		{name: "exact match", required: "ship it", provided: "ship it", want: true},
		{name: "wrong phrase", required: "ship it", provided: "nope", want: false},
		{name: "empty required", required: "", provided: "", want: false},
		{
			name:     "match under dry run",
			required: "ship it",
			provided: "ship it",
			dryRun:   true,
			want:     false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := authority.GateSatisfied(tt.required, tt.provided, tt.dryRun); got != tt.want {
				t.Errorf("GateSatisfied = %v, want %v", got, tt.want)
			}
		})
	}
}
