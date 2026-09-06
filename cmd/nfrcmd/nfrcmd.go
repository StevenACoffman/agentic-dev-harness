// Package nfrcmd implements the "nfr" CLI command: inspect the nonfunctional-
// requirement specs (SPEC-ADDITIONS §10.5) — list them, show one, or lint them for
// Planguage well-formedness (taxonomy, thresholds ordered, a meter and scale).
package nfrcmd

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/agentic-dev-harness/cmd/root"
	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
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
		return root.MissingVerbError{
			Scope: "nfr",
			Verbs: []string{"list", "show", "lint"},
		}
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
		return root.UnknownVerbError{
			Scope: "nfr", Got: args[0],
			Verbs: []string{"list", "show", "lint"},
		}
	}
}

// noExtraArgs refuses a verb's trailing arguments.
//
// The flag case — `nfr list --guards`, where ff leaves the flag unparsed and the
// command lists everything having been told not to — is caught earlier and centrally,
// by cmd.Run's stray-flag guard, so every verb in adh gets it rather than this one.
// What is left here is the non-flag case, `nfr list foo`, which is command-specific:
// several verbs take a positional legitimately and these two do not.
func noExtraArgs(verb string, rest []string) error {
	if len(rest) == 0 {
		return nil
	}
	return &adh.Error{
		Code:    adh.EINVALID,
		Message: fmt.Sprintf("nfr: %s takes no arguments, got %q", verb, rest[0]),
	}
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
	if cfg.JSONL {
		// One envelope per spec, as `arc list` and `tool list` do. `guard` travels as
		// the bool rather than the display word: a caller branches on the property,
		// and `role` exists to make the human column scannable by eye.
		//
		// This branch was missing. The contract test's seeded tree mints no spec, so
		// `nfr list` answered from an empty store, printed nothing, and satisfied
		// "empty or JSON" without reaching the loop -- the same too-poor-fixture blind
		// spot that hid five other commands until the seeded tree existed.
		for i := range specs {
			if err := cfg.EmitOK(map[string]any{
				"id": specs[i].ID, "tag": specs[i].Tag,
				"scale": specs[i].Scale, "guard": specs[i].Guard,
			}); err != nil {
				return fmt.Errorf("nfr: %w", err)
			}
		}
		return nil
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
		return root.MissingOperandError{Scope: "nfr show", Kind: root.OperandSpec}
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
	return &adh.Error{
		Code:    adh.ENOTFOUND,
		Message: fmt.Sprintf("nfr: no such spec %q", id),
	}
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
	if cfg.JSONL {
		if err := cfg.EmitOK(map[string]any{"specs": len(specs), "valid": true}); err != nil {
			return fmt.Errorf("nfr: %w", err)
		}
		return nil
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
