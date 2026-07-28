// Package contextcmd implements the "context" CLI command: inspect the
// just-in-time context store (SPEC-ADDITIONS §10) — list units, route a working
// set by labels, or lint the store.
package contextcmd

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/agentic-dev-harness/cmd/root"
	"github.com/StevenACoffman/agentic-dev-harness/internal/contextstore"
)

// Config holds the configuration for the context command.
type Config struct {
	*root.Config
	Flags   *ff.FlagSet
	Command *ff.Command
}

// New creates and registers the context command with the given parent config.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("context").SetParent(parent.Flags)
	cfg.Command = &ff.Command{
		Name:      "context",
		Usage:     "agentic-dev-harness context <list|route|lint> [labels...]",
		ShortHelp: "list, route, and lint context units",
		LongHelp:  "Inspect the just-in-time context store (SPEC-ADDITIONS §10).",
		Flags:     cfg.Flags,
		Exec:      cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

func (cfg *Config) exec(_ context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("context: expected a verb: list, route, or lint")
	}
	units, err := contextstore.Load(contextstore.DefaultStoreDir)
	if err != nil {
		return fmt.Errorf("context: %w", err)
	}
	switch args[0] {
	case "list":
		for _, unit := range units {
			_, _ = fmt.Fprintf(
				cfg.Stdout,
				"%s\t%s\t%s\n",
				unit.ID,
				unit.Kind,
				strings.Join(unit.Labels, ","),
			)
		}
		return nil
	case "route":
		for _, unit := range contextstore.Route(units, args[1:], nil, contextstore.DefaultWorkingSet) {
			_, _ = fmt.Fprintln(cfg.Stdout, unit.ID)
		}
		return nil
	case "lint":
		return cfg.lint(units)
	default:
		return fmt.Errorf("context: unknown verb %q; want list, route, or lint", args[0])
	}
}

func (cfg *Config) lint(units []contextstore.Unit) error {
	bad := 0
	for _, unit := range units {
		if unit.ID == "" || unit.Kind == "" {
			bad++
			_, _ = fmt.Fprintf(cfg.Stderr, "unit missing id or kind: %+v\n", unit)
		}
	}
	if bad > 0 {
		return root.ExitError(12)
	}
	_, _ = fmt.Fprintf(cfg.Stdout, "%d context units, all valid\n", len(units))
	return nil
}
