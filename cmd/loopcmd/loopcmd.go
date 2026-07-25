// Package loopcmd implements the "loop" CLI command: list, run, and retire
// maintenance loops (SPEC-ADDITIONS §15).
package loopcmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/agentic-dev-harness/cmd/root"
	looplib "github.com/StevenACoffman/agentic-dev-harness/internal/loop"
)

const registryFile = ".adh/loops.json"

// Config holds the configuration for the loop command.
type Config struct {
	*root.Config
	Flags   *ff.FlagSet
	Command *ff.Command
}

// New creates and registers the loop command with the given parent config.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("loop").SetParent(parent.Flags)
	cfg.Command = &ff.Command{
		Name:      "loop",
		Usage:     "agentic-dev-harness loop <list|run|retire> [id]",
		ShortHelp: "list, run, and retire maintenance loops",
		LongHelp:  "Manage maintenance loops (SPEC-ADDITIONS §15): list, run, or retire one.",
		Flags:     cfg.Flags,
		Exec:      cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

func (cfg *Config) exec(_ context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("loop: expected a verb: list, run, or retire")
	}
	reg, err := looplib.Load(registryFile)
	if err != nil {
		return fmt.Errorf("loop: %w", err)
	}
	switch args[0] {
	case "list":
		if verr := reg.Validate(); verr != nil {
			return fmt.Errorf("loop: %w", verr)
		}
		for _, l := range reg.Loops {
			_, _ = fmt.Fprintf(cfg.Stdout, "%s\t%s\n", l.ID, l.Goal)
		}
		return nil
	case "run":
		return cfg.act(reg, args[1:], "would run sensor")
	case "retire":
		return cfg.act(reg, args[1:], "retired")
	default:
		return fmt.Errorf("loop: unknown verb %q; want list, run, or retire", args[0])
	}
}

func (cfg *Config) act(reg looplib.Registry, args []string, verb string) error {
	if len(args) == 0 {
		return errors.New("loop: run and retire require a loop id")
	}
	loop, ok := reg.Find(args[0])
	if !ok {
		return fmt.Errorf("loop: no such loop %q", args[0])
	}
	if verb == "would run sensor" {
		_, _ = fmt.Fprintf(cfg.Stdout, "%s: %s\n", verb, loop.Sensor)
		return nil
	}
	_, _ = fmt.Fprintf(cfg.Stdout, "%s %s (retire when: %s)\n", verb, loop.ID, loop.RetireWhen)
	return nil
}
