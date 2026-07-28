// Package status implements the "status" CLI command: a summary of the harness
// state — open arcs, arcs blocked at a gate, and the current autonomy level.
package status

import (
	"context"
	"fmt"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/agentic-dev-harness/cmd/root"
	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/state"
)

// defaultAutonomy is the current autonomy level until config wiring (Phase 3)
// makes it configurable; SPEC §6 sets L2 as the default.
const defaultAutonomy = "L2"

// summary is the machine-readable harness state emitted under --jsonl.
type summary struct {
	Autonomy     string `json:"autonomy"`
	ArcsTotal    int    `json:"arcs_total"`
	ArcsOpen     int    `json:"arcs_open"`
	PendingGates int    `json:"pending_gates"`
}

// Config holds the configuration for the status command.
type Config struct {
	*root.Config
	Flags   *ff.FlagSet
	Command *ff.Command
}

// New creates and registers the status command with the given parent config.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("status").SetParent(parent.Flags)
	cfg.Command = &ff.Command{
		Name:      "status",
		Usage:     "agentic-dev-harness status",
		ShortHelp: "show harness state: active arcs and pending gates",
		LongHelp:  "Print the harness state: open arcs, arcs blocked at a gate, and the autonomy level.",
		Flags:     cfg.Flags,
		Exec:      cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

func (cfg *Config) exec(_ context.Context, _ []string) error {
	arcs, err := state.Default().List()
	if err != nil {
		return fmt.Errorf("status: %w", err)
	}
	var open, blocked int
	for i := range arcs {
		switch arcs[i].Status {
		case adh.StatusOpen:
			open++
		case adh.StatusBlocked:
			blocked++
		case adh.StatusClosed, adh.StatusFailed:
		}
	}
	if cfg.JSONL {
		if err := cfg.EmitOK(summary{
			Autonomy:     defaultAutonomy,
			ArcsTotal:    len(arcs),
			ArcsOpen:     open,
			PendingGates: blocked,
		}); err != nil {
			return fmt.Errorf("status: %w", err)
		}
		return nil
	}
	_, _ = fmt.Fprintf(cfg.Stdout,
		"autonomy:      %s\narcs total:    %d\narcs open:     %d\npending gates: %d\n",
		defaultAutonomy, len(arcs), open, blocked)
	return nil
}
