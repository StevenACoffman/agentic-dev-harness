// Package selfeval implements the "selfeval" CLI command (SPEC §2.5): a periodic
// self-evaluation that reports effectiveness health (§16) and the failure taxonomy
// (§4.1, §11). It is a thin shell over metrics.Summarize and lesson.Distill;
// delta-vs-prior awaits a persisted health snapshot and is not reported yet.
package selfeval

import (
	"context"
	"fmt"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/agentic-dev-harness/cmd/root"
	"github.com/StevenACoffman/agentic-dev-harness/internal/failures"
	"github.com/StevenACoffman/agentic-dev-harness/internal/lesson"
	"github.com/StevenACoffman/agentic-dev-harness/internal/metrics"
	"github.com/StevenACoffman/agentic-dev-harness/internal/state"
)

// Config holds the configuration for the selfeval command.
type Config struct {
	*root.Config
	Flags   *ff.FlagSet
	Command *ff.Command
}

// New creates and registers the selfeval command with the given parent config.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("selfeval").SetParent(parent.Flags)
	cfg.Command = &ff.Command{
		Name:      "selfeval",
		Usage:     "agentic-dev-harness selfeval",
		ShortHelp: "report effectiveness health and the failure taxonomy",
		LongHelp:  "Periodic self-evaluation: effectiveness health (§16) and the failure taxonomy (§4.1, §11).",
		Flags:     cfg.Flags,
		Exec:      cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

func (cfg *Config) exec(_ context.Context, _ []string) error {
	records, err := metrics.Load(metrics.LedgerFile)
	if err != nil {
		return fmt.Errorf("selfeval: %w", err)
	}
	summary := metrics.Summarize(records)
	_, _ = fmt.Fprintf(cfg.Stdout,
		"health:\n  arcs %d, accepted %d, attention/accept %.1f min, compute %d tokens\n",
		summary.Arcs, summary.Accepted, summary.AttentionPerAccept, summary.ComputeTokens)

	// The effectiveness north-star (§16): the deterministic share of arc steps —
	// accretion (routing rules, checks, lessons) should trend it upward as fewer
	// steps need a relayed model turn. A coarse proxy classified from arc history.
	steps, err := cfg.stepClass()
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(cfg.Stdout,
		"  steps: %d deterministic / %d model (%.0f%% deterministic — coarse proxy)\n",
		steps.Deterministic, steps.Model, steps.Ratio()*100)

	notes, err := failures.Load(failures.RegistryFile)
	if err != nil {
		return fmt.Errorf("selfeval: %w", err)
	}
	if len(notes) == 0 {
		_, _ = fmt.Fprintln(cfg.Stdout, "failure taxonomy:\n  none recorded")
		return nil
	}
	_, _ = fmt.Fprintln(cfg.Stdout, "failure taxonomy:")
	for _, class := range lesson.Distill(notes) {
		_, _ = fmt.Fprintf(
			cfg.Stdout,
			"  %s\t(%d instance(s))\n",
			class.Class,
			len(class.Instances),
		)
	}
	return nil
}

// stepClass aggregates the deterministic-vs-model step classification across every
// arc in the store (§16) — the coarse effectiveness north-star.
func (cfg *Config) stepClass() (metrics.StepClass, error) {
	arcs, err := state.Default().List()
	if err != nil {
		return metrics.StepClass{}, fmt.Errorf("selfeval: %w", err)
	}
	var total metrics.StepClass
	for i := range arcs {
		total = total.Add(metrics.ClassifyHistory(arcs[i].History))
	}
	return total, nil
}
