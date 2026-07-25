// Package gate implements the "gate" CLI command: the comparative validation
// ratchet (SPEC-ADDITIONS §18.2). Given a candidate score and the current/best
// scores it decides accept_new_best, accept, or reject using strict ">". Phase 9
// regroups this under "harness gate"; it is top-level while the surface is built.
package gate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/agentic-dev-harness/cmd/root"
	gatelib "github.com/StevenACoffman/agentic-dev-harness/internal/gate"
)

// Config holds the configuration for the gate command.
type Config struct {
	*root.Config
	Candidate  string
	Current    string
	Best       string
	BestStep   string
	GlobalStep string
	JSON       bool
	Flags      *ff.FlagSet
	Command    *ff.Command
}

// New creates and registers the gate command with the given parent config.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("gate").SetParent(parent.Flags)
	cfg.Flags.StringVar(&cfg.Candidate, 'c', "candidate", "", "candidate score (required)")
	cfg.Flags.StringVar(&cfg.Current, 'u', "current", "", "current score (required)")
	cfg.Flags.StringVar(&cfg.Best, 'b', "best", "", "best-so-far score (defaults to current)")
	cfg.Flags.StringVar(&cfg.BestStep, 0, "best-step", "0", "step at which best was set")
	cfg.Flags.StringVar(&cfg.GlobalStep, 0, "global-step", "0", "current step")
	cfg.Flags.BoolVar(&cfg.JSON, 0, "json", "emit the decision as JSON")
	cfg.Command = &ff.Command{
		Name:      "gate",
		Usage:     "agentic-dev-harness gate --candidate N --current N [--best N]",
		ShortHelp: "comparative validation ratchet (keep/revert a candidate score)",
		LongHelp: `Decide keep or revert for a candidate score using the harness
self-optimization ratchet (SPEC-ADDITIONS §18.2), a port of SkillOpt's
validation gate. Comparison is strict ">": a candidate is accepted only if it
beats the current score, and becomes the new best only if it also beats the best
score. Ties reject and do not promote.

Scores are the already-projected comparison metric (the held-out score). Outcomes:

  accept_new_best   candidate > current AND candidate > best   -> keep, new best
  accept            candidate > current only                   -> keep, best unchanged
  reject            candidate <= current                       -> revert

Exit code is 0 on any accept, 1 on reject, so a script can branch on it.`,
		Flags: cfg.Flags,
		Exec:  cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

func (cfg *Config) exec(_ context.Context, _ []string) error {
	if cfg.Candidate == "" || cfg.Current == "" {
		return errors.New("gate: --candidate and --current are required")
	}
	candidate, err := parseScore("candidate", cfg.Candidate)
	if err != nil {
		return err
	}
	current, err := parseScore("current", cfg.Current)
	if err != nil {
		return err
	}
	best := current
	if cfg.Best != "" {
		if best, err = parseScore("best", cfg.Best); err != nil {
			return err
		}
	}
	bestStep, err := parseStep("best-step", cfg.BestStep)
	if err != nil {
		return err
	}
	globalStep, err := parseStep("global-step", cfg.GlobalStep)
	if err != nil {
		return err
	}

	res := gatelib.Evaluate(candidate, current, best, bestStep, globalStep)
	if err := cfg.render(res); err != nil {
		return err
	}
	if res.Action == gatelib.Reject {
		return root.ExitError(1)
	}
	return nil
}

func (cfg *Config) render(res gatelib.Result) error {
	if cfg.JSON {
		enc := json.NewEncoder(cfg.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(res); err != nil {
			return fmt.Errorf("gate: encode json: %w", err)
		}
		return nil
	}
	_, _ = fmt.Fprintf(cfg.Stdout, "%s\n", res.Action)
	_, _ = fmt.Fprintf(cfg.Stdout, "  current -> %.1f\n  best    -> %.1f (step %d)\n",
		res.CurrentScore, res.BestScore, res.BestStep)
	return nil
}

func parseScore(name, value string) (float64, error) {
	score, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, fmt.Errorf("gate: --%s: invalid score %q", name, value)
	}
	return score, nil
}

func parseStep(name, value string) (int, error) {
	step, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("gate: --%s: invalid integer %q", name, value)
	}
	return step, nil
}
