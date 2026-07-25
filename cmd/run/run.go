// Package run implements the "run" CLI command: relay an arc through stages
// until it reaches a human gate or closes (SPEC §2.1), honoring the autonomy
// level. Until config wiring lands it relays at the default level L2.
package run

import (
	"context"
	"errors"
	"fmt"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/agentic-dev-harness/cmd/root"
	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/authority"
	"github.com/StevenACoffman/agentic-dev-harness/internal/model"
	"github.com/StevenACoffman/agentic-dev-harness/internal/stage"
	"github.com/StevenACoffman/agentic-dev-harness/internal/state"
)

// Config holds the configuration for the run command.
type Config struct {
	*root.Config
	Flags   *ff.FlagSet
	Command *ff.Command
}

// New creates and registers the run command with the given parent config.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("run").SetParent(parent.Flags)
	cfg.Command = &ff.Command{
		Name:      "run",
		Usage:     "agentic-dev-harness run <arc-id>",
		ShortHelp: "advance an arc through the loop until a gate or completion",
		LongHelp:  "Relay the arc through stages until a human gate or closure (SPEC §2.1).",
		Flags:     cfg.Flags,
		Exec:      cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

func (cfg *Config) exec(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("run: requires an arc id")
	}
	store := state.Default()
	arc, err := store.Get(args[0])
	if err != nil {
		return fmt.Errorf("run: %w", err)
	}
	for arc.Status == adh.StatusOpen {
		if arc.Stage == adh.StageOps {
			return cfg.block(store, &arc, "ops is the ship gate; approve then `close`")
		}
		from := arc.Stage
		if err := stage.Execute(ctx, model.Mock{}, &arc); err != nil {
			return fmt.Errorf("run: %w", err)
		}
		if err := store.Save(&arc); err != nil {
			return fmt.Errorf("run: %w", err)
		}
		_, _ = fmt.Fprintf(cfg.Stdout, "ran %s\n", from)
		if !stage.AutoAdvances(from, authority.L2) {
			return cfg.block(store, &arc, string(arc.Stage)+" requires a human gate")
		}
	}
	_, _ = fmt.Fprintf(cfg.Stdout, "arc %s %s\n", arc.ID, arc.Status)
	return nil
}

// block parks the arc at a human gate (StatusBlocked) and records why, so the
// approve/reject loop can act on it. Blocking at a gate is not an error.
func (cfg *Config) block(store *state.Store, arc *adh.Arc, reason string) error {
	arc.Status = adh.StatusBlocked
	arc.History = append(arc.History, "blocked: "+reason)
	if err := store.Save(arc); err != nil {
		return fmt.Errorf("run: %w", err)
	}
	_, _ = fmt.Fprintf(cfg.Stdout, "blocked at %s: %s\n", arc.Stage, reason)
	return nil
}
