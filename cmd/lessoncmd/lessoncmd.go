// Package lessoncmd implements the "lesson" CLI command: distill the failure
// registry into candidate lessons and promote one to a durable owner
// (SPEC-ADDITIONS §11). Promotion to an executable owner is human-gated.
package lessoncmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/agentic-dev-harness/cmd/root"
	"github.com/StevenACoffman/agentic-dev-harness/internal/failures"
	lessonlib "github.com/StevenACoffman/agentic-dev-harness/internal/lesson"
)

// Config holds the configuration for the lesson command.
type Config struct {
	*root.Config
	To      string
	Approve bool
	Flags   *ff.FlagSet
	Command *ff.Command
}

// New creates and registers the lesson command with the given parent config.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("lesson").SetParent(parent.Flags)
	cfg.Flags.StringVar(
		&cfg.To,
		0,
		"to",
		"",
		"durable owner: context|skill|doc|check|invariant|type",
	)
	cfg.Flags.BoolVar(&cfg.Approve, 0, "approve", "approve promotion to an executable owner")
	cfg.Command = &ff.Command{
		Name:      "lesson",
		Usage:     "agentic-dev-harness lesson <list|promote> [class] [--to owner]",
		ShortHelp: "distill failures into lessons and promote them",
		LongHelp:  "List candidate lessons distilled from the failure registry, or promote one (SPEC-ADDITIONS §11).",
		Flags:     cfg.Flags,
		Exec:      cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

func (cfg *Config) exec(_ context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("lesson: expected a verb: list or promote")
	}
	switch args[0] {
	case "list":
		return cfg.list()
	case "promote":
		return cfg.promote(args[1:])
	default:
		return fmt.Errorf("lesson: unknown verb %q; want list or promote", args[0])
	}
}

func (cfg *Config) list() error {
	notes, err := failures.Load(failures.RegistryFile)
	if err != nil {
		return fmt.Errorf("lesson: %w", err)
	}
	for _, l := range lessonlib.Distill(notes) {
		_, _ = fmt.Fprintf(cfg.Stdout, "%s\t(%d instances)\n", l.Class, len(l.Instances))
	}
	return nil
}

func (cfg *Config) promote(args []string) error {
	if len(args) == 0 {
		return errors.New("lesson: promote requires a class")
	}
	if cfg.To == "" {
		return errors.New("lesson: promote requires --to <owner>")
	}
	owner := lessonlib.Owner(cfg.To)
	if owner.RequiresApproval() && !cfg.Approve {
		_, _ = fmt.Fprintf(
			cfg.Stderr,
			"promotion of %q to executable owner %q requires approval; re-run with --approve\n",
			args[0],
			owner,
		)
		return root.ExitError(13)
	}
	_, _ = fmt.Fprintf(cfg.Stdout, "promoted %q to %s\n", args[0], owner)
	return nil
}
