// Package autonomy implements the "autonomy" CLI command: show or set the
// autonomy ladder level (SPEC §6). The level persists in .adh/autonomy.
package autonomy

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/agentic-dev-harness/cmd/root"
	"github.com/StevenACoffman/agentic-dev-harness/internal/authority"
	"github.com/StevenACoffman/agentic-dev-harness/internal/config"
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
		return root.MissingVerbError{
			Scope: "autonomy",
			Verbs: []string{"show", "set"},
		}
	}
	switch args[0] {
	case "show":
		conf, err := config.Load(cfg.ConfigGetenv())
		if err != nil {
			return fmt.Errorf("autonomy: %w", err)
		}
		if cfg.JSONL {
			if err := cfg.EmitOK(map[string]any{
				"level": conf.AutonomyLevel().String(),
			}); err != nil {
				return fmt.Errorf("autonomy: %w", err)
			}
			return nil
		}
		_, _ = fmt.Fprintln(cfg.Stdout, conf.AutonomyLevel().String())
		return nil
	case "set":
		return cfg.set(args[1:])
	default:
		return root.UnknownVerbError{
			Scope: "autonomy", Got: args[0],
			Verbs: []string{"show", "set"},
		}
	}
}

func (cfg *Config) set(args []string) error {
	if len(args) == 0 {
		return root.MissingOperandError{Scope: "autonomy set", Kind: root.OperandLevel}
	}
	next, err := authority.ParseLevel(args[0])
	if err != nil {
		return fmt.Errorf("autonomy: %w", err)
	}
	conf, err := config.Load(cfg.ConfigGetenv())
	if err != nil {
		return fmt.Errorf("autonomy: %w", err)
	}
	cur := conf.AutonomyLevel()
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
	if cfg.JSONL {
		// `level` is the key `autonomy show` already uses for the same value, so a
		// caller reads the level the same way whether it just set it or asked. The
		// gating note stays on stderr: it is advice about the next raise, not the
		// outcome of this one.
		if err := cfg.EmitOK(map[string]any{"level": next.String()}); err != nil {
			return fmt.Errorf("autonomy: %w", err)
		}
		return nil
	}
	_, _ = fmt.Fprintf(cfg.Stdout, "autonomy set to %s\n", next)
	return nil
}
