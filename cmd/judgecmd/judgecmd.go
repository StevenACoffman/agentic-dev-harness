// Package judgecmd implements the "judge" CLI command: score an output against
// a JSON array of deterministic rule checks (SPEC-ADDITIONS §11). Exit 1 when
// hard is 0, so a script can branch on a failed behavioral check.
package judgecmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/agentic-dev-harness/cmd/root"
	judgelib "github.com/StevenACoffman/agentic-dev-harness/internal/judge"
)

// Config holds the configuration for the judge command.
type Config struct {
	*root.Config
	Checks  string
	Output  string
	Flags   *ff.FlagSet
	Command *ff.Command
}

// New creates and registers the judge command with the given parent config.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("judge").SetParent(parent.Flags)
	cfg.Flags.StringVar(
		&cfg.Checks,
		'c',
		"checks",
		"",
		"path to a JSON array of rule checks (required)",
	)
	cfg.Flags.StringVar(&cfg.Output, 'o', "output", "", "output file to judge (default: stdin)")
	cfg.Command = &ff.Command{
		Name:      "judge",
		Usage:     "agentic-dev-harness judge --checks checks.json [--output out.txt]",
		ShortHelp: "score an output against deterministic rule checks",
		LongHelp:  "Score an output against a JSON array of rule checks (SPEC-ADDITIONS §11): hard = 1 iff all pass, soft = passed/total.",
		Flags:     cfg.Flags,
		Exec:      cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

func (cfg *Config) exec(_ context.Context, _ []string) error {
	if cfg.Checks == "" {
		return errors.New("judge: --checks is required")
	}
	checks, err := loadChecks(cfg.Checks)
	if err != nil {
		return err
	}
	output, err := cfg.readOutput()
	if err != nil {
		return err
	}
	res, err := judgelib.Score(output, checks)
	if err != nil {
		return fmt.Errorf("judge: %w", err)
	}
	_, _ = fmt.Fprintf(cfg.Stdout, "hard: %.0f  soft: %.2f\n", res.Hard, res.Soft)
	for _, why := range res.Why {
		_, _ = fmt.Fprintf(cfg.Stdout, "  - %s\n", why)
	}
	if res.Hard == 0 {
		return root.ExitError(1)
	}
	return nil
}

func (cfg *Config) readOutput() (string, error) {
	if cfg.Output == "" {
		data, err := io.ReadAll(cfg.Stdin)
		if err != nil {
			return "", fmt.Errorf("judge: read stdin: %w", err)
		}
		return string(data), nil
	}
	data, err := os.ReadFile(cfg.Output)
	if err != nil {
		return "", fmt.Errorf("judge: read output: %w", err)
	}
	return string(data), nil
}

func loadChecks(path string) ([]judgelib.Check, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("judge: read checks: %w", err)
	}
	var checks []judgelib.Check
	if err := json.Unmarshal(data, &checks); err != nil {
		return nil, fmt.Errorf("judge: parse checks: %w", err)
	}
	return checks, nil
}
