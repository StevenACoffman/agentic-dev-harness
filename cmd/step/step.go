// Package step implements the "step" CLI command: run exactly one stage
// transition on an arc through the model seam, then stop (SPEC §2.1).
package step

import (
	"context"
	"errors"
	"fmt"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/agentic-dev-harness/cmd/root"
	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/model"
	"github.com/StevenACoffman/agentic-dev-harness/internal/stage"
	"github.com/StevenACoffman/agentic-dev-harness/internal/state"
)

// Config holds the configuration for the step command.
type Config struct {
	*root.Config
	Flags   *ff.FlagSet
	Command *ff.Command
}

// New creates and registers the step command with the given parent config.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("step").SetParent(parent.Flags)
	cfg.Command = &ff.Command{
		Name:      "step",
		Usage:     "agentic-dev-harness step <arc-id>",
		ShortHelp: "run exactly one stage transition on an arc",
		LongHelp:  "Run the arc's current stage through the model, then stop (SPEC §2.1).",
		Flags:     cfg.Flags,
		Exec:      cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

func (cfg *Config) exec(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("step: requires an arc id")
	}
	store := state.Default()
	arc, err := store.Get(args[0])
	if err != nil {
		return fmt.Errorf("step: %w", err)
	}
	if arc.Status != adh.StatusOpen {
		return fmt.Errorf("step: arc %s is not open (status %s)", arc.ID, arc.Status)
	}
	if arc.Stage == adh.StageOps {
		return fmt.Errorf("step: arc %s is at ops; ship it with `close`", arc.ID)
	}
	if err := stage.Execute(ctx, model.Mock{}, &arc); err != nil {
		return fmt.Errorf("step: %w", err)
	}
	if err := store.Save(&arc); err != nil {
		return fmt.Errorf("step: %w", err)
	}
	_, _ = fmt.Fprintf(cfg.Stdout, "%s now at %s (%s)\n", arc.ID, arc.Stage, arc.Status)
	return nil
}
