package critic_test

import (
	"path/filepath"
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/critic"
)

func TestFindingKinds(t *testing.T) {
	got := critic.FindingKinds([]adh.Finding{
		{Kind: adh.FindingNFR}, {Kind: adh.FindingNFR}, {Kind: adh.FindingOracle}, {Kind: ""},
	})
	if len(got) != 2 || got[0] != "nfr" || got[1] != "oracle" {
		t.Errorf("FindingKinds = %v, want [nfr oracle] (distinct, sorted)", got)
	}
}

func TestUnderCovered(t *testing.T) {
	all := adh.FindingKinds()
	// Empty history: every kind is under-covered.
	if got := critic.UnderCovered(nil, all); len(got) != len(all) {
		t.Errorf("UnderCovered(empty) = %v, want all %d kinds", got, len(all))
	}
	// Oracle surfaced twice, invariant once; the rest never → the never-seen kinds
	// are the least-covered (tied at 0).
	entries := []critic.CoverageEntry{
		{Arc: "a1", Kinds: []string{"oracle", "invariant"}},
		{Arc: "a2", Kinds: []string{"oracle"}},
	}
	under := critic.UnderCovered(entries, all)
	for _, k := range under {
		if k == adh.FindingOracle || k == adh.FindingInvariant {
			t.Errorf("UnderCovered included a surfaced kind %q: %v", k, under)
		}
	}
	if len(under) != 3 { // device, nfr, contract never surfaced
		t.Errorf("UnderCovered = %v, want the 3 never-surfaced kinds", under)
	}
}

func TestAppendLoadCoverage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "coverage.jsonl")
	e1 := critic.CoverageEntry{Arc: "a1", Kinds: []string{"nfr"}}
	if err := critic.AppendCoverage(path, &e1); err != nil {
		t.Fatalf("AppendCoverage: %v", err)
	}
	// An entry with no kinds is a no-op (does not pollute the log).
	empty := critic.CoverageEntry{Arc: "a2"}
	if err := critic.AppendCoverage(path, &empty); err != nil {
		t.Fatalf("AppendCoverage(empty): %v", err)
	}
	got, err := critic.LoadCoverage(path)
	if err != nil {
		t.Fatalf("LoadCoverage: %v", err)
	}
	if len(got) != 1 || got[0].Arc != "a1" {
		t.Fatalf("LoadCoverage = %+v, want one entry a1", got)
	}
}
