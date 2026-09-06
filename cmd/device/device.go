// Package device implements the "device" CLI command: on-device validation
// (SPEC §4). It uses a mock backend until the adb adapter lands; exit 7 on a
// failed validation.
package device

import (
	"context"
	"fmt"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/agentic-dev-harness/cmd/root"
	devicelib "github.com/StevenACoffman/agentic-dev-harness/internal/device"
)

// Config holds the configuration for the device command.
type Config struct {
	*root.Config
	Flags   *ff.FlagSet
	Command *ff.Command
}

// New creates and registers the device command with the given parent config.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("device").SetParent(parent.Flags)
	cfg.Command = &ff.Command{
		Name:      "device",
		Usage:     "agentic-dev-harness device validate",
		ShortHelp: "run on-device validation",
		LongHelp:  "Validate behavior on a device. Uses a mock backend until the adb adapter lands.",
		Flags:     cfg.Flags,
		Exec:      cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

// deviceVerbs is what exec dispatches, named once so the refusals below and the usage
// line cannot disagree about it.
func deviceVerbs() []string { return []string{"validate"} }

func (cfg *Config) exec(ctx context.Context, args []string) error {
	// One prose refusal covered both the missing verb and the wrong one, which cost two
	// things beyond the wording: the contract walk reads a command's verbs off
	// MissingVerbError, so `device validate` was never enumerated and its output never
	// checked; and a mistyped verb got the same message as no verb at all.
	if len(args) == 0 {
		return root.MissingVerbError{Scope: "device", Verbs: deviceVerbs()}
	}
	if args[0] != "validate" {
		return root.UnknownVerbError{Scope: "device", Got: args[0], Verbs: deviceVerbs()}
	}
	report, err := devicelib.Mock{Healthy: true}.Validate(ctx)
	if err != nil {
		return fmt.Errorf("device: %w", err)
	}
	if cfg.JSONL {
		// `ok` is the verdict a caller branches on and `detail` is the evidence for it.
		// Printing only the detail left a machine consumer parsing a sentence for a
		// boolean it could not otherwise recover, since a failed validation exits 7 but
		// the envelope is still the outcome.
		if err := cfg.EmitOK(map[string]any{"ok": report.OK, "detail": report.Detail}); err != nil {
			return fmt.Errorf("device: %w", err)
		}
		if !report.OK {
			return root.ExitError(7)
		}
		return nil
	}
	_, _ = fmt.Fprintln(cfg.Stdout, report.Detail)
	if !report.OK {
		return root.ExitError(7)
	}
	return nil
}
