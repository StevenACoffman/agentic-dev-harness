// Package toolcmd implements the "tool" CLI command: inspect the tool registry
// (SPEC-ADDITIONS §13) — list declared tools or run doctor to validate them.
package toolcmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/agentic-dev-harness/cmd/root"
	"github.com/StevenACoffman/agentic-dev-harness/internal/toolreg"
)

// Config holds the configuration for the tool command.
type Config struct {
	*root.Config
	Flags   *ff.FlagSet
	Command *ff.Command
}

// New creates and registers the tool command with the given parent config.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("tool").SetParent(parent.Flags)
	cfg.Command = &ff.Command{
		Name:      "tool",
		Usage:     "agentic-dev-harness tool <list|doctor>",
		ShortHelp: "list and check the tool registry",
		LongHelp:  "Inspect the tool registry (SPEC-ADDITIONS §13): list declared tools or validate them.",
		Flags:     cfg.Flags,
		Exec:      cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

func (cfg *Config) exec(_ context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("tool: expected a verb: list or doctor")
	}
	reg, err := toolreg.Load(toolreg.DefaultRegistryFile)
	if err != nil {
		return fmt.Errorf("tool: %w", err)
	}
	switch args[0] {
	case "list":
		for _, tool := range reg.Tools {
			_, _ = fmt.Fprintf(cfg.Stdout, "%s\tverifies: %s\n", tool.ID, tool.Verifies)
		}
		return nil
	case "doctor":
		if verr := reg.Validate(); verr != nil {
			_, _ = fmt.Fprintf(cfg.Stderr, "tool registry invalid: %s\n", verr)
			return root.ExitError(10)
		}
		_, _ = fmt.Fprintf(cfg.Stdout, "%d tools declared, registry valid\n", len(reg.Tools))
		return nil
	default:
		return fmt.Errorf("tool: unknown verb %q; want list or doctor", args[0])
	}
}
