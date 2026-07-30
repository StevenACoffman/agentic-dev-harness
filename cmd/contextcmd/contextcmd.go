// Package contextcmd implements the "context" CLI command: inspect the
// just-in-time context store (SPEC-ADDITIONS §10) — list units, show one unit's
// text and provenance, route a working set by labels, or lint the store.
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
		Usage:     "agentic-dev-harness context <list|show|route|lint> [id|labels...]",
		ShortHelp: "list, show, route, and lint context units",
		LongHelp: "Inspect the just-in-time context store (SPEC-ADDITIONS §10): list " +
			"units, show one unit's text and provenance, route a working set by labels, " +
			"or lint the store.",
		Flags: cfg.Flags,
		Exec:  cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

func (cfg *Config) exec(_ context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("context: expected a verb: list, show, route, or lint")
	}
	units, err := contextstore.Load(contextstore.DefaultStoreDir)
	if err != nil {
		return fmt.Errorf("context: %w", err)
	}
	switch args[0] {
	case "list":
		return cfg.list(units)
	case "show":
		return cfg.show(units, args[1:])
	case "route":
		routed := contextstore.Route(units, args[1:], nil, contextstore.DefaultWorkingSet)
		for i := range routed {
			_, _ = fmt.Fprintln(cfg.Stdout, routed[i].ID)
		}
		return nil
	case "lint":
		return cfg.lint(units)
	default:
		return fmt.Errorf("context: unknown verb %q; want list, show, route, or lint", args[0])
	}
}

// list prints each unit's id, kind, and labels.
func (cfg *Config) list(units []contextstore.Unit) error {
	for i := range units {
		_, _ = fmt.Fprintf(cfg.Stdout, "%s\t%s\t%s\n",
			units[i].ID, units[i].Kind, strings.Join(units[i].Labels, ","))
	}
	return nil
}

// show prints one unit's text and provenance — the content the routing preview
// points a worker at, pulled just in time (§10.4). Under --jsonl it emits one OK
// outcome carrying the metadata, provenance, and content.
func (cfg *Config) show(units []contextstore.Unit, args []string) error {
	if len(args) == 0 {
		return errors.New("context: show requires a unit id")
	}
	id := args[0]
	for i := range units {
		unit := &units[i]
		if unit.ID != id {
			continue
		}
		content, err := contextstore.Content(contextstore.DefaultStoreDir, unit)
		if err != nil {
			return fmt.Errorf("context: %w", err)
		}
		return cfg.reportUnit(unit, content)
	}
	return fmt.Errorf("context: no such unit %q", id)
}

// reportUnit emits a unit and its content, as one outcome under --jsonl else text.
func (cfg *Config) reportUnit(unit *contextstore.Unit, content string) error {
	if cfg.JSONL {
		if err := cfg.EmitOK(map[string]any{
			"id": unit.ID, "kind": unit.Kind, "owner": unit.Owner,
			"provenance": unit.Provenance, "content": content,
		}); err != nil {
			return fmt.Errorf("context: %w", err)
		}
		return nil
	}
	if unit.Provenance != "" {
		_, _ = fmt.Fprintf(cfg.Stdout, "# %s (%s) — %s\n", unit.ID, unit.Kind, unit.Provenance)
	} else {
		_, _ = fmt.Fprintf(cfg.Stdout, "# %s (%s)\n", unit.ID, unit.Kind)
	}
	if content != "" {
		_, _ = fmt.Fprintln(cfg.Stdout, content)
	}
	return nil
}

func (cfg *Config) lint(units []contextstore.Unit) error {
	bad := 0
	for i := range units {
		unit := &units[i]
		if unit.ID == "" || unit.Kind == "" {
			bad++
			_, _ = fmt.Fprintf(cfg.Stderr, "unit missing id or kind: %+v\n", unit)
			continue
		}
		// The content the routing preview promises must exist and stay in the store.
		if _, err := contextstore.Content(contextstore.DefaultStoreDir, unit); err != nil {
			bad++
			_, _ = fmt.Fprintf(
				cfg.Stderr,
				"unit %s: content_path does not resolve: %v\n",
				unit.ID,
				err,
			)
		}
	}
	if bad > 0 {
		return root.ExitError(12)
	}
	_, _ = fmt.Fprintf(cfg.Stdout, "%d context units, all valid\n", len(units))
	return nil
}
