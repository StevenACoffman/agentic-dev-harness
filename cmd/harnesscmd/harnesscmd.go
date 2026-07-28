// Package harnesscmd implements the "harness" CLI command, the self-optimization
// surface (SPEC-ADDITIONS §18.2, §11) with real nested subcommands: `eval` scores
// a guiding artifact (rubric floor + judge-only dimensions), `gate` runs the
// comparative validation ratchet on candidate/current/best scores, and `hash`
// reports an artifact's content identity (sha256[:16]).
package harnesscmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/agentic-dev-harness/cmd/root"
	gatelib "github.com/StevenACoffman/agentic-dev-harness/internal/gate"
	"github.com/StevenACoffman/agentic-dev-harness/internal/harness"
	"github.com/StevenACoffman/agentic-dev-harness/internal/identity"
	"github.com/StevenACoffman/agentic-dev-harness/internal/judge"
)

// Config holds the configuration for the harness command and its subcommands.
type Config struct {
	*root.Config
	// eval flags
	Checks string
	Output string
	Min    float64
	// gate flags
	Candidate  string
	Current    string
	Best       string
	BestStep   string
	GlobalStep string
	Flags      *ff.FlagSet
	Command    *ff.Command
}

// New creates and registers the harness command (with eval/gate/hash subcommands)
// on the given parent config.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("harness").SetParent(parent.Flags)
	cfg.Command = &ff.Command{
		Name:      "harness",
		Usage:     "agentic-dev-harness harness <eval|gate|hash> [flags] [artifact]",
		ShortHelp: "score, gate, and hash the harness artifact",
		LongHelp: "Self-optimization surface (SPEC-ADDITIONS §18.2): `eval` scores an " +
			"artifact (floor + judge dimensions), `gate` runs the comparative ratchet, " +
			"`hash` reports content identity.",
		Flags: cfg.Flags,
	}
	cfg.addEval()
	cfg.addGate()
	cfg.addHash()
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

func (cfg *Config) addEval() {
	fs := ff.NewFlagSet("eval").SetParent(cfg.Flags)
	fs.StringVar(&cfg.Checks, 'c', "checks", "", "JSON rule checks for a behavioral pass")
	fs.StringVar(
		&cfg.Output,
		'o',
		"output",
		"",
		"output to judge against --checks (default: stdin)",
	)
	fs.Float64Var(&cfg.Min, 'm', "min", 0,
		"exit non-zero when the deterministic score is below this floor (0 = no floor)")
	cfg.Command.Subcommands = append(cfg.Command.Subcommands, &ff.Command{
		Name:      "eval",
		Usage:     "agentic-dev-harness harness eval [--min N] [--checks c.json] [--output o.txt] <artifact>",
		ShortHelp: "score a harness guiding artifact",
		LongHelp:  "Score a harness guiding artifact: a deterministic floor plus judge-only dimensions (SPEC-ADDITIONS §18.2).",
		Flags:     fs,
		Exec:      func(_ context.Context, args []string) error { return cfg.eval(args) },
	})
}

func (cfg *Config) addGate() {
	fs := ff.NewFlagSet("gate").SetParent(cfg.Flags)
	fs.StringVar(&cfg.Candidate, 'c', "candidate", "", "candidate score (required)")
	fs.StringVar(&cfg.Current, 'u', "current", "", "current score (required)")
	fs.StringVar(&cfg.Best, 'b', "best", "", "best-so-far score (defaults to current)")
	fs.StringVar(&cfg.BestStep, 0, "best-step", "0", "step at which best was set")
	fs.StringVar(&cfg.GlobalStep, 0, "global-step", "0", "current step")
	cfg.Command.Subcommands = append(cfg.Command.Subcommands, &ff.Command{
		Name:      "gate",
		Usage:     "agentic-dev-harness harness gate --candidate N --current N [--best N]",
		ShortHelp: "comparative validation ratchet (keep/revert a candidate score)",
		LongHelp: `Decide keep or revert for a candidate score using the self-optimization
ratchet (SPEC-ADDITIONS §18.2). Comparison is strict ">": a candidate is accepted
only if it beats the current score, and becomes the new best only if it also beats
the best. Ties reject. Exit code is 0 on any accept, 1 on reject.`,
		Flags: fs,
		Exec:  func(_ context.Context, _ []string) error { return cfg.gate() },
	})
}

func (cfg *Config) addHash() {
	fs := ff.NewFlagSet("hash").SetParent(cfg.Flags)
	cfg.Command.Subcommands = append(cfg.Command.Subcommands, &ff.Command{
		Name:      "hash",
		Usage:     "agentic-dev-harness harness hash <artifact>",
		ShortHelp: "report an artifact's content identity (sha256[:16])",
		LongHelp:  "Print sha256[:16] of an artifact; an unchanged hash is a no-op edit that skips re-evaluation (SPEC-ADDITIONS §18.2).",
		Flags:     fs,
		Exec:      func(_ context.Context, args []string) error { return cfg.hash(args) },
	})
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
	if err := cfg.render(&report); err != nil {
		return err
	}
	// Render first so the operator always sees the score, then gate: a score below
	// the --min floor exits non-zero for CI (SPEC-ADDITIONS §18.2).
	if !report.MeetsFloor(cfg.Min) {
		_, _ = fmt.Fprintf(cfg.Stderr,
			"harness: det score %.1f is below the --min floor %.1f\n",
			report.Rubric.DetScore, cfg.Min)
		return root.ExitError(1)
	}
	return nil
}

// gate runs the comparative validation ratchet (SPEC-ADDITIONS §18.2): strict ">"
// at both comparisons. Exit 0 on any accept, 1 on reject.
func (cfg *Config) gate() error {
	if cfg.Candidate == "" || cfg.Current == "" {
		return errors.New("harness: gate requires --candidate and --current")
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
	if err := cfg.renderGate(res); err != nil {
		return err
	}
	if res.Action == gatelib.Reject {
		return root.ExitError(1)
	}
	return nil
}

func (cfg *Config) hash(args []string) error {
	if len(args) == 0 {
		return errors.New("harness: hash requires an artifact path")
	}
	data, err := os.ReadFile(args[0])
	if err != nil {
		return fmt.Errorf("harness: read artifact: %w", err)
	}
	_, _ = fmt.Fprintln(cfg.Stdout, identity.Hash(string(data)))
	return nil
}

// behavioral loads the optional rule checks and the output under test. It returns
// nil checks (and skips reading output) when --checks is unset.
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
	if cfg.JSONL {
		if err := cfg.EmitJSONL(report); err != nil {
			return fmt.Errorf("harness: %w", err)
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

func (cfg *Config) renderGate(res gatelib.Result) error {
	if cfg.JSONL {
		if err := cfg.EmitJSONL(res); err != nil {
			return fmt.Errorf("harness: %w", err)
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
		return 0, fmt.Errorf("harness: --%s: invalid score %q", name, value)
	}
	return score, nil
}

func parseStep(name, value string) (int, error) {
	step, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("harness: --%s: invalid integer %q", name, value)
	}
	return step, nil
}
