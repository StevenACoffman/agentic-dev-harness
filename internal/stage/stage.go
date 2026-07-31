// Package stage sequences the five-stage loop (SPEC §1): it runs an arc's
// current stage through the model seam and advances it, and decides whether the
// relay may auto-launch the next stage under the autonomy level (SPEC §6). A
// stage's work splits into two pure halves — Request (gate the model class and
// render the prompt) and Apply (record the response and advance) — so a relay
// can emit the prompt on one process invocation and apply the reply on the next.
// Execute composes them for the synchronous Mock/API path. The critic runs cold:
// the injected Prompter withholds the builder's transcript (SPEC §1).
package stage

import (
	"context"
	"fmt"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/authority"
	"github.com/StevenACoffman/agentic-dev-harness/internal/critic"
	"github.com/StevenACoffman/agentic-dev-harness/internal/model"
)

// Client is the model seam the stages need, declared here at the point of use
// and satisfied by model.Mock, model.Relay, and the real client. ModelClass
// reports the capability tier the binding runs at, so the gate can be enforced.
type Client interface {
	Complete(ctx context.Context, req model.Request) (model.Response, error)
	ModelClass() authority.ModelClass
}

// Prompter renders the prompt for an arc's current stage. It is declared here at
// the point of use and satisfied by *prompt.Renderer; the critic's prompt is
// cold by construction, since the renderer withholds the builder's transcript.
// ground is the critic's repository-owned working set (§19.1); it is nil for
// every other stage and for an ungrounded critic.
type Prompter interface {
	Render(arc *adh.Arc, ground *critic.Grounding) (string, error)
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

// Request builds the model request for the arc's current stage. It enforces the
// model-gate (SPEC §5.1) against class and renders the prompt via prompter,
// passing ground (the critic's working set, §19.1; nil for other stages). It is
// pure: no I/O, no mutation of arc. Ops has no model step (it ships via close),
// so Request refuses it with EINVALID. The gate is checked before the prompt is
// rendered, so a gated stage never produces a prompt.
func Request(
	prompter Prompter,
	arc *adh.Arc,
	ground *critic.Grounding,
	class authority.ModelClass,
	judgment authority.JudgmentRoles,
) (model.Request, error) {
	if arc.Stage == adh.StageOps {
		return model.Request{}, &adh.Error{
			Code:    adh.EINVALID,
			Message: "ops ships via close, not a model step",
		}
	}
	if err := authority.ModelGate(arc.Stage, class, judgment); err != nil {
		return model.Request{}, fmt.Errorf("stage: %w", err)
	}
	prompt, err := prompter.Render(arc, ground)
	if err != nil {
		return model.Request{}, fmt.Errorf("stage: %w", err)
	}
	return model.Request{Role: arc.Stage, Prompt: prompt}, nil
}

// Apply records resp in the arc's history and advances the arc to the next stage
// (SPEC §1). It mutates arc in place. Strategy chooses the resolution (§12); an
// unset one defaults to a code change so a downstream close has a proof contract
// to check. Apply's precondition is that the arc is not at ops — Request refuses
// ops, so a pending turn never parks there — and the ok guard makes ops a safe
// no-op advance rather than blanking the stage.
func Apply(arc *adh.Arc, resp model.Response) {
	arc.History = append(arc.History, string(arc.Stage)+": "+resp.Text)
	if arc.Stage == adh.StageStrategy && arc.Resolution == "" {
		arc.Resolution = adh.ResolutionChange
	}
	next, ok := adh.NextStage(arc.Stage)
	if !ok {
		return
	}
	arc.Stage = next
}

// Execute runs the arc's current stage through client and advances it, composing
// the two pure halves: Request gates and renders, client.Complete answers, Apply
// records and advances. It mutates arc in place. Ops is refused by Request. The
// critic's grounding is nil here: Execute drives the synchronous Mock/API path
// and the relayed resume, where the rendered critic prompt is not consumed by a
// reasoner — the relay's grounded emit is handled by the step command via Request.
func Execute(
	ctx context.Context,
	client Client,
	prompter Prompter,
	arc *adh.Arc,
	judgment authority.JudgmentRoles,
) error {
	req, err := Request(prompter, arc, nil, client.ModelClass(), judgment)
	if err != nil {
		return err
	}
	resp, err := client.Complete(ctx, req)
	if err != nil {
		return fmt.Errorf("stage: %w", err)
	}
	Apply(arc, resp)
	return nil
}
