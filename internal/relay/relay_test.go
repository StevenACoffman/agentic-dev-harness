package relay_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/authority"
	"github.com/StevenACoffman/agentic-dev-harness/internal/critic"
	"github.com/StevenACoffman/agentic-dev-harness/internal/relay"
)

// fakePrompter renders a deterministic prompt so the engine's orchestration is
// exercised without the real templates.
type fakePrompter struct{}

func (fakePrompter) Render(arc *adh.Arc, _ *critic.Grounding) (string, error) {
	return "prompt for " + string(arc.Stage), nil
}

// noStore is a context-store dir that does not exist, so routing is unconfigured
// (not a gap) and a stage emits normally.
func noStore(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "no-store")
}

func TestEmitParksPending(t *testing.T) {
	arc := adh.Arc{ID: "arc-0001", Stage: adh.StageStrategy, Status: adh.StatusOpen}
	out, err := relay.Emit(&arc, noStore(t), &critic.Inputs{}, fakePrompter{},
		authority.ClassReasoning, nil)
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}
	if out.Kind != relay.Awaiting || out.Prompt == "" {
		t.Errorf("emit = %+v, want an awaiting prompt", out)
	}
	if arc.Pending == nil || arc.Pending.Stage != adh.StageStrategy {
		t.Errorf("pending not parked for the stage: %+v", arc.Pending)
	}
}

func TestEmitReEmitIsIdempotent(t *testing.T) {
	arc := adh.Arc{
		ID:      "arc-0001",
		Stage:   adh.StageStrategy,
		Status:  adh.StatusOpen,
		Pending: &adh.Pending{Stage: adh.StageStrategy, Prompt: "already parked"},
	}
	out, err := relay.Emit(&arc, noStore(t), &critic.Inputs{}, fakePrompter{},
		authority.ClassReasoning, nil)
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}
	if out.Prompt != "already parked" {
		t.Errorf("re-emit = %q, want the parked prompt reprinted", out.Prompt)
	}
}

func TestResumeAdvancesAndClearsPending(t *testing.T) {
	arc := adh.Arc{
		ID:      "arc-0001",
		Stage:   adh.StageStrategy,
		Status:  adh.StatusOpen,
		Pending: &adh.Pending{Stage: adh.StageStrategy, Prompt: "p"},
	}
	out, err := relay.Resume(context.Background(), &arc, "widen it", fakePrompter{}, nil)
	if err != nil {
		t.Fatalf("Resume: %v", err)
	}
	if out.Kind != relay.Advanced || arc.Stage != adh.StageExecution || arc.Pending != nil {
		t.Errorf(
			"resume = %+v, arc stage %s pending %v; want advanced to execution, pending cleared",
			out,
			arc.Stage,
			arc.Pending,
		)
	}
}

func TestResumeStrategyChoosesResolution(t *testing.T) {
	arc := adh.Arc{ID: "arc-0001", Stage: adh.StageStrategy, Status: adh.StatusOpen}
	if _, err := relay.Resume(context.Background(), &arc,
		"resolution: investigation\ninspect only", fakePrompter{}, nil); err != nil {
		t.Fatalf("Resume: %v", err)
	}
	if arc.Resolution != adh.ResolutionInvestigation {
		t.Errorf("resolution = %q, want investigation", arc.Resolution)
	}
}

func TestResumeRejectsMalformedCriticReplyWithoutAdvancing(t *testing.T) {
	arc := adh.Arc{ID: "arc-0001", Stage: adh.StageCritic, Status: adh.StatusOpen}
	if _, err := relay.Resume(context.Background(), &arc, "not findings json",
		fakePrompter{}, nil); adh.ErrorCode(err) != adh.EINVALID {
		t.Fatalf("Resume of a malformed critic reply = %v, want EINVALID", err)
	}
	if arc.Stage != adh.StageCritic {
		t.Errorf("a rejected reply advanced the arc to %s", arc.Stage)
	}
}

// TestAnUngroundedCriticIsReported. critic.HasGrounding computed this and, outside the
// Gap branch, its answer was dropped — so a review grounded in six routed units and one
// grounded in nothing reached Ops as the same artifact. Third instance in this repo of
// a distinction the code computes and discards.
//
// It is deliberately not a Gap. A Gap is an arc that declared a footprint against a
// store with units and still routed nothing, which means routing failed and blocks with
// exit 12. These are arcs with no footprint, or repositories with no store at all, and
// refusing them would make adh unusable before a context store exists.
// seedStore writes one context unit carrying the given labels, so an arc declaring
// them routes something.
func seedStore(t *testing.T, labels []string) string {
	t.Helper()
	dir := t.TempDir()
	unit := map[string]any{
		"id": "u1", "kind": "requirement", "labels": labels,
		"body": "the cache is cleared on restart",
	}
	b, err := json.Marshal(unit)
	if err != nil {
		t.Fatalf("marshal unit: %v", err)
	}
	if wErr := os.WriteFile(filepath.Join(dir, "u1.json"), b, 0o600); wErr != nil {
		t.Fatalf("write unit: %v", wErr)
	}
	return dir
}

func TestAnUngroundedCriticIsReported(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		stage      adh.Stage
		labels     []string
		seedUnit   bool
		wantKind   relay.Kind
		ungrounded bool
	}{
		"a critic with no footprint and no store": {
			adh.StageCritic, nil, false, relay.Awaiting, true,
		},
		// A proof packet is grounding on its own: HasGrounding is proof OR context.
		// A routed context unit is grounding, so the note must not fire.
		"a critic the store taught": {
			adh.StageCritic, []string{"cache"}, true, relay.Awaiting, false,
		},
		// Only the critic runs cold, so only the critic can be ungrounded in the
		// sense §19.1 means.
		"another stage with nothing routed": {
			adh.StageExecution, nil, false, relay.Awaiting, false,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			arc := &adh.Arc{
				ID: "a1", Title: "t", Stage: tc.stage, Status: adh.StatusOpen,
				Resolution: adh.ResolutionChange, Labels: tc.labels,
			}
			dir := noStore(t)
			if tc.seedUnit {
				dir = seedStore(t, tc.labels)
			}
			in := critic.Inputs{AcceptanceBar: "bar", Diff: "d"}
			out, err := relay.Emit(arc, dir, &in, fakePrompter{},
				authority.ClassReasoning, nil)
			if err != nil {
				t.Fatalf("emit: %v", err)
			}
			if out.Kind != tc.wantKind {
				t.Fatalf("kind = %v, want %v", out.Kind, tc.wantKind)
			}
			if out.Ungrounded != tc.ungrounded {
				t.Errorf("ungrounded = %v, want %v", out.Ungrounded, tc.ungrounded)
			}
		})
	}
}
