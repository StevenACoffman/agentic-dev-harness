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
	"github.com/StevenACoffman/agentic-dev-harness/internal/toolreg"
	"github.com/StevenACoffman/skillet/finding"
	"github.com/StevenACoffman/skillet/proof"
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

// TestRenderStrategyShowsContextAndTools confirms non-critic stages load their
// routed working set (§10, §13) into the prompt.
func TestRenderStrategyShowsContextAndTools(t *testing.T) {
	r, err := prompt.New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	arc := &adh.Arc{ID: "arc-0001", Stage: adh.StageStrategy, Labels: []string{"auth"}}
	ground := &critic.Grounding{
		Context: []contextstore.Unit{{ID: "u-auth", Kind: "runbook"}},
		Tools:   []toolreg.Tool{{ID: "oracle-diff", Verifies: "equivalence", Run: "make oracle"}},
	}
	out, err := r.Render(arc, ground)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	for _, want := range []string{"u-auth", "oracle-diff", "equivalence"} {
		if !strings.Contains(out, want) {
			t.Errorf("strategy prompt missing %q:\n%s", want, out)
		}
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

// TestRenderCriticNoisyHint: when the grounding names over-flagged (noisy) finding
// kinds, the critic prompt renders the higher-bar hint and the kinds.
func TestRenderCriticNoisyHint(t *testing.T) {
	r, err := prompt.New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	arc := &adh.Arc{ID: "arc-0007", Stage: adh.StageCritic, Resolution: adh.ResolutionChange}
	ground := &critic.Grounding{
		AcceptanceBar: adh.ResolutionChange.ProofKind(),
		Noisy:         []string{"oracle", "nfr"},
	}
	out, err := r.Render(arc, ground)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(out, "higher bar") {
		t.Errorf("noisy critic prompt missing the higher-bar hint:\n%s", out)
	}
	for _, kind := range []string{"oracle", "nfr"} {
		if !strings.Contains(out, kind) {
			t.Errorf("noisy critic prompt missing kind %q:\n%s", kind, out)
		}
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

// TestTheCriticViewWithholdsTheTranscript is the guarantee itself, asserted at the
// level that matters: the rendered prompt. A critic that could read the builder's
// history is not cold, and independence is the whole basis for its findings counting.
func TestTheCriticViewWithholdsTheTranscript(t *testing.T) {
	t.Parallel()
	const secret = "I already tried the obvious fix and it did not work"
	r, err := prompt.New()
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	arc := &adh.Arc{
		ID: "a1", Title: "t", Stage: adh.StageCritic,
		Resolution: adh.ResolutionChange, History: []string{secret},
	}
	got, err := r.Render(arc, &critic.Grounding{Diff: "d"})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if strings.Contains(got, secret) {
		t.Fatalf("the critic prompt carries the builder's transcript:\n%s", got)
	}
}

// wantRenderError asserts a render outcome: no error when says is empty, otherwise an
// error mentioning it.
func wantRenderError(t *testing.T, err error, says string) {
	t.Helper()
	if says == "" {
		if err != nil {
			t.Fatalf("render: %v", err)
		}
		return
	}
	if err == nil {
		t.Fatal("the render succeeded where it should have refused")
	}
	if !strings.Contains(err.Error(), says) {
		t.Errorf("the error %q omits %q", err, says)
	}
}

// TestDenyIsEnforced. The list used to be read by nothing, which is worse than an
// ordinary unused field: it reads as the mechanism excluding the transcript, and as
// configurable, so somebody hardening the critic would edit it and believe they had
// succeeded.
//
// This checks the assertion, not the guarantee. The guarantee is that the view is
// never populated for the critic; a check running afterwards can only notice.
func TestDenyIsEnforced(t *testing.T) {
	t.Parallel()
	r, err := prompt.New()
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	cases := map[string]struct {
		stage  adh.Stage
		denied []adh.CriticInput
		says   string
	}{
		"the critic denying the transcript": {
			adh.StageCritic, []adh.CriticInput{adh.InputTranscript}, "",
		},
		// A stage that legitimately sees its history, with a deny list that says it
		// must not. The renderer reports rather than silently stripping: adh does not
		// filter, and a config asking it to is a config error.
		"a stage that carries what is denied": {
			adh.StageExecution, []adh.CriticInput{adh.InputTranscript}, "which is denied",
		},
		// An input the renderer cannot check must fail loudly rather than pass. This
		// is the default branch, and it is the whole reason the mapping is a switch
		// rather than a reflection walk over field names.
		"denying an input the renderer cannot enforce": {
			adh.StageCritic, []adh.CriticInput{adh.InputContext}, "cannot enforce",
		},
		"no deny list": {adh.StageCritic, nil, ""},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			arc := &adh.Arc{
				ID: "a1", Title: "t", Stage: tc.stage,
				Resolution: adh.ResolutionChange, History: []string{"prior turn"},
			}
			_, rErr := r.Render(arc, &critic.Grounding{Diff: "d", Denied: tc.denied})
			wantRenderError(t, rErr, tc.says)
		})
	}
}

// TestThePriorRunsGapsReachTheCritic. §19.1 withholds exactly one input, the builder's
// transcript, and a previous critic's coverage is neither the builder's reasoning nor a
// conclusion — it is a record of angles already taken, so feeding it forward steers a
// fresh critic toward unexamined ground rather than contaminating it.
func TestThePriorRunsGapsReachTheCritic(t *testing.T) {
	t.Parallel()
	r, err := prompt.New()
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	arc := &adh.Arc{
		ID: "a1", Title: "t", Stage: adh.StageCritic, Resolution: adh.ResolutionChange,
	}
	got, err := r.Render(arc, &critic.Grounding{
		Diff: "d",
		PriorGaps: []finding.Unexamined{
			{Aspect: "concurrency", Reason: "no repro harness"},
		},
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	for _, want := range []string{"concurrency", "no repro harness"} {
		if !strings.Contains(got, want) {
			t.Errorf("the prompt omits %q:\n%s", want, got)
		}
	}
	// The wording has to say these are not the scope. Offering last run's gaps
	// invites a critic to treat them as a checklist and examine only those.
	if !strings.Contains(got, "not the scope of this one") {
		t.Errorf("the prompt does not warn against reading the gaps as a checklist:\n%s", got)
	}
}

// TestTheCriticIsToldItMayDeclareAGap. Without this the parser is a reader with no
// writer: nothing would ever populate `unexamined`, and the field would be the same
// dead knob Critic.Deny was.
func TestTheCriticIsToldItMayDeclareAGap(t *testing.T) {
	t.Parallel()
	r, err := prompt.New()
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	arc := &adh.Arc{
		ID: "a1", Title: "t", Stage: adh.StageCritic, Resolution: adh.ResolutionChange,
	}
	got, err := r.Render(arc, &critic.Grounding{Diff: "d"})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	for _, want := range []string{`"unexamined"`, "aspect", "reason", "never blocks"} {
		if !strings.Contains(got, want) {
			t.Errorf("the critic prompt omits %q:\n%s", want, got)
		}
	}
}
