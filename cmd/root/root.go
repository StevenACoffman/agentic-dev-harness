// Package root defines the root configuration for the CLI.
package root

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/peterbourgon/ff/v4"
)

// ExitError is returned by commands that want a specific non-zero exit code
// without printing an additional error message. run() in main.go checks for
// ExitError with errors.As and calls os.Exit(int(e)) directly, bypassing the
// default "error: ..." printer.
type ExitError int

// Config holds shared I/O writers, the injected environment accessor, the global
// flags (SPEC §2, bound once), and the root ff.Command. All subcommand configs
// embed *Config to inherit these.
type Config struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
	Getenv func(string) string
	// Global flags, available on every command via the parent flag chain.
	ConfigPath string // --config: override the config file path
	Profile    string // --profile: named config profile (not yet wired; SPEC §3 tier 3)
	Repo       string // --repo: target repository root (not yet threaded to the store)
	Verbose    bool   // --verbose: increase log verbosity (no logging subsystem yet)
	Quiet      bool   // --quiet: suppress non-error stdout (wired in cmd.Run)
	NoColor    bool   // --no-color: disable ANSI color (no color output yet)
	JSONL      bool   // --jsonl: emit machine output as JSON Lines (SPEC §8)
	Flags      *ff.FlagSet
	Command    *ff.Command
}

// Error reports the exit status. ExitError satisfies the error interface so a
// command can request a specific exit code without printing an "error: ..." line.
func (e ExitError) Error() string { return fmt.Sprintf("exit status %d", int(e)) }

// New returns a new root Config with the given I/O writers and environment
// accessor. getenv is injected (not os.Getenv) so config precedence is testable.
func New(getenv func(string) string, stdin io.Reader, stdout, stderr io.Writer) *Config {
	var cfg Config
	cfg.Stdin = stdin
	cfg.Stdout = stdout
	cfg.Stderr = stderr
	cfg.Getenv = getenv
	// Global flags (SPEC §2), bound once here; subcommands inherit them via
	// SetParent(parent.Flags), so `adh <cmd> --quiet` resolves through the chain.
	cfg.Flags = ff.NewFlagSet("agentic-dev-harness")
	cfg.Flags.StringVar(&cfg.ConfigPath, 0, "config", "", "override the config file path")
	cfg.Flags.StringVar(&cfg.Profile, 0, "profile", "", "select a named config profile")
	cfg.Flags.StringVar(&cfg.Repo, 0, "repo", "", "target repository root")
	cfg.Flags.BoolVar(&cfg.Verbose, 'v', "verbose", "increase log verbosity")
	cfg.Flags.BoolVar(&cfg.Quiet, 'q', "quiet", "suppress non-error output")
	cfg.Flags.BoolVar(&cfg.NoColor, 0, "no-color", "disable ANSI color")
	cfg.Flags.BoolVar(&cfg.JSONL, 0, "jsonl", "emit machine output as JSON Lines")
	cfg.Command = &ff.Command{
		Name:      "agentic-dev-harness",
		Usage:     "agentic-dev-harness [global flags] <SUBCOMMAND> ...",
		ShortHelp: "TODO: describe agentic-dev-harness here",
		Flags:     cfg.Flags,
	}
	return &cfg
}

// EmitJSONL writes v as one compact JSON object on its own line to stdout — a
// single JSON Lines record (SPEC §8). A command calls it once for a single result
// or once per record for a list, so an agent parses every command's output the
// same way. Under --quiet stdout is io.Discard, so the record is suppressed like
// any other stdout.
func (c *Config) EmitJSONL(v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("emit jsonl: %w", err)
	}
	_, _ = fmt.Fprintln(c.Stdout, string(data))
	return nil
}

// ConfigGetenv returns an environment accessor that honors --config: when set, it
// overrides ADH_CONFIG so config.Load reads the chosen file; other keys pass
// through to the injected Getenv. The approval phrase is still never sourced from
// the environment (§5.2) — ADH_APPROVAL_PHRASE stays ignored downstream.
func (c *Config) ConfigGetenv() func(string) string {
	if c.ConfigPath == "" {
		return c.Getenv
	}
	return func(key string) string {
		if key == "ADH_CONFIG" {
			return c.ConfigPath
		}
		return c.Getenv(key)
	}
}
