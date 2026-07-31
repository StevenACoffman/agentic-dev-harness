// Package nfrcmd implements the "nfr" CLI command: inspect the nonfunctional-
// requirement specs (SPEC-ADDITIONS §10.5) — list them, show one, or lint them for
// Planguage well-formedness (taxonomy, thresholds ordered, a meter and scale).
package nfrcmd

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/agentic-dev-harness/cmd/root"
	"github.com/StevenACoffman/agentic-dev-harness/internal/nfr"
)

// lintCode is the exit code for an invalid NFR spec (SPEC §7).
const lintCode = 17

// Config holds the configuration for the nfr command.
type Config struct {
	*root.Config
	Flags   *ff.FlagSet
	Command *ff.Command
}

// New creates and registers the nfr command with the given parent config.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("nfr").SetParent(parent.Flags)
	cfg.Command = &ff.Command{
		Name:      "nfr",
		Usage:     "agentic-dev-harness nfr <list|show|lint> [id]",
		ShortHelp: "list, show, and lint nonfunctional-requirement specs",
		LongHelp: "Inspect the nonfunctional-requirement specs (SPEC-ADDITIONS §10.5): a " +
			"Planguage-quantified quality attribute named by an agreed taxonomy. list " +
			"prints each spec's tag and scale; show prints one in full; lint validates " +
			"every spec (taxonomy, a meter and scale, and ordered fail/goal/stretch).",
		Flags: cfg.Flags,
		Exec:  cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

func (cfg *Config) exec(_ context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("nfr: expected a verb: list, show, or lint")
	}
	specs, err := nfr.Load(cfg.specDir())
	if err != nil {
		return fmt.Errorf("nfr: %w", err)
	}
	switch args[0] {
	case "list":
		return cfg.list(specs)
	case "show":
		return cfg.show(specs, args[1:])
	case "lint":
		return cfg.lint(specs)
	default:
		return fmt.Errorf("nfr: unknown verb %q; want list, show, or lint", args[0])
	}
}

// list prints each spec's id, tag, and scale.
func (cfg *Config) list(specs []nfr.Spec) error {
	for i := range specs {
		_, _ = fmt.Fprintf(cfg.Stdout, "%s\t%s\t%s\n", specs[i].ID, specs[i].Tag, specs[i].Scale)
	}
	return nil
}

// show prints one spec in full — the Planguage keywords a worker reasons over.
func (cfg *Config) show(specs []nfr.Spec, args []string) error {
	if len(args) == 0 {
		return errors.New("nfr: show requires a spec id")
	}
	id := args[0]
	for i := range specs {
		if specs[i].ID != id {
			continue
		}
		if cfg.JSONL {
			if err := cfg.EmitOK(specs[i]); err != nil {
				return fmt.Errorf("nfr: %w", err)
			}
			return nil
		}
		cfg.printSpec(&specs[i])
		return nil
	}
	return fmt.Errorf("nfr: no such spec %q", id)
}

// printSpec renders one spec's Planguage keywords for a human.
func (cfg *Config) printSpec(spec *nfr.Spec) {
	_, _ = fmt.Fprintf(cfg.Stdout, "%s  (%s)\n", spec.ID, spec.Tag)
	if spec.Gist != "" {
		_, _ = fmt.Fprintf(cfg.Stdout, "  Gist:     %s\n", spec.Gist)
	}
	_, _ = fmt.Fprintf(cfg.Stdout, "  Scale:    %s\n", spec.Scale)
	_, _ = fmt.Fprintf(cfg.Stdout, "  Meter:    %s\n", spec.Meter)
	_, _ = fmt.Fprintf(
		cfg.Stdout,
		"  %s-is-better  fail=%g goal=%g",
		spec.Direction,
		spec.Fail,
		spec.Goal,
	)
	if spec.Stretch != 0 {
		_, _ = fmt.Fprintf(cfg.Stdout, " stretch=%g", spec.Stretch)
	}
	_, _ = fmt.Fprintln(cfg.Stdout)
	if spec.Ambition != "" {
		_, _ = fmt.Fprintf(cfg.Stdout, "  Ambition: %s\n", spec.Ambition)
	}
}

// lint validates every spec's Planguage well-formedness, reporting each defect and
// exiting lintCode when any spec is invalid.
func (cfg *Config) lint(specs []nfr.Spec) error {
	bad := 0
	for i := range specs {
		if err := specs[i].Valid(); err != nil {
			bad++
			_, _ = fmt.Fprintf(cfg.Stderr, "%s\n", err)
		}
	}
	if bad > 0 {
		return root.ExitError(lintCode)
	}
	_, _ = fmt.Fprintf(cfg.Stdout, "%d NFR spec(s), all valid\n", len(specs))
	return nil
}

// specDir is the NFR spec directory under the repo root (the --repo global).
func (cfg *Config) specDir() string {
	repo := "."
	if cfg.Repo != "" {
		repo = cfg.Repo
	}
	return filepath.Join(repo, nfr.DefaultDir)
}
