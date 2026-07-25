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
// and satisfied by model.Mock and the real client.
type Client interface {
	Complete(ctx context.Context, req model.Request) (model.Response, error)
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
// arc's history, and advances the arc to the next stage — or marks it closed
// once ops completes. It mutates arc in place.
func Execute(ctx context.Context, client Client, arc *adh.Arc) error {
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
	if next, ok := adh.NextStage(arc.Stage); ok {
		arc.Stage = next
		return nil
	}
	arc.Status = adh.StatusClosed
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
