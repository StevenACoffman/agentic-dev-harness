// Package stagecmd implements the per-stage direct commands (SPEC §2.2):
// strategy, execute, and critic each run one stage on an arc already at that
// stage, through the mock model, for debugging or manual re-runs — normal
// operation uses `adh run`/`adh step`. Ops is the human-gated ship, not a model
// step, so `adh ops` reports the gate rather than advancing.
package stagecmd

import (
	"context"
	"fmt"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/agentic-dev-harness/cmd/root"
	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/config"
	"github.com/StevenACoffman/agentic-dev-harness/internal/model"
	"github.com/StevenACoffman/agentic-dev-harness/internal/prompt"
	"github.com/StevenACoffman/agentic-dev-harness/internal/stage"
	"github.com/StevenACoffman/agentic-dev-harness/internal/state"
)

// stageCmd runs a single named stage on an arc positioned at it.
type stageCmd struct {
	*root.Config
	name    string
	stage   adh.Stage
	Flags   *ff.FlagSet
	Command *ff.Command
}

// opsCmd reports the ops ship gate for an arc.
type opsCmd struct {
	*root.Config
	Flags   *ff.FlagSet
	Command *ff.Command
}

// New registers the per-stage manual commands with the given parent config.
func New(parent *root.Config) {
	newStage(parent, "strategy", adh.StageStrategy, "plan the change (manual single stage)")
	newStage(parent, "execute", adh.StageExecution, "build the change (manual single stage)")
	newStage(
		parent,
		"critic",
		adh.StageCritic,
		"review the change in cold context (manual single stage)",
	)
	newOps(parent)
}

func newStage(parent *root.Config, name string, stg adh.Stage, help string) {
	cmd := &stageCmd{Config: parent, name: name, stage: stg}
	cmd.Flags = ff.NewFlagSet(name).SetParent(parent.Flags)
	cmd.Command = &ff.Command{
		Name:      name,
		Usage:     "agentic-dev-harness " + name + " <arc-id>",
		ShortHelp: help,
		LongHelp: "Run the " + string(stg) + " stage on an arc already at it, through " +
			"the mock model (SPEC §2.2). For debugging or manual re-runs; normal " +
			"operation uses `adh run`.",
		Flags: cmd.Flags,
		Exec:  cmd.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cmd.Command)
}

func newOps(parent *root.Config) {
	cmd := &opsCmd{Config: parent}
	cmd.Flags = ff.NewFlagSet("ops").SetParent(parent.Flags)
	cmd.Command = &ff.Command{
		Name:      "ops",
		Usage:     "agentic-dev-harness ops <arc-id>",
		ShortHelp: "report the ship gate for an arc at ops",
		LongHelp: "Ops is the human-gated ship (SPEC §2.2, §5.2). This reports what an " +
			"arc at ops needs; it does not ship — a human approves, then `close` ships.",
		Flags: cmd.Flags,
		Exec:  cmd.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cmd.Command)
}

func (c *stageCmd) exec(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return root.MissingOperandError{Scope: c.name, Kind: root.OperandArc}
	}
	if c.DryRun {
		return root.DryRunUnsupportedError(c.name)
	}
	store := state.Default()
	arc, err := store.Get(args[0])
	if err != nil {
		return fmt.Errorf("%s: %w", c.name, err)
	}
	if arc.Status != adh.StatusOpen {
		return &adh.Error{
			Code:    adh.ECONFLICT,
			Message: fmt.Sprintf("%s: arc %s is not open (status %s)", c.name, arc.ID, arc.Status),
		}
	}
	if arc.Stage != c.stage {
		return &adh.Error{
			Code:    adh.ECONFLICT,
			Message: fmt.Sprintf("%s: arc %s is at %s, not %s", c.name, arc.ID, arc.Stage, c.stage),
		}
	}
	conf, err := config.Load(c.ConfigGetenv())
	if err != nil {
		return fmt.Errorf("%s: %w", c.name, err)
	}
	renderer, err := prompt.Default()
	if err != nil {
		return fmt.Errorf("%s: %w", c.name, err)
	}
	if err := stage.Execute(ctx, model.Mock{}, renderer, &arc, conf.JudgmentRoles()); err != nil {
		return fmt.Errorf("%s: %w", c.name, err)
	}
	if err := store.Save(&arc); err != nil {
		return fmt.Errorf("%s: %w", c.name, err)
	}
	if c.JSONL {
		// `arc`/`stage`/`status` are the keys `step` already emits for the same answer
		// — where the arc ended up — so a caller reads a manual single stage and a
		// driven one the same way.
		//
		// All three stage commands share this function, so all three were printing
		// prose under --jsonl. Only `strategy` was observably wrong: a fresh arc sits
		// at strategy, so `execute` and `critic` refuse for their stage and print
		// nothing, which satisfies the contract without exercising it.
		if err := c.EmitOK(map[string]any{
			"arc": arc.ID, "stage": string(arc.Stage), "status": string(arc.Status),
		}); err != nil {
			return fmt.Errorf("%s: %w", c.name, err)
		}
		return nil
	}
	_, _ = fmt.Fprintf(c.Stdout, "%s now at %s (%s)\n", arc.ID, arc.Stage, arc.Status)
	return nil
}

func (c *opsCmd) exec(_ context.Context, args []string) error {
	if len(args) == 0 {
		return root.MissingOperandError{Scope: "ops", Kind: root.OperandArc}
	}
	arc, err := state.Default().Get(args[0])
	if err != nil {
		return fmt.Errorf("ops: %w", err)
	}
	if c.JSONL {
		// Both prose branches say the same two things -- which stage the arc is at, and
		// whether that is the ship gate -- so one payload carries both and the caller
		// branches on `at_gate` instead of matching a sentence. `ops` reports rather
		// than advances, so reaching the gate is an outcome, not an error.
		if err := c.EmitOK(map[string]any{
			"arc": arc.ID, "stage": string(arc.Stage), "at_gate": arc.Stage == adh.StageOps,
		}); err != nil {
			return fmt.Errorf("ops: %w", err)
		}
		return nil
	}
	if arc.Stage != adh.StageOps {
		_, _ = fmt.Fprintf(
			c.Stdout,
			"%s is at %s, not yet at the ops ship gate\n",
			arc.ID,
			arc.Stage,
		)
		return nil
	}
	_, _ = fmt.Fprintf(
		c.Stdout,
		"%s is at the ops ship gate: a human must approve it (`adh approve %s`) then `adh close %s`\n",
		arc.ID,
		arc.ID,
		arc.ID,
	)
	return nil
}
