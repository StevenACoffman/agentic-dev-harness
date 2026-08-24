package harnesscheck_test

import (
	"slices"
	"sort"
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/contextstore"
	"github.com/StevenACoffman/agentic-dev-harness/internal/harnesscheck"
	"github.com/StevenACoffman/agentic-dev-harness/internal/loop"
	"github.com/StevenACoffman/agentic-dev-harness/internal/nfr"
	"github.com/StevenACoffman/agentic-dev-harness/internal/toolreg"
)

func cleanInputs() harnesscheck.Inputs {
	return harnesscheck.Inputs{
		Units: []contextstore.Unit{{ID: "model", Kind: "domain-model", Integrity: "drift-check"}},
		Tools: toolreg.Registry{Tools: []toolreg.Tool{
			{ID: "drift-check", Run: "true", Verifies: "no drift"},
		}},
		Loops: loop.StarterRegistry(),
		Specs: []nfr.Spec{{
			ID: "latency", Tag: "Performance.Latency", Scale: "ms",
			Meter: "bench", Direction: nfr.Lower, Fail: 300, Goal: 200,
		}, {
			// A guard, so the clean fixture is a repository that protects something
			// as well as measuring something. Adding the no-guards check found this
			// fixture had objectives and nothing guarded, which is the gap the check
			// exists to report.
			ID: "footprint", Tag: "Performance.Memory", Scale: "MB",
			Meter: "bench", Direction: nfr.Lower, Fail: 512, Goal: 256, Guard: true,
		}},
	}
}

func TestCheckClean(t *testing.T) {
	in := cleanInputs()
	if problems := harnesscheck.Check(&in); len(problems) != 0 {
		t.Fatalf("Check(clean) = %+v, want none", problems)
	}
}

func TestCheckDanglingIntegrity(t *testing.T) {
	in := cleanInputs()
	in.Units[0].Integrity = "no-such-tool"
	problems := harnesscheck.Check(&in)
	if len(problems) != 1 || problems[0].Kind != harnesscheck.KindDanglingIntegrity {
		t.Fatalf("Check = %+v, want one dangling_integrity", problems)
	}
	if problems[0].Ref != "model" {
		t.Errorf("problem ref = %q, want model", problems[0].Ref)
	}
}

func TestCheckDuplicateUnitAndBadSpec(t *testing.T) {
	in := cleanInputs()
	in.Units = append(in.Units, contextstore.Unit{ID: "model", Kind: "runbook"})
	in.Specs[0].Tag = "Vibes.Speed" // unknown category → invalid spec
	kinds := map[string]bool{}
	for _, p := range harnesscheck.Check(&in) {
		kinds[p.Kind] = true
	}
	if !kinds[harnesscheck.KindDuplicateUnit] {
		t.Error("want a duplicate_unit problem")
	}
	if !kinds[harnesscheck.KindNFRSpec] {
		t.Error("want an nfr_spec problem")
	}
}

// TestNoGuardsIsReported. An NFR spec is either an objective or a guard, and a
// repository with specs and no guards has thought about measurement and not about
// protection — an objective without guards is hill-climbed by trading away everything
// unmeasured.
func TestNoGuardsIsReported(t *testing.T) {
	t.Parallel()
	objective := nfr.Spec{
		ID: "latency", Tag: "Performance.Latency", Scale: "ms",
		Meter: "bench", Direction: nfr.Lower, Fail: 300, Goal: 200,
	}
	guard := objective
	guard.ID, guard.Guard = "footprint", true

	cases := map[string]struct {
		specs []nfr.Spec
		want  bool
	}{
		"objectives and no guard": {[]nfr.Spec{objective}, true},
		"one guard among them":    {[]nfr.Spec{objective, guard}, false},
		// Derived applicability: having declared nothing is not the same as having
		// declared only objectives. The first has not started; reporting it would fire
		// on every fresh tree and teach a reader to ignore the check.
		"no specs at all": {nil, false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			in := cleanInputs()
			in.Specs = tc.specs
			var found bool
			for _, p := range harnesscheck.Check(&in) {
				if p.Kind == harnesscheck.KindNoGuards {
					found = true
				}
			}
			if found != tc.want {
				t.Errorf("no_guards reported = %v, want %v", found, tc.want)
			}
		})
	}
}

// wantUnclaimed asserts which finding kinds a registry leaves unclaimed. The helper is
// extracted before the table so each case reads as the claim it makes about a registry
// rather than as filtering machinery.
func wantUnclaimed(t *testing.T, reg toolreg.Registry, want []string) {
	t.Helper()
	got := make([]string, 0)
	for _, p := range harnesscheck.Check(&harnesscheck.Inputs{Tools: reg}) {
		if p.Kind == harnesscheck.KindUnclaimedKind {
			got = append(got, p.Ref)
		}
	}
	sort.Strings(got)
	if !slices.Equal(got, want) {
		t.Errorf("unclaimed kinds = %v, want %v", got, want)
	}
}

// TestUnclaimedFindingKinds covers the check and, more importantly, where it declines to
// judge: a registry that has not adopted `adjudicates` is not a registry with gaps.
func TestUnclaimedFindingKinds(t *testing.T) {
	t.Parallel()
	tool := func(id string, kinds ...adh.FindingKind) toolreg.Tool {
		return toolreg.Tool{ID: id, Run: "x", Verifies: "y", Adjudicates: kinds}
	}
	tests := []struct {
		name string
		reg  toolreg.Registry
		want []string
	}{{
		// Not adopted: reporting five gaps on a fresh init would be noise on every new
		// repository, and the field's absence says nothing about coverage.
		name: "a registry claiming nothing is not judged",
		reg:  toolreg.Registry{Tools: []toolreg.Tool{tool("a"), tool("b")}},
		want: []string{},
	}, {
		name: "an empty registry is not judged either",
		reg:  toolreg.Registry{},
		want: []string{},
	}, {
		// Opted in, so the gaps are meaningful.
		name: "one claim makes the remaining gaps meaningful",
		reg:  toolreg.Registry{Tools: []toolreg.Tool{tool("a", adh.FindingOracle)}},
		want: []string{"contract", "device", "invariant", "nfr"},
	}, {
		// The tempting rule is that an unclassified tool might cover the rest. That is
		// the silence this check removes: if nothing claims a kind, nothing claims it.
		name: "an unclassified tool alongside a claiming one suppresses nothing",
		reg: toolreg.Registry{Tools: []toolreg.Tool{
			tool("a", adh.FindingOracle), tool("b"),
		}},
		want: []string{"contract", "device", "invariant", "nfr"},
	}, {
		name: "a fully claimed registry reports nothing",
		reg: toolreg.Registry{Tools: []toolreg.Tool{tool("a",
			adh.FindingOracle, adh.FindingInvariant, adh.FindingDevice,
			adh.FindingNFR, adh.FindingContract)}},
		want: []string{},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			wantUnclaimed(t, tt.reg, tt.want)
		})
	}
}

func TestRegistryRejectsUnknownAdjudicatedKind(t *testing.T) {
	t.Parallel()
	reg := toolreg.Registry{Tools: []toolreg.Tool{{
		ID: "a", Run: "x", Verifies: "y",
		Adjudicates: []adh.FindingKind{"teleport"},
	}}}
	if err := reg.Validate(); adh.ErrorCode(err) != adh.EINVALID {
		t.Errorf("Validate with an unknown kind = %v, want EINVALID", err)
	}
}
