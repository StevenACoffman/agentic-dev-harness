package harnesscheck_test

import (
	"testing"

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
