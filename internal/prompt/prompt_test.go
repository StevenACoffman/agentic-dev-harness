package prompt_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/contextstore"
	"github.com/StevenACoffman/agentic-dev-harness/internal/critic"
	"github.com/StevenACoffman/agentic-dev-harness/internal/prompt"
	"github.com/StevenACoffman/agentic-dev-harness/internal/proof"
)

func TestRenderStageAndID(t *testing.T) {
	r, err := prompt.New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	arc := &adh.Arc{ID: "arc-0001", Title: "widen the gate", Stage: adh.StageStrategy}
	out, err := r.Render(arc, nil)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(out, "arc-0001") {
		t.Errorf("strategy prompt missing arc id:\n%s", out)
	}
	if !strings.Contains(out, "widen the gate") {
		t.Errorf("strategy prompt missing title:\n%s", out)
	}
}

// TestRenderCriticIsCold is the load-bearing test: the critic's prompt must never
// carry the builder's transcript, even though the arc holds one.
func TestRenderCriticIsCold(t *testing.T) {
	r, err := prompt.New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	const leak = "BUILDER-TRANSCRIPT-SECRET"
	arc := &adh.Arc{
		ID:         "arc-0002",
		Stage:      adh.StageCritic,
		Resolution: adh.ResolutionChange,
		History:    []string{"execution: " + leak},
	}
	out, err := r.Render(arc, nil)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if strings.Contains(out, leak) {
		t.Errorf("critic prompt leaked the builder transcript:\n%s", out)
	}
	if !strings.Contains(out, "arc-0002") {
		t.Errorf("critic prompt missing arc id:\n%s", out)
	}
}

// TestRenderCriticGrounded shows the §19.1 working set in the prompt — acceptance
// bar, touched paths, proof artifacts, routed context units — while the builder's
// transcript stays out.
func TestRenderCriticGrounded(t *testing.T) {
	r, err := prompt.New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	const leak = "BUILDER-TRANSCRIPT-SECRET"
	arc := &adh.Arc{
		ID:         "arc-0006",
		Stage:      adh.StageCritic,
		Resolution: adh.ResolutionChange,
		History:    []string{"execution: " + leak},
	}
	ground := &critic.Grounding{
		Paths:         []string{"internal/authz/policy.go"},
		Diff:          "--- a/internal/authz/policy.go\n+++ b/internal/authz/policy.go\n+allow := true\n",
		AcceptanceBar: adh.ResolutionChange.ProofKind(),
		Proof:         []proof.Artifact{{Path: "proof/oracle.txt"}},
		Context:       []contextstore.Unit{{ID: "u-auth", Kind: "runbook"}},
	}
	out, err := r.Render(arc, ground)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	for _, want := range []string{"internal/authz/policy.go", "proof/oracle.txt", "u-auth", "+allow := true"} {
		if !strings.Contains(out, want) {
			t.Errorf("grounded critic prompt missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, leak) {
		t.Errorf("grounded critic prompt leaked the builder transcript:\n%s", out)
	}
}

func TestRenderExecutionCarriesHistory(t *testing.T) {
	r, err := prompt.New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	arc := &adh.Arc{
		ID:      "arc-0003",
		Stage:   adh.StageExecution,
		History: []string{"strategy: chose change"},
	}
	out, err := r.Render(arc, nil)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(out, "strategy: chose change") {
		t.Errorf("execution prompt dropped prior history:\n%s", out)
	}
}

func TestRenderRejectsStagesWithoutTemplate(t *testing.T) {
	r, err := prompt.New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	for _, stage := range []adh.Stage{adh.StageOps, adh.Stage("nonsense")} {
		arc := &adh.Arc{ID: "arc-0004", Stage: stage}
		if _, renderErr := r.Render(arc, nil); adh.ErrorCode(renderErr) != adh.EINVALID {
			t.Errorf("Render(%s) = %v, want EINVALID", stage, renderErr)
		}
	}
}

func TestDefaultUsesEmbeddedWhenNoOverride(t *testing.T) {
	t.Chdir(t.TempDir()) // no .adh/prompts here, so the embedded defaults stand
	r, err := prompt.Default()
	if err != nil {
		t.Fatalf("Default: %v", err)
	}
	out, err := r.Render(&adh.Arc{ID: "arc-0009", Stage: adh.StageStrategy}, nil)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(out, "arc-0009") {
		t.Errorf("default renderer dropped arc id:\n%s", out)
	}
}

func TestOverrideWins(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(dir, "strategy.tmpl"),
		[]byte("OVERRIDE {{.ID}}\n"),
		0o600,
	); err != nil {
		t.Fatalf("write override: %v", err)
	}
	r, err := prompt.New(os.DirFS(dir))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	arc := &adh.Arc{ID: "arc-0005", Stage: adh.StageStrategy}
	out, err := r.Render(arc, nil)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(out, "OVERRIDE arc-0005") {
		t.Errorf("override not applied:\n%s", out)
	}
}
