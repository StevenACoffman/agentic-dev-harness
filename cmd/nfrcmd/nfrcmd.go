// Package nfrcmd implements the "nfr" CLI command: inspect the nonfunctional-
// requirement specs (SPEC-ADDITIONS §10.5) — list them, show one, or lint them for
// Planguage well-formedness (taxonomy, thresholds ordered, a meter and scale).
package nfrcmd

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/agentic-dev-harness/cmd/root"
	"github.com/StevenACoffman/agentic-dev-harness/internal/nfr"
)

// lintCode is the exit code for an invalid NFR spec (SPEC §7).
const lintCode = 17

// Config holds the configuration for the nfr command.
type Config struct {
	*root.Config

	// GuardsOnly narrows list to the specs a change must not breach.
	//
	// A filter and not the answer to "which are guards" — the guard column below is
	// that, and it is always shown. This is for the other question, "show me only what
	// constrains this change", which is worth asking once the list is long.
	GuardsOnly bool

	Flags   *ff.FlagSet
	Command *ff.Command
}

// New creates and registers the nfr command with the given parent config.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("nfr").SetParent(parent.Flags)
	cfg.Flags.BoolVar(&cfg.GuardsOnly, 'g', "guards", "list only the guards")
	cfg.Command = &ff.Command{
		Name:      "nfr",
		Usage:     "agentic-dev-harness nfr <list|show|lint> [id]",
		ShortHelp: "list, show, and lint nonfunctional-requirement specs",
		LongHelp: "Inspect the nonfunctional-requirement specs (SPEC-ADDITIONS §10.5): a " +
			"Planguage-quantified quality attribute named by an agreed taxonomy. list " +
			"prints each spec's tag, scale, and whether it is a guard; show prints one " +
			"in full; lint validates every spec (taxonomy, a meter and scale, and " +
			"ordered fail/goal/stretch).\n\n" +
			"A guard is a spec the change must not breach, as opposed to one it is " +
			"trying to improve (§19.2). It is shown in every listing rather than behind " +
			"a flag, because the whole value of the guard set is that it is known " +
			"before the work starts: an objective without guards is hill-climbed by " +
			"trading away everything unmeasured. --guards narrows to them.",
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
		if err := noExtraArgs("list", args[1:]); err != nil {
			return err
		}
		return cfg.list(specs)
	case "show":
		return cfg.show(specs, args[1:])
	case "lint":
		if err := noExtraArgs("lint", args[1:]); err != nil {
			return err
		}
		return cfg.lint(specs)
	default:
		return fmt.Errorf("nfr: unknown verb %q; want list, show, or lint", args[0])
	}
}

// noExtraArgs refuses a verb's trailing arguments, naming the flag case specially.
//
// ff stops parsing flags at the first positional, so `nfr list --guards` leaves
// --guards as a trailing argument and the flag unset — and the command would then list
// everything, having been told not to. **A flag that is silently ignored is worse than
// one that is unsupported**: the caller believes the filter applied. Found by running
// the command, not by a test.
//
// The message names the working form rather than describing the parser, because a
// reader hitting this wants the line that works.
func noExtraArgs(verb string, rest []string) error {
	if len(rest) == 0 {
		return nil
	}
	if strings.HasPrefix(rest[0], "-") {
		return fmt.Errorf("nfr: flags come before the verb: try `nfr %s %s`", rest[0], verb)
	}
	return fmt.Errorf("nfr: %s takes no arguments, got %q", verb, rest[0])
}

// list prints each spec's id, tag, scale, and role.
//
// The role column is always present, and that is the item this closes rather than a
// formatting choice. `nfr.Spec.Guard` shipped with no reader, so the answer to "what
// will this change be held to?" was a grep over the spec files — and the failure the
// guard set exists to prevent is an author optimising an objective without knowing what
// it must not cost. A filter would show the guards to someone who already suspected
// they mattered; a column shows them to anyone reading the specs for any reason.
func (cfg *Config) list(specs []nfr.Spec) error {
	if cfg.GuardsOnly {
		specs = nfr.Guards(specs)
	}
	for i := range specs {
		_, _ = fmt.Fprintf(cfg.Stdout, "%s\t%s\t%s\t%s\n",
			specs[i].ID, specs[i].Tag, specs[i].Scale, role(&specs[i]))
	}
	return nil
}

// role names what a spec is for, in a word a reader can scan a column of.
//
// "GUARD" is capitalised and "objective" is not, so the constrained specs are findable
// by eye in a long list without reading the column header.
func role(s *nfr.Spec) string {
	if s.Guard {
		return "GUARD"
	}
	return "objective"
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
