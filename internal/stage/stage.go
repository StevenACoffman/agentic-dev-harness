// Package stage sequences the five-stage loop (SPEC §1): it runs an arc's
// current stage through the model seam and advances it, and decides whether the
// relay may auto-launch the next stage under the autonomy level (SPEC §6). The
// critic runs cold — its prompt carries only the change under review, never the
// builder's transcript (SPEC-ADDITIONS §11, cold-context critic).
package stage

import (
	"context"
	"fmt"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/authority"
	"github.com/StevenACoffman/agentic-dev-harness/internal/model"
)

// Client is the model seam the stages need, declared here at the point of use
// and satisfied by model.Mock and the real client. ModelClass reports the
// capability tier the binding runs at, so Execute can enforce the model-gate.
type Client interface {
	Complete(ctx context.Context, req model.Request) (model.Response, error)
	ModelClass() authority.ModelClass
}

// AutoAdvances reports whether, after completing stage s at autonomy level lvl,
// the relay may launch the next stage without a human. Ops always stops (the
// ship is a human-gated irreversible action); below L2 every transition stops.
func AutoAdvances(s adh.Stage, lvl authority.Level) bool {
	if s == adh.StageOps {
		return false
	}
	if _, ok := adh.NextStage(s); !ok {
		return false
	}
	return lvl >= authority.L2
}

// Execute runs the arc's current stage through client, appends the output to the
// arc's history, and advances the arc to the next stage. It mutates arc in place.
// Ops is not a model step — it is the human-gated ship, performed by the close
// command — so Execute refuses it (EINVALID) and never auto-closes an arc. The
// judgment set (config-driven) is enforced against the model's class.
func Execute(
	ctx context.Context,
	client Client,
	arc *adh.Arc,
	judgment authority.JudgmentRoles,
) error {
	if arc.Stage == adh.StageOps {
		return &adh.Error{Code: adh.EINVALID, Message: "ops ships via close, not a model step"}
	}
	// Model-gate (SPEC §5.1): a judgment role must run on a reasoning-class model.
	if err := authority.ModelGate(arc.Stage, client.ModelClass(), judgment); err != nil {
		return fmt.Errorf("stage: %w", err)
	}
	resp, err := client.Complete(ctx, model.Request{Role: arc.Stage, Prompt: promptFor(arc)})
	if err != nil {
		return fmt.Errorf("stage: %w", err)
	}
	arc.History = append(arc.History, string(arc.Stage)+": "+resp.Text)
	// Strategy chooses the resolution (§12); the mock defaults an unset one to a
	// code change so a downstream close has a proof contract to check.
	if arc.Stage == adh.StageStrategy && arc.Resolution == "" {
		arc.Resolution = adh.ResolutionChange
	}
	next, _ := adh.NextStage(arc.Stage) // always present: ops is refused above
	arc.Stage = next
	return nil
}

// promptFor builds the stage's prompt. The critic gets a cold context — only the
// change and its proof, never the prior stages' transcript — so it cannot
// rationalize choices it never watched being made.
func promptFor(arc *adh.Arc) string {
	if arc.Stage == adh.StageCritic {
		return "review the change and its proof for arc " + arc.ID
	}
	return string(arc.Stage) + " the work for arc " + arc.ID
}
