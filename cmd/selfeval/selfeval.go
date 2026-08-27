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
	steps, err := cfg.stepClass()
	if err != nil {
		return err
	}
	notes, err := failures.Load(failures.RegistryFile)
	if err != nil {
		return fmt.Errorf("selfeval: %w", err)
	}
	classes := lesson.Distill(notes)

	if cfg.JSONL {
		// Gathered above the branch so both renderings answer from one computation:
		// a JSON report that disagreed with the human one would be worse than either.
		return cfg.emitJSON(summary, steps, classes)
	}
	cfg.writeHuman(summary, steps, classes)
	return nil
}

// emitJSON reports the self-evaluation as one envelope.
//
// It exists because --jsonl used to print the human report, handing prose to a caller
// that asked for JSON. The failure taxonomy travels as counts per class rather than as
// the rendered lines, since a consumer wants the number and can render its own label.
func (cfg *Config) emitJSON(
	summary metrics.Summary, steps metrics.StepClass, classes []lesson.Lesson,
) error {
	taxonomy := make(map[string]int, len(classes))
	for _, class := range classes {
		taxonomy[class.Class] = len(class.Instances)
	}
	if err := cfg.EmitOK(map[string]any{
		"arcs": summary.Arcs, "accepted": summary.Accepted,
		"attention_per_accept": summary.AttentionPerAccept,
		"compute_tokens":       summary.ComputeTokens,
		"steps_deterministic":  steps.Deterministic,
		"steps_model":          steps.Model,
		"deterministic_ratio":  steps.Ratio(),
		"failure_taxonomy":     taxonomy,
	}); err != nil {
		return fmt.Errorf("selfeval: %w", err)
	}
	return nil
}

// writeHuman renders the same report for a person.
func (cfg *Config) writeHuman(
	summary metrics.Summary, steps metrics.StepClass, classes []lesson.Lesson,
) {
	_, _ = fmt.Fprintf(cfg.Stdout,
		"health:\n  arcs %d, accepted %d, attention/accept %.1f min, compute %d tokens\n",
		summary.Arcs, summary.Accepted, summary.AttentionPerAccept, summary.ComputeTokens)

	// The effectiveness north-star (§16): the deterministic share of arc steps —
	// accretion (routing rules, checks, lessons) should trend it upward as fewer
	// steps need a relayed model turn. A coarse proxy classified from arc history.
	_, _ = fmt.Fprintf(cfg.Stdout,
		"  steps: %d deterministic / %d model (%.0f%% deterministic — coarse proxy)\n",
		steps.Deterministic, steps.Model, steps.Ratio()*100)

	if len(classes) == 0 {
		_, _ = fmt.Fprintln(cfg.Stdout, "failure taxonomy:\n  none recorded")
		return
	}
	_, _ = fmt.Fprintln(cfg.Stdout, "failure taxonomy:")
	for _, class := range classes {
		_, _ = fmt.Fprintf(cfg.Stdout, "  %s\t(%d instance(s))\n",
			class.Class, len(class.Instances))
	}
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
