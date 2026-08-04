package relay_test

import (
	"context"
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
