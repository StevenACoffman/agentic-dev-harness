// Package root defines the root configuration for the CLI.
package root

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
)

// Outcome status values (SPEC §8): the class of a command's result, the field an
// agent switches on under --jsonl.
const (
	StatusOK      = "ok"      // the command succeeded
	StatusBlocked = "blocked" // stopped at a human gate or an environment gap, not a failure
	StatusError   = "error"   // the command failed
)

// Outcome reason tokens: a stable machine string for a blocked/error outcome, so
// an agent branches on the token instead of matching prose. Free-form reasons
// (e.g. a confirmed finding's kind) may also appear; these are the shared ones.
const (
	ReasonAtOps      = "at_ops"     // arc reached the ops ship gate (step)
	ReasonUngrounded = "ungrounded" // critic routing gap, exit 12 (step)
	ReasonGate       = "gate"       // pending human approval, exit 4 (approve)
	ReasonProof      = "proof"      // proof verification failed, exit 8 (proof/close)
	ReasonRequalify  = "requalify"  // worker changed; requalify before running, exit 9 (§14)
	ReasonFailed     = "failed"     // arc failed evaluation past its rework budget (run, §4.1)
)

// Exit codes surfaced in an error outcome's Code for a generic returned error
// (SPEC §7): usage/validation vs. everything else. Domain gates set their own
// (4, 5–8, 12) at the call site.
const (
	codeGeneric = 1
	codeUsage   = 2
)

// ExitError is returned by commands that want a specific non-zero exit code
// without printing an additional error message. run() in main.go checks for
// ExitError with errors.As and calls os.Exit(int(e)) directly, bypassing the
// default "error: ..." printer.
type ExitError int

// Outcome is the structured result an agent reads under --jsonl (SPEC §8): a
// single JSON object per command outcome. Status is the class; Code is the process
// exit code (0 for ok); Reason is a stable machine token for a blocked/error
// outcome; Message is the human detail; Data is the command payload on success.
type Outcome struct {
	Status  string `json:"status"`
	Code    int    `json:"code"`
	Reason  string `json:"reason,omitempty"`
	Message string `json:"message,omitempty"`
	Data    any    `json:"data,omitempty"`
}

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

// EmitOK writes a success outcome carrying the command payload (SPEC §8).
func (c *Config) EmitOK(data any) error {
	return c.EmitJSONL(Outcome{Status: StatusOK, Code: 0, Data: data})
}

// EmitBlocked writes a blocked outcome — a human gate or environment gap, not a
// failure — with its exit code and a machine reason token.
func (c *Config) EmitBlocked(code int, reason, message string) error {
	return c.EmitJSONL(Outcome{Status: StatusBlocked, Code: code, Reason: reason, Message: message})
}

// EmitError writes an error outcome with its exit code and a machine reason token.
func (c *Config) EmitError(code int, reason, message string) error {
	return c.EmitJSONL(Outcome{Status: StatusError, Code: code, Reason: reason, Message: message})
}

// CodeForError maps a returned error to the process exit code an error outcome
// reports when the call site set none: a validation error is a usage error (2),
// everything else is generic (1). Domain gates (4, 5–8, 12) set their own code.
func CodeForError(err error) int {
	if adh.ErrorCode(err) == adh.EINVALID {
		return codeUsage
	}
	return codeGeneric
}

// ReasonForError is the machine reason token for a returned error: its domain
// error code (e.g. "not_found", "invalid", "conflict"), or "internal" for an
// untyped error. It lets an agent branch on the failure class under --jsonl.
func ReasonForError(err error) string {
	return adh.ErrorCode(err)
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
