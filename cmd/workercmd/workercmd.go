// Package workercmd implements the "worker" CLI command: show the adoption
// epoch or requalify it after a worker change (SPEC §14).
package workercmd

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/agentic-dev-harness/cmd/root"
	"github.com/StevenACoffman/agentic-dev-harness/internal/identity"
	workerlib "github.com/StevenACoffman/agentic-dev-harness/internal/worker"
)

const stateFile = ".adh/worker.json"

// Config holds the configuration for the worker command.
type Config struct {
	*root.Config
	Flags   *ff.FlagSet
	Command *ff.Command
}

// New creates and registers the worker command with the given parent config.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("worker").SetParent(parent.Flags)
	cfg.Command = &ff.Command{
		Name:      "worker",
		Usage:     "agentic-dev-harness worker <show|requalify>",
		ShortHelp: "show or requalify the worker epoch",
		LongHelp:  "Show the current adoption epoch or requalify it after a worker change (SPEC §14).",
		Flags:     cfg.Flags,
		Exec:      cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

func (cfg *Config) exec(_ context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("worker: expected a verb: show or requalify")
	}
	switch args[0] {
	case "show":
		return cfg.show()
	case "requalify":
		return cfg.requalify()
	default:
		return fmt.Errorf("worker: unknown verb %q; want show or requalify", args[0])
	}
}

func (cfg *Config) show() error {
	epoch, err := workerlib.Load(stateFile)
	if err != nil {
		return fmt.Errorf("worker: %w", err)
	}
	if epoch.ID == "" {
		_, _ = fmt.Fprintln(cfg.Stdout, "no epoch recorded; run 'worker requalify'")
		return nil
	}
	_, _ = fmt.Fprintf(cfg.Stdout, "epoch: %s\n", epoch.ID)
	for _, role := range sortedRoles(epoch.Models) {
		_, _ = fmt.Fprintf(cfg.Stdout, "  %s -> %s\n", role, epoch.Models[role])
	}
	return nil
}

func (cfg *Config) requalify() error {
	models := baselineModels()
	epoch := workerlib.Epoch{ID: identity.Hash(canonical(models)), Models: models}
	if err := workerlib.Save(stateFile, epoch); err != nil {
		return fmt.Errorf("worker: %w", err)
	}
	_, _ = fmt.Fprintf(cfg.Stdout, "requalified: epoch %s (%d roles)\n", epoch.ID, len(models))
	return nil
}

// baselineModels is the per-role model binding recorded at requalification. It
// is a placeholder until model config lands; the epoch mechanism is real.
func baselineModels() map[string]string {
	return map[string]string{
		"strategy":   "reasoning",
		"execution":  "fast",
		"critic":     "reasoning",
		"evaluation": "reasoning",
		"ops":        "fast",
	}
}

func canonical(models map[string]string) string {
	var b strings.Builder
	for _, role := range sortedRoles(models) {
		b.WriteString(role + "=" + models[role] + ";")
	}
	return b.String()
}

func sortedRoles(models map[string]string) []string {
	roles := make([]string, 0, len(models))
	for role := range models {
		roles = append(roles, role)
	}
	sort.Strings(roles)
	return roles
}
