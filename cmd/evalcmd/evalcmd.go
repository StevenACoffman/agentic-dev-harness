// Package evalcmd implements the "eval" CLI command: the deterministic Evaluation
// stage (SPEC-ADDITIONS §19.2). It adjudicates the cold critic's findings through
// internal/evaluation — a finding is never trusted on the critic's text. A
// confirmed finding (its artifact ran and failed) returns the arc to Execution
// and records a failure-registry entry (exit 5–8); an unconfirmed one becomes a
// §11 lesson candidate and advances the arc to ops.
package evalcmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/agentic-dev-harness/cmd/root"
	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/config"
	"github.com/StevenACoffman/agentic-dev-harness/internal/evaluation"
	"github.com/StevenACoffman/agentic-dev-harness/internal/state"
)

// Exit codes for a confirmed finding, by the artifact its kind names, matching the
// standalone gates: oracle=5, invariant=6, device=7, contract/proof=8.
const (
	exitOracle    = 5
	exitInvariant = 6
	exitDevice    = 7
	exitContract  = 8
)

// Config holds the configuration for the eval command.
type Config struct {
	*root.Config
	adjudicator evaluation.Adjudicator
	Flags       *ff.FlagSet
	Command     *ff.Command
}

// New creates and registers the eval command with the given parent config.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.adjudicator = evaluation.RepoAdjudicator{}
	cfg.Flags = ff.NewFlagSet("eval").SetParent(parent.Flags)
	cfg.Command = &ff.Command{
		Name:      "eval",
		Usage:     "agentic-dev-harness eval <arc-id>",
		ShortHelp: "adjudicate the critic's findings and dispose of the arc",
		LongHelp: "Run the artifact each critic finding names (SPEC-ADDITIONS §19.2). " +
			"A confirmed finding returns the arc to execution (exit 5-8); an " +
			"unconfirmed one becomes a lesson candidate and the arc advances to ops.",
		Flags: cfg.Flags,
		Exec:  cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

func (cfg *Config) exec(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("eval: requires an arc id")
	}
	store := state.Default()
	arc, err := store.Get(args[0])
	if err != nil {
		return fmt.Errorf("eval: %w", err)
	}
	if arc.Status != adh.StatusOpen {
		return fmt.Errorf("eval: arc %s is not open (status %s)", arc.ID, arc.Status)
	}
	if arc.Stage != adh.StageEvaluation {
		return fmt.Errorf("eval: arc %s is at %s, not evaluation", arc.ID, arc.Stage)
	}
	conf, err := config.Load(cfg.ConfigGetenv())
	if err != nil {
		return fmt.Errorf("eval: %w", err)
	}

	verdict, err := evaluation.Adjudicate(ctx, cfg.adjudicator, arc.Findings)
	if err != nil {
		return fmt.Errorf("eval: %w", err)
	}
	recordLessons := conf.CriticUnconfirmed() == config.UnconfirmedLesson
	if err := evaluation.Apply(&arc, &verdict, recordLessons); err != nil {
		return fmt.Errorf("eval: %w", err)
	}
	if err := store.Save(&arc); err != nil {
		return fmt.Errorf("eval: %w", err)
	}
	if verdict.ReturnsToExecution() {
		_, _ = fmt.Fprintf(cfg.Stdout,
			"eval: %d finding(s) confirmed; arc %s returned to execution\n",
			len(verdict.Confirmed), arc.ID)
		return root.ExitError(exitFor(verdict.BlockingKind()))
	}
	_, _ = fmt.Fprintf(cfg.Stdout,
		"eval: no findings confirmed; arc %s advanced to ops (%d lesson candidate(s))\n",
		arc.ID, len(verdict.Unconfirmed))
	return nil
}

// exitFor maps a confirmed finding's kind to its Evaluation gate exit code. An
// unknown or NFR kind (no dedicated gate yet) surfaces as the invariant code.
func exitFor(kind adh.FindingKind) int {
	switch kind {
	case adh.FindingOracle:
		return exitOracle
	case adh.FindingInvariant:
		return exitInvariant
	case adh.FindingDevice:
		return exitDevice
	case adh.FindingContract:
		return exitContract
	case adh.FindingNFR:
		return exitInvariant
	default:
		return exitInvariant
	}
}
