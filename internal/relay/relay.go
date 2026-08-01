// Package relay is the shared engine for the out-of-band model relay (SPEC §1-2):
// it emits a stage's prompt and parks a pending turn, then on a later invocation
// validates the operator's reply and advances the arc. The `step` and `run`
// command shells both drive it, so there is one relay implementation. It is
// pure with respect to I/O: it mutates the arc and reports an Outcome, while the
// shell owns the reads (the diff that grounds the critic, the footprint after an
// execution turn) and the writes (persisting the arc, printing the outcome).
package relay

import (
	"context"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/authority"
	"github.com/StevenACoffman/agentic-dev-harness/internal/contextstore"
	"github.com/StevenACoffman/agentic-dev-harness/internal/critic"
	"github.com/StevenACoffman/agentic-dev-harness/internal/model"
	"github.com/StevenACoffman/agentic-dev-harness/internal/stage"
)

const (
	// Awaiting means a prompt was emitted and the arc is parked for a reply.
	Awaiting Kind = iota
	// Gap means the critic is ungrounded — the environment did not teach it, so no
	// prompt was emitted (§19.1, exit 12); the shell reports the routing gap.
	Gap
	// Advanced means a reply was applied and the arc moved on a stage.
	Advanced
)

// Kind is what a relay step did.
type Kind int

// Outcome is the result of a relay step: its Kind and, for Awaiting, the emitted
// prompt.
type Outcome struct {
	Kind   Kind
	Prompt string
}

// Emit renders the arc's current stage prompt and parks it as a pending turn,
// grounding the critic from in (the shell supplies the diff). Re-emitting an
// already-open turn for the same stage is idempotent. It returns Gap without a
// prompt when the critic is ungrounded (§19.1). class/judgment gate the model
// (§5.1). It mutates arc.Pending; the shell persists the arc.
func Emit(
	arc *adh.Arc,
	storeDir string,
	in *critic.Inputs,
	renderer stage.Prompter,
	class authority.ModelClass,
	judgment authority.JudgmentRoles,
) (Outcome, error) {
	if arc.Pending != nil && arc.Pending.Stage == arc.Stage {
		return Outcome{Kind: Awaiting, Prompt: arc.Pending.Prompt}, nil
	}
	ground, gap, err := critic.ForStage(arc, storeDir, in)
	if err != nil {
		return Outcome{}, &adh.Error{Op: "relay.Emit", Err: err}
	}
	if gap {
		return Outcome{Kind: Gap}, nil
	}
	req, err := stage.Request(renderer, arc, ground, class, judgment)
	if err != nil {
		return Outcome{}, &adh.Error{Op: "relay.Emit", Err: err}
	}
	arc.Pending = &adh.Pending{Stage: arc.Stage, Prompt: req.Prompt}
	arc.Context = contextIDs(ground.Context) // record the loaded working set (§10.3)
	return Outcome{Kind: Awaiting, Prompt: req.Prompt}, nil
}

// contextIDs is the IDs of the routed units, the working set recorded on the arc.
func contextIDs(units []contextstore.Unit) []string {
	if len(units) == 0 {
		return nil
	}
	ids := make([]string, len(units))
	for i := range units {
		ids[i] = units[i].ID
	}
	return ids
}

// Resume validates replyText against the current stage's contract (critic.ParseReply)
// and advances the arc one stage, clearing the pending turn. A malformed reply is
// rejected before any state changes, so it never advances the arc (§19.2). A
// strategy reply's chosen resolution is applied (§12) and a critic reply's findings
// are recorded. It does not capture the execution footprint — that needs the
// worktree, so the shell does it after an execution turn. It mutates arc; the shell
// persists it.
func Resume(
	ctx context.Context,
	arc *adh.Arc,
	replyText string,
	renderer stage.Prompter,
	judgment authority.JudgmentRoles,
) (Outcome, error) {
	reply, err := critic.ParseReply(arc.Stage, replyText)
	if err != nil {
		return Outcome{}, &adh.Error{Op: "relay.Resume", Err: err}
	}
	wasCritic := arc.Stage == adh.StageCritic
	if arc.Stage == adh.StageStrategy && reply.Resolution != "" {
		arc.Resolution = reply.Resolution
	}
	// Record the validated reply text (a strategy reply's resolution line is
	// stripped), so history holds the plan, not the marker.
	if err := stage.Execute(
		ctx,
		model.Relay{Response: reply.Text},
		renderer,
		arc,
		judgment,
	); err != nil {
		return Outcome{}, &adh.Error{Op: "relay.Resume", Err: err}
	}
	if wasCritic {
		arc.Findings = reply.Findings
	}
	arc.Pending = nil
	return Outcome{Kind: Advanced}, nil
}
