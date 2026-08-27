// Package failurescmd implements the "failures" CLI command (SPEC §2.5): show the
// failure registry (§4.1) and the §11 lesson candidates, grouped into governing
// classes so a recurring one is visible. It is a thin shell over failures.Load
// and lesson.Distill; it records nothing.
package failurescmd

import (
	"context"
	"fmt"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/agentic-dev-harness/cmd/root"
	"github.com/StevenACoffman/agentic-dev-harness/internal/failures"
	"github.com/StevenACoffman/agentic-dev-harness/internal/lesson"
)

// Config holds the configuration for the failures command.
type Config struct {
	*root.Config
	Flags   *ff.FlagSet
	Command *ff.Command
}

// failureClass is one distilled class and how many notes it governs.
type failureClass struct {
	Class     string `json:"class"`
	Instances int    `json:"instances"`
}

// New creates and registers the failures command with the given parent config.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("failures").SetParent(parent.Flags)
	cfg.Command = &ff.Command{
		Name:      "failures",
		Usage:     "agentic-dev-harness failures list",
		ShortHelp: "show the failure registry and lesson candidates",
		LongHelp:  "List the failure registry (§4.1) and §11 lesson candidates, grouped by governing class.",
		Flags:     cfg.Flags,
		Exec:      cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

func (cfg *Config) exec(_ context.Context, args []string) error {
	if len(args) == 0 {
		return root.MissingVerbError{
			Scope: "failures",
			Verbs: []string{"list"},
		}
	}
	switch args[0] {
	case "list":
		return cfg.list()
	default:
		return root.UnknownVerbError{
			Scope: "failures", Got: args[0],
			Verbs: []string{"list"},
		}
	}
}

func (cfg *Config) list() error {
	confirmed, err := failures.Load(failures.RegistryFile)
	if err != nil {
		return fmt.Errorf("failures: %w", err)
	}
	candidates, err := failures.Load(failures.CandidatesFile)
	if err != nil {
		return fmt.Errorf("failures: %w", err)
	}
	if cfg.JSONL {
		// Both keys always present and always lists: "no failures recorded" is an
		// empty confirmed set, and a caller should not have to tell that from a
		// command that declined to answer.
		if err := cfg.EmitOK(map[string]any{
			"confirmed":  classes(confirmed),
			"candidates": classes(candidates),
		}); err != nil {
			return fmt.Errorf("failures: %w", err)
		}
		return nil
	}
	if len(confirmed) == 0 && len(candidates) == 0 {
		_, _ = fmt.Fprintln(cfg.Stdout, "no failures recorded")
		return nil
	}
	cfg.printClasses("confirmed failures", confirmed)
	cfg.printClasses("lesson candidates", candidates)
	return nil
}

// classes distils notes the same way printClasses renders them, so the two cannot
// disagree about what a class is or how many instances it has.
func classes(notes []string) []failureClass {
	distilled := lesson.Distill(notes)
	out := make([]failureClass, 0, len(distilled))
	for i := range distilled {
		out = append(out, failureClass{
			Class:     distilled[i].Class,
			Instances: len(distilled[i].Instances),
		})
	}
	return out
}

// printClasses distills notes into governing classes and prints each with its
// instance count, so a recurring class stands out.
func (cfg *Config) printClasses(heading string, notes []string) {
	_, _ = fmt.Fprintf(cfg.Stdout, "%s (%d):\n", heading, len(notes))
	for _, l := range lesson.Distill(notes) {
		_, _ = fmt.Fprintf(cfg.Stdout, "  %s\t(%d instance(s))\n", l.Class, len(l.Instances))
	}
}
