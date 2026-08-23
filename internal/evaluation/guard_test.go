package evaluation_test

import (
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/evaluation"
	"github.com/StevenACoffman/agentic-dev-harness/internal/nfr"
)

func spec(id string, guard bool) nfr.Spec {
	return nfr.Spec{
		ID: id, Tag: "Performance.Latency", Scale: "ms", Meter: "bench",
		Direction: nfr.Lower, Fail: 100, Goal: 50, Guard: guard,
	}
}

// TestGuardFindings. An arc's acceptance bar names what the change is for and nothing
// named what it must not cost. ruflo's loop raised a harness score 40 → 55 with no
// capability change by adding files a presence-counting metric rewarded.
func TestGuardFindings(t *testing.T) {
	t.Parallel()
	got := evaluation.GuardFindings([]nfr.Spec{
		spec("latency", false), spec("density", true), spec("recall", true),
	})
	if len(got) != 2 {
		t.Fatalf("got %d findings, want one per guard: %+v", len(got), got)
	}
	// Routed through the existing NFR adjudicator, so a breached guard is a confirmed
	// finding and blocks by the existing rule -- no second gate, no new verdict.
	for _, f := range got {
		if f.Kind != adh.FindingNFR {
			t.Errorf("guard finding kind = %q, want nfr", f.Kind)
		}
		if f.Ref == "" {
			t.Error("a guard finding names no spec, so nothing can measure it")
		}
		if !f.Kind.Valid() || !f.Class.Valid() {
			t.Errorf("guard finding would be rejected by ParseFindings: %+v", f)
		}
	}
	if got[0].Ref != "density" || got[1].Ref != "recall" {
		t.Errorf("guards out of declared order: %v, %v", got[0].Ref, got[1].Ref)
	}
}

// TestAnUnmarkedSpecIsNotAGuard. False is an ordinary objective, so a spec nobody
// marked does not silently become a blocker -- the direction a mistake should fail in.
func TestAnUnmarkedSpecIsNotAGuard(t *testing.T) {
	t.Parallel()
	if got := evaluation.GuardFindings([]nfr.Spec{spec("latency", false)}); len(got) != 0 {
		t.Errorf("an unmarked spec produced %+v", got)
	}
	if got := evaluation.GuardFindings(nil); got == nil {
		t.Error("GuardFindings(nil) returned nil rather than empty")
	}
}
