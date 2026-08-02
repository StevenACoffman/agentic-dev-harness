// Package kpicmd implements the "kpi" CLI command: measure each context unit's and
// declared tool's KPIs against how they actually behaved — the failure-record log for
// units, the tool-run log for tools — and report the degradation proposals a breach
// earned (SPEC-ADDITIONS §16/§18). It proposes, never adopts — the change is a human's,
// and a proposal fires only after the breach replicates across independent strata (§18.2).
package kpicmd

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/agentic-dev-harness/cmd/root"
	"github.com/StevenACoffman/agentic-dev-harness/internal/contextstore"
	"github.com/StevenACoffman/agentic-dev-harness/internal/failures"
	kpilib "github.com/StevenACoffman/agentic-dev-harness/internal/kpi"
	"github.com/StevenACoffman/agentic-dev-harness/internal/toolreg"
	"github.com/StevenACoffman/agentic-dev-harness/internal/toolrun"
)

// Config holds the configuration for the kpi command.
type Config struct {
	*root.Config
	Flags   *ff.FlagSet
	Command *ff.Command
}

// New creates and registers the kpi command with the given parent config.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("kpi").SetParent(parent.Flags)
	cfg.Command = &ff.Command{
		Name:      "kpi",
		Usage:     "agentic-dev-harness kpi",
		ShortHelp: "propose config changes from degraded unit and tool KPIs",
		LongHelp: "Measure each context unit's and declared tool's KPIs against how they " +
			"behaved — the failure-record log for units, the tool-run log for tools — and " +
			"report the degradation proposals a breach earned (SPEC-ADDITIONS §16/§18). " +
			"Advisory: a proposal is never auto-adopted, and fires only after the breach " +
			"replicates across independent time strata (§18.2).",
		Flags: cfg.Flags,
		Exec:  cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

func (cfg *Config) exec(_ context.Context, _ []string) error {
	units, err := contextstore.Load(cfg.storeDir())
	if err != nil {
		return fmt.Errorf("kpi: %w", err)
	}
	records, err := failures.LoadRecords(filepath.Join(cfg.repoDir(), failures.RecordsFile))
	if err != nil {
		return fmt.Errorf("kpi: %w", err)
	}
	reg, err := toolreg.LoadRepo(cfg.repoDir())
	if err != nil {
		return fmt.Errorf("kpi: %w", err)
	}
	runs, err := toolrun.Load(filepath.Join(cfg.repoDir(), toolrun.RunFile))
	if err != nil {
		return fmt.Errorf("kpi: %w", err)
	}
	subjects := append(
		kpilib.ObserveUnits(units, records),
		kpilib.ObserveTools(reg.Tools, runs)...,
	)
	proposals := kpilib.Propose(subjects, kpilib.MinStrata)
	return cfg.report(proposals)
}

// report emits the proposals: under --jsonl one OK outcome carrying the list, else a
// human line per proposal (or a clean-bill line). It is advisory — the command always
// exits 0; a proposal is the operator's to act on.
func (cfg *Config) report(proposals []kpilib.Proposal) error {
	if cfg.JSONL {
		if err := cfg.EmitOK(map[string]any{"proposals": proposals}); err != nil {
			return fmt.Errorf("kpi: %w", err)
		}
		return nil
	}
	if len(proposals) == 0 {
		_, _ = fmt.Fprintln(cfg.Stdout, "no KPI degradations proposed")
		return nil
	}
	for i := range proposals {
		p := proposals[i]
		_, _ = fmt.Fprintf(cfg.Stdout,
			"propose: %s %s degraded on %s — observed %g vs threshold %g across %d strata\n",
			p.Kind, p.Subject, p.Metric, p.Observed, p.Threshold, p.Strata)
	}
	return nil
}

// repoDir is the repository root — the --repo global, or the current directory.
func (cfg *Config) repoDir() string {
	if cfg.Repo != "" {
		return cfg.Repo
	}
	return "."
}

// storeDir is the context store under the repo root.
func (cfg *Config) storeDir() string {
	return filepath.Join(cfg.repoDir(), contextstore.DefaultStoreDir)
}
