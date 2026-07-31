// Package device implements the "device" CLI command: on-device validation
// (SPEC §4). It uses a mock backend until the adb adapter lands; exit 7 on a
// failed validation.
package device

import (
	"context"
	"errors"
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

func (cfg *Config) exec(ctx context.Context, args []string) error {
	if len(args) == 0 || args[0] != "validate" {
		return errors.New("device: expected 'validate'")
	}
	report, err := devicelib.Mock{Healthy: true}.Validate(ctx)
	if err != nil {
		return fmt.Errorf("device: %w", err)
	}
	_, _ = fmt.Fprintln(cfg.Stdout, report.Detail)
	if !report.OK {
		return root.ExitError(7)
	}
	return nil
}
