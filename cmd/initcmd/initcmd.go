// Package initcmd implements the "init" CLI command: scaffold the .adh
// workspace so later commands have somewhere to store arcs.
package initcmd

import (
	"context"
	"fmt"
	"os"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/agentic-dev-harness/cmd/root"
	"github.com/StevenACoffman/agentic-dev-harness/internal/state"
)

// Config holds the configuration for the init command.
type Config struct {
	*root.Config
	Flags   *ff.FlagSet
	Command *ff.Command
}

// New creates and registers the init command with the given parent config.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("init").SetParent(parent.Flags)
	cfg.Command = &ff.Command{
		Name:      "init",
		Usage:     "agentic-dev-harness init",
		ShortHelp: "scaffold the .adh workspace",
		LongHelp:  "Create the .adh workspace (the arc store) in the current repository.",
		Flags:     cfg.Flags,
		Exec:      cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

func (cfg *Config) exec(_ context.Context, _ []string) error {
	if err := os.MkdirAll(state.DefaultArcsDir, 0o750); err != nil {
		return fmt.Errorf("init: create workspace: %w", err)
	}
	_, _ = fmt.Fprintf(cfg.Stdout, "initialized workspace at %s\n", state.DefaultArcsDir)
	return nil
}
