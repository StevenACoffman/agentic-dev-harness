// Package reject implements the "reject" CLI command: reject a pending human
// gate on an arc, returning it to a failed state with an optional reason.
package reject

import (
	"context"
	"errors"
	"fmt"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/agentic-dev-harness/cmd/root"
	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/state"
)

// Config holds the configuration for the reject command.
type Config struct {
	*root.Config
	Reason  string
	Flags   *ff.FlagSet
	Command *ff.Command
}

// New creates and registers the reject command with the given parent config.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("reject").SetParent(parent.Flags)
	cfg.Flags.StringVar(&cfg.Reason, 'r', "reason", "", "why the gate was rejected")
	cfg.Command = &ff.Command{
		Name:      "reject",
		Usage:     "agentic-dev-harness reject <arc-id> [--reason text]",
		ShortHelp: "reject a pending human gate on an arc",
		LongHelp:  "Reject a pending gate, returning the arc to a failed state with an optional reason.",
		Flags:     cfg.Flags,
		Exec:      cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

func (cfg *Config) exec(_ context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("reject: requires an arc id")
	}
	id := args[0]
	store := state.Default()
	arc, err := store.Get(id)
	if err != nil {
		return fmt.Errorf("reject: %w", err)
	}
	note := "gate rejected"
	if cfg.Reason != "" {
		note += ": " + cfg.Reason
	}
	arc.Status = adh.StatusFailed
	arc.History = append(arc.History, note)
	if err := store.Save(&arc); err != nil {
		return fmt.Errorf("reject: %w", err)
	}
	_, _ = fmt.Fprintf(cfg.Stdout, "rejected %s\n", id)
	return nil
}
