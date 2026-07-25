// Package approve implements the "approve" CLI command: satisfy a pending human
// safety gate on an arc (SPEC §5.2). Approval requires an explicit phrase — the
// arc's own ID — typed by a human via --phrase. --yes and --dry-run never
// satisfy a gate, so the agent has no code route to self-grant.
package approve

import (
	"context"
	"errors"
	"fmt"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/agentic-dev-harness/cmd/root"
	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/authority"
	"github.com/StevenACoffman/agentic-dev-harness/internal/state"
)

// Config holds the configuration for the approve command.
type Config struct {
	*root.Config
	Phrase  string
	Yes     bool
	DryRun  bool
	Flags   *ff.FlagSet
	Command *ff.Command
}

// New creates and registers the approve command with the given parent config.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("approve").SetParent(parent.Flags)
	cfg.Flags.StringVar(
		&cfg.Phrase,
		'p',
		"phrase",
		"",
		"the approval phrase (the arc ID), typed by a human",
	)
	cfg.Flags.BoolVar(
		&cfg.Yes,
		0,
		"yes",
		"pre-answer non-gate prompts; never satisfies a safety gate",
	)
	cfg.Flags.BoolVar(&cfg.DryRun, 0, "dry-run", "print the decision without mutating state")
	cfg.Command = &ff.Command{
		Name:      "approve",
		Usage:     "agentic-dev-harness approve <arc-id> --phrase <arc-id>",
		ShortHelp: "approve a pending human gate on an arc",
		LongHelp:  "Approve a pending gate with an explicit human phrase (SPEC §5.2). --yes never satisfies a gate.",
		Flags:     cfg.Flags,
		Exec:      cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

func (cfg *Config) exec(_ context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("approve: requires an arc id")
	}
	id := args[0]
	if cfg.Yes {
		_, _ = fmt.Fprintln(
			cfg.Stderr,
			"note: --yes does not satisfy a safety gate; the phrase is still required",
		)
	}
	store := state.Default()
	arc, err := store.Get(id)
	if err != nil {
		return fmt.Errorf("approve: %w", err)
	}
	if arc.Status != adh.StatusBlocked {
		return fmt.Errorf("approve: arc %s is not waiting at a gate (status %s)", id, arc.Status)
	}
	if !authority.GateSatisfied(id, cfg.Phrase, cfg.DryRun) {
		_, _ = fmt.Fprintf(cfg.Stderr, "gate not satisfied: pass --phrase %s to approve\n", id)
		return root.ExitError(4)
	}
	arc.Status = adh.StatusOpen
	arc.History = append(arc.History, "gate approved")
	if err := store.Save(&arc); err != nil {
		return fmt.Errorf("approve: %w", err)
	}
	_, _ = fmt.Fprintf(cfg.Stdout, "approved %s\n", id)
	return nil
}
