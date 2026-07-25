// Package autonomy implements the "autonomy" CLI command: show or set the
// autonomy ladder level (SPEC §6). The level persists in .adh/autonomy.
package autonomy

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/agentic-dev-harness/cmd/root"
	"github.com/StevenACoffman/agentic-dev-harness/internal/authority"
)

const stateFile = ".adh/autonomy"

// Config holds the configuration for the autonomy command.
type Config struct {
	*root.Config
	Flags   *ff.FlagSet
	Command *ff.Command
}

// New creates and registers the autonomy command with the given parent config.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("autonomy").SetParent(parent.Flags)
	cfg.Command = &ff.Command{
		Name:      "autonomy",
		Usage:     "agentic-dev-harness autonomy <show|set L0-L4>",
		ShortHelp: "show or set the autonomy level (L0-L4)",
		LongHelp:  "Show or set the autonomy level. Raising it is itself a human-gated action (SPEC §6).",
		Flags:     cfg.Flags,
		Exec:      cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

func (cfg *Config) exec(_ context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("autonomy: expected a verb: show or set")
	}
	switch args[0] {
	case "show":
		_, _ = fmt.Fprintln(cfg.Stdout, cfg.load().String())
		return nil
	case "set":
		return cfg.set(args[1:])
	default:
		return fmt.Errorf("autonomy: unknown verb %q; want show or set", args[0])
	}
}

func (cfg *Config) set(args []string) error {
	if len(args) == 0 {
		return errors.New("autonomy: set requires a level (L0-L4)")
	}
	next, err := authority.ParseLevel(args[0])
	if err != nil {
		return fmt.Errorf("autonomy: %w", err)
	}
	cur := cfg.load()
	if authority.RaiseIsGated(cur, next) {
		_, _ = fmt.Fprintf(
			cfg.Stderr,
			"note: raising %s -> %s is a human-gated action; reliability earns launch-automation, never gate-removal\n",
			cur,
			next,
		)
	}
	if err := os.MkdirAll(filepath.Dir(stateFile), 0o750); err != nil {
		return fmt.Errorf("autonomy: %w", err)
	}
	if err := os.WriteFile(stateFile, []byte(next.String()+"\n"), 0o600); err != nil {
		return fmt.Errorf("autonomy: %w", err)
	}
	_, _ = fmt.Fprintf(cfg.Stdout, "autonomy set to %s\n", next)
	return nil
}

// load returns the persisted level, defaulting to L2 when the file is absent or
// unreadable (SPEC §6 default).
func (cfg *Config) load() authority.Level {
	data, err := os.ReadFile(stateFile)
	if err != nil {
		return authority.L2
	}
	lvl, err := authority.ParseLevel(strings.TrimSpace(string(data)))
	if err != nil {
		return authority.L2
	}
	return lvl
}
