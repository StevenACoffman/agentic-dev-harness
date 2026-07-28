package critic_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/contextstore"
	"github.com/StevenACoffman/agentic-dev-harness/internal/critic"
	"github.com/StevenACoffman/agentic-dev-harness/internal/proof"
)

func TestGroundRoutesAndAssembles(t *testing.T) {
	arc := &adh.Arc{
		ID:         "arc-0001",
		Stage:      adh.StageCritic,
		Resolution: adh.ResolutionChange,
		Labels:     []string{"auth"},
		Paths:      []string{"internal/authz"},
	}
	units := []contextstore.Unit{
		{ID: "u-auth", Kind: "runbook", Labels: []string{"auth"}},
		{ID: "u-other", Kind: "note", Labels: []string{"billing"}},
	}
	pkt := &proof.Packet{
		Arc:       "arc-0001",
		Artifacts: []proof.Artifact{{Path: "proof/oracle.txt", Digest: "abc"}},
	}

	g := critic.Ground(arc, units, pkt, "tests pass and CI is green")
	if len(g.Context) != 1 || g.Context[0].ID != "u-auth" {
		t.Errorf("routed context = %+v, want only u-auth", g.Context)
	}
	if g.AcceptanceBar != "tests pass and CI is green" {
		t.Errorf("acceptance bar = %q, want the passed-in contract text", g.AcceptanceBar)
	}
	if len(g.Proof) != 1 || g.Proof[0].Path != "proof/oracle.txt" {
		t.Errorf("proof artifacts = %+v, want the packet's artifact", g.Proof)
	}
	if len(g.Paths) != 1 || g.Paths[0] != "internal/authz" {
		t.Errorf("paths = %+v, want the arc's touched paths", g.Paths)
	}
}

func TestHasGrounding(t *testing.T) {
	tests := []struct {
		name string
		g    critic.Grounding
		want bool
	}{
		{"empty", critic.Grounding{AcceptanceBar: "some bar"}, false},
		{"proof only", critic.Grounding{Proof: []proof.Artifact{{Path: "p"}}}, true},
		{"context only", critic.Grounding{Context: []contextstore.Unit{{ID: "u"}}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.g.HasGrounding(); got != tt.want {
				t.Errorf("HasGrounding() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestForStageNonCriticIsNil(t *testing.T) {
	arc := &adh.Arc{ID: "arc-0001", Stage: adh.StageStrategy, Labels: []string{"auth"}}
	g, gap, err := critic.ForStage(arc, t.TempDir(), "bar")
	if err != nil {
		t.Fatalf("ForStage: %v", err)
	}
	if g != nil || gap {
		t.Errorf("ForStage at strategy = (%+v, %v), want (nil, false)", g, gap)
	}
}

func TestForStageEmptyStoreIsNotGap(t *testing.T) {
	// A declared arc against an absent/empty store is ungrounded, not a routing
	// gap: grounding is simply not configured yet (§19.1 smoothing).
	arc := &adh.Arc{ID: "arc-0001", Stage: adh.StageCritic, Labels: []string{"auth"}}
	_, gap, err := critic.ForStage(arc, filepath.Join(t.TempDir(), "no-store"), "bar")
	if err != nil {
		t.Fatalf("ForStage: %v", err)
	}
	if gap {
		t.Error("an empty/absent store should not be a routing gap")
	}
}

func TestForStageGapWhenPopulatedStoreDoesNotMatch(t *testing.T) {
	dir := t.TempDir()
	unit := `{"id":"u-billing","kind":"runbook","labels":["billing"]}`
	if err := os.WriteFile(filepath.Join(dir, "u-billing.json"), []byte(unit), 0o600); err != nil {
		t.Fatalf("write unit: %v", err)
	}
	// The store exists (has a unit) but nothing routes for the arc's labels — the
	// environment is set up yet did not teach this arc: a routing gap.
	arc := &adh.Arc{ID: "arc-0001", Stage: adh.StageCritic, Labels: []string{"auth"}}
	_, gap, err := critic.ForStage(arc, dir, "bar")
	if err != nil {
		t.Fatalf("ForStage: %v", err)
	}
	if !gap {
		t.Error("a declared arc against a populated but non-matching store should be a gap")
	}
}

func TestForStageUndeclaredIsNotGap(t *testing.T) {
	arc := &adh.Arc{ID: "arc-0001", Stage: adh.StageCritic, Resolution: adh.ResolutionChange}
	g, gap, err := critic.ForStage(arc, filepath.Join(t.TempDir(), "no-store"), "bar")
	if err != nil {
		t.Fatalf("ForStage: %v", err)
	}
	if gap {
		t.Error("an arc that declared no footprint is ungrounded, not a routing gap")
	}
	if g == nil {
		t.Fatal("critic stage should still return a grounding value")
	}
}

func TestForStageGroundedFromStore(t *testing.T) {
	dir := t.TempDir()
	unit := `{"id":"u-auth","kind":"runbook","labels":["auth"],"paths":["internal/authz"]}`
	if err := os.WriteFile(filepath.Join(dir, "u-auth.json"), []byte(unit), 0o600); err != nil {
		t.Fatalf("write unit: %v", err)
	}
	arc := &adh.Arc{ID: "arc-0001", Stage: adh.StageCritic, Labels: []string{"auth"}}
	g, gap, err := critic.ForStage(arc, dir, "bar")
	if err != nil {
		t.Fatalf("ForStage: %v", err)
	}
	if gap {
		t.Error("a matching context unit should ground the critic (no gap)")
	}
	if g == nil || len(g.Context) != 1 || g.Context[0].ID != "u-auth" {
		t.Errorf("grounding did not route the store unit: %+v", g)
	}
}

func TestLoadReadsProofManifest(t *testing.T) {
	dir := t.TempDir()
	manifest := filepath.Join(dir, "packet.json")
	pkt := `{"arc":"arc-0001","artifacts":[{"path":"proof/oracle.txt","sha256":"abc"}]}`
	if err := os.WriteFile(manifest, []byte(pkt), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	arc := &adh.Arc{ID: "arc-0001", Stage: adh.StageCritic, Proof: manifest}
	g, err := critic.Load(arc, filepath.Join(dir, "no-store"), "bar")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(g.Proof) != 1 || g.Proof[0].Path != "proof/oracle.txt" {
		t.Errorf("Load did not read the proof packet: %+v", g.Proof)
	}
}
