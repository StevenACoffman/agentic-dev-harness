// Package harnesscmd implements the "harness" CLI command: score a harness
// guiding artifact with the rubric's deterministic floor plus judge-only
// dimensions, optionally folding in a behavioral rule-judge pass
// (SPEC-ADDITIONS §18.2, §11). Verb: eval. gate/hash are reserved for the
// Phase-9 regroup.
package harnesscmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/agentic-dev-harness/cmd/root"
	"github.com/StevenACoffman/agentic-dev-harness/internal/harness"
	"github.com/StevenACoffman/agentic-dev-harness/internal/judge"
)

// Config holds the configuration for the harness command.
type Config struct {
	*root.Config
	Checks  string
	Output  string
	JSON    bool
	Flags   *ff.FlagSet
	Command *ff.Command
}

// New creates and registers the harness command with the given parent config.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("harness").SetParent(parent.Flags)
	cfg.Flags.StringVar(&cfg.Checks, 'c', "checks", "", "JSON rule checks for a behavioral pass")
	cfg.Flags.StringVar(
		&cfg.Output,
		'o',
		"output",
		"",
		"output to judge against --checks (default: stdin)",
	)
	cfg.Flags.BoolVar(&cfg.JSON, 0, "json", "emit the report as JSON")
	cfg.Command = &ff.Command{
		Name:      "harness",
		Usage:     "agentic-dev-harness harness eval <artifact> [--checks c.json] [--output o.txt]",
		ShortHelp: "score and gate the harness artifact",
		LongHelp:  "Score a harness guiding artifact: a deterministic floor plus judge-only dimensions (SPEC-ADDITIONS §18.2).",
		Flags:     cfg.Flags,
		Exec:      cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

func (cfg *Config) exec(_ context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("harness: expected a verb: eval")
	}
	switch args[0] {
	case "eval":
		return cfg.eval(args[1:])
	default:
		return fmt.Errorf("harness: unknown verb %q; want eval", args[0])
	}
}

func (cfg *Config) eval(args []string) error {
	if len(args) == 0 {
		return errors.New("harness: eval requires an artifact path")
	}
	doc, err := os.ReadFile(args[0])
	if err != nil {
		return fmt.Errorf("harness: read artifact: %w", err)
	}
	checks, output, err := cfg.behavioral()
	if err != nil {
		return err
	}
	report, err := harness.Eval(string(doc), output, checks)
	if err != nil {
		return fmt.Errorf("harness: %w", err)
	}
	return cfg.render(&report)
}

// behavioral loads the optional rule checks and the output under test. It
// returns nil checks (and skips reading output) when --checks is unset.
func (cfg *Config) behavioral() ([]judge.Check, string, error) {
	if cfg.Checks == "" {
		return nil, "", nil
	}
	data, err := os.ReadFile(cfg.Checks)
	if err != nil {
		return nil, "", fmt.Errorf("harness: read checks: %w", err)
	}
	var checks []judge.Check
	if err := json.Unmarshal(data, &checks); err != nil {
		return nil, "", fmt.Errorf("harness: parse checks: %w", err)
	}
	output, err := cfg.readOutput()
	if err != nil {
		return nil, "", err
	}
	return checks, output, nil
}

func (cfg *Config) readOutput() (string, error) {
	if cfg.Output == "" {
		data, err := io.ReadAll(cfg.Stdin)
		if err != nil {
			return "", fmt.Errorf("harness: read stdin: %w", err)
		}
		return string(data), nil
	}
	data, err := os.ReadFile(cfg.Output)
	if err != nil {
		return "", fmt.Errorf("harness: read output: %w", err)
	}
	return string(data), nil
}

func (cfg *Config) render(report *harness.EvalReport) error {
	if cfg.JSON {
		enc := json.NewEncoder(cfg.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			return fmt.Errorf("harness: encode json: %w", err)
		}
		return nil
	}
	_, _ = fmt.Fprintf(cfg.Stdout, "det score: %.1f/100\n", report.Rubric.DetScore)
	for i := range report.Rubric.Dims {
		dim := &report.Rubric.Dims[i]
		_, _ = fmt.Fprintf(cfg.Stdout, "  %-24s w%-3d %.2f  %s\n",
			dim.Key, dim.Weight, dim.Deterministic, dim.Reason)
	}
	_, _ = fmt.Fprintf(cfg.Stdout, "diagnosis: %s\n", report.Diagnosis)
	if report.Behavioral != nil {
		_, _ = fmt.Fprintf(cfg.Stdout, "behavioral: hard %.0f  soft %.2f\n",
			report.Behavioral.Hard, report.Behavioral.Soft)
	}
	return nil
}
