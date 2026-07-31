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
