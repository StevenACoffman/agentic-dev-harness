// Package root defines the root configuration for the CLI.
package root

import (
	"fmt"
	"io"
	"log/slog"

	"github.com/peterbourgon/ff/v4"
)

// ExitError is returned by commands that want a specific non-zero exit code
// without printing an additional error message. run() in main.go checks for
// ExitError with errors.As and calls os.Exit(int(e)) directly, bypassing the
// default "error: ..." printer.
type ExitError int

// DryRunUnsupportedError is the error a command returns when --dry-run is set but the
// command does not honor it — only approve, reject, and close do — so a global
// --dry-run never silently mutates state on a command that would otherwise ignore
// the flag. Its string value is the command name, so the message reads
// "<cmd>: --dry-run not supported ...". Like ExitError, it is a defined type, so
// callers return it by conversion (root.DryRunUnsupportedError("run")) rather than a
// cross-package call; the dispatcher envelopes it like any other command error.
type DryRunUnsupportedError string

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
	Yes        bool   // --yes: pre-answer non-gate prompts; never satisfies a safety gate (§5.2)
	DryRun     bool   // --dry-run: preview a mutation without persisting (approve/reject/close)
	// Log is the diagnostic stream on stderr, separate from the stdout data plane
	// (SPEC §8). New seeds a default; Run rebuilds it once the flags are parsed, so
	// --verbose/--quiet set the level and --jsonl selects the JSON handler.
	Log     *slog.Logger
	Flags   *ff.FlagSet
	Command *ff.Command
}

// Error reports the exit status. ExitError satisfies the error interface so a
// command can request a specific exit code without printing an "error: ..." line.
func (e ExitError) Error() string { return fmt.Sprintf("exit status %d", int(e)) }

// Error renders the refusal in the standard "<cmd>: <reason>" form (SPEC §8).
func (c DryRunUnsupportedError) Error() string {
	return string(c) + ": --dry-run not supported (honored by approve, reject, close)"
}

// New returns a new root Config with the given I/O writers and environment
// accessor. getenv is injected (not os.Getenv) so config precedence is testable.
func New(getenv func(string) string, stdin io.Reader, stdout, stderr io.Writer) *Config {
	var cfg Config
	cfg.Stdin = stdin
	cfg.Stdout = stdout
	cfg.Stderr = stderr
	cfg.Getenv = getenv
	// A safe default logger so cfg.Log is never nil (a command exercised without
	// Run still logs); Run rebuilds it from the parsed flags.
	cfg.Log = NewLogger(stderr, false, slog.LevelWarn)
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
	cfg.Flags.BoolVar(
		&cfg.Yes,
		0,
		"yes",
		"pre-answer non-gate prompts; never satisfies a safety gate",
	)
	cfg.Flags.BoolVar(&cfg.DryRun, 0, "dry-run",
		"preview a mutation without persisting (honored by approve, reject, close)")
	cfg.Command = &ff.Command{
		Name:      "agentic-dev-harness",
		Usage:     "agentic-dev-harness [global flags] <SUBCOMMAND> ...",
		ShortHelp: "drive a change through the five-stage arc loop",
		LongHelp: "Agentic Development Harness (adh): a deterministic CLI that drives a " +
			"change through the five-stage arc loop — strategy, execution, critic, " +
			"evaluation, ops — one gated step at a time (SPEC §1). The model turns are " +
			"relayed to a driving agent (Claude or Gemini running adh as a skill), so adh " +
			"itself needs no API key; --jsonl makes every outcome machine-readable and the " +
			"exit code the primary signal. `docs` generates the full man-page reference.",
		Flags: cfg.Flags,
	}
	return &cfg
}

// LogLevel resolves the diagnostic log level from the verbosity flags: --quiet
// shows only errors ("suppress non-error output", SPEC §2), --verbose unlocks
// debug, and the default shows warnings and errors — the incidental notices.
// quiet wins when both are set.
func LogLevel(verbose, quiet bool) slog.Level {
	switch {
	case quiet:
		return slog.LevelError
	case verbose:
		return slog.LevelDebug
	default:
		return slog.LevelWarn
	}
}

// NewLogger builds the stderr diagnostic logger at the given level: a JSON handler
// under --jsonl, so the log stream is structured like the stdout data plane, or a
// text handler for humans.
func NewLogger(w io.Writer, jsonl bool, level slog.Level) *slog.Logger {
	opts := &slog.HandlerOptions{Level: level}
	if jsonl {
		return slog.New(slog.NewJSONHandler(w, opts))
	}
	return slog.New(slog.NewTextHandler(w, opts))
}

// ConfigGetenv returns an environment accessor that bridges the config flags into
// the injected environment: --config overrides ADH_CONFIG (the chosen file) and
// --profile overrides ADH_PROFILE (the config.Load profile layer, SPEC §3 tier 3);
// other keys pass through to the injected Getenv. The approval phrase is still
// never sourced from the environment (§5.2) — ADH_APPROVAL_PHRASE stays ignored.
func (c *Config) ConfigGetenv() func(string) string {
	if c.ConfigPath == "" && c.Profile == "" {
		return c.Getenv
	}
	// The flags win over their environment variables; an unset flag falls through
	// to the real environment, so --profile and ADH_PROFILE both resolve here.
	return func(key string) string {
		switch {
		case key == "ADH_CONFIG" && c.ConfigPath != "":
			return c.ConfigPath
		case key == "ADH_PROFILE" && c.Profile != "":
			return c.Profile
		default:
			return c.Getenv(key)
		}
	}
}
