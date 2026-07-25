// Package arc implements the "arc" CLI command: create, list, and show work
// arcs. It dispatches on a positional verb (new, list, show) and delegates
// persistence to internal/state and the state machine to internal/adh.
package arc

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/agentic-dev-harness/cmd/root"
	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/state"
)

// Config holds the configuration for the arc command.
type Config struct {
	*root.Config
	Flags   *ff.FlagSet
	Command *ff.Command
}

// New creates and registers the arc command with the given parent config.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("arc").SetParent(parent.Flags)
	cfg.Command = &ff.Command{
		Name:      "arc",
		Usage:     "agentic-dev-harness arc <new|list|show> [args]",
		ShortHelp: "create, list, and show work arcs",
		LongHelp:  "Manage work arcs: 'arc new <title>', 'arc list', 'arc show <id>'.",
		Flags:     cfg.Flags,
		Exec:      cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

func (cfg *Config) exec(_ context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("arc: expected a verb: new, list, or show")
	}
	store := state.Default()
	switch args[0] {
	case "new":
		return cfg.newArc(store, args[1:])
	case "list":
		return cfg.listArcs(store)
	case "show":
		return cfg.showArc(store, args[1:])
	default:
		return fmt.Errorf("arc: unknown verb %q; want new, list, or show", args[0])
	}
}

func (cfg *Config) newArc(store *state.Store, args []string) error {
	title := strings.TrimSpace(strings.Join(args, " "))
	if title == "" {
		return errors.New("arc: new requires a title")
	}
	id, err := store.NextID()
	if err != nil {
		return fmt.Errorf("arc: %w", err)
	}
	arc := adh.Arc{ID: id, Title: title, Stage: adh.StageStrategy, Status: adh.StatusOpen}
	if err := store.Create(&arc); err != nil {
		return fmt.Errorf("arc: %w", err)
	}
	_, _ = fmt.Fprintln(cfg.Stdout, id)
	return nil
}

func (cfg *Config) listArcs(store *state.Store) error {
	arcs, err := store.List()
	if err != nil {
		return fmt.Errorf("arc: %w", err)
	}
	if len(arcs) == 0 {
		_, _ = fmt.Fprintln(cfg.Stdout, "no arcs")
		return nil
	}
	for _, arc := range arcs {
		_, _ = fmt.Fprintf(cfg.Stdout, "%s\t%s\t%s\t%s\n", arc.ID, arc.Stage, arc.Status, arc.Title)
	}
	return nil
}

func (cfg *Config) showArc(store *state.Store, args []string) error {
	if len(args) == 0 {
		return errors.New("arc: show requires an id")
	}
	arc, err := store.Get(args[0])
	if err != nil {
		return fmt.Errorf("arc: %w", err)
	}
	res := string(arc.Resolution)
	if res == "" {
		res = "(unset)"
	}
	_, _ = fmt.Fprintf(cfg.Stdout,
		"id:         %s\ntitle:      %s\nstage:      %s\nstatus:     %s\nresolution: %s\n",
		arc.ID, arc.Title, arc.Stage, arc.Status, res)
	return nil
}
