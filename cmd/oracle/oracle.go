// Package oracle implements the "oracle" CLI command: the differential oracle
// (diff), the invariant checks (invariants), and the planted-defect self-test
// (selftest) of the eval layer (SPEC §4, SPEC-ADDITIONS §18).
package oracle

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/agentic-dev-harness/cmd/root"
	oraclelib "github.com/StevenACoffman/agentic-dev-harness/internal/oracle"
	"github.com/StevenACoffman/agentic-dev-harness/internal/shell"
)

const corpusSeed uint64 = 1234

const (
	corpusBoards = 3000
	corpusRows   = 4
	corpusCols   = 4
	corpusHues   = 3
)

// oracleGateCode is the exit code a divergence reports (SPEC §7): it matches the
// FindingOracle evaluation gate, so a §13 tool wrapping `oracle diff` confirms an
// oracle finding when it exits non-zero.
const oracleGateCode = 5

// Config holds the configuration for the oracle command.
type Config struct {
	*root.Config
	Reference string
	Candidate string
	Flags     *ff.FlagSet
	Command   *ff.Command
}

// New creates and registers the oracle command with the given parent config.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("oracle").SetParent(parent.Flags)
	cfg.Flags.StringVar(&cfg.Reference, 0, "reference", "",
		"reference command for `diff`; with --candidate runs a command-level differential oracle")
	cfg.Flags.StringVar(&cfg.Candidate, 0, "candidate", "",
		"candidate command for `diff`, compared against --reference")
	cfg.Command = &ff.Command{
		Name:      "oracle",
		Usage:     "agentic-dev-harness oracle [--reference cmd --candidate cmd] <diff|invariants|selftest>",
		ShortHelp: "differential oracle, invariants, and gate self-test",
		LongHelp: "Run the differential oracle (diff), the invariant checks (invariants), or the " +
			"planted-defect self-test (selftest). `diff --reference <cmd> --candidate <cmd>` runs a " +
			"command-level differential oracle over two repository commands (declare it as a §13 tool " +
			"so an oracle finding confirms real divergence); with no flags, diff runs the built-in oracle.",
		Flags: cfg.Flags,
		Exec:  cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

func (cfg *Config) exec(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("oracle: expected a verb: diff, invariants, or selftest")
	}
	switch args[0] {
	case "diff":
		return cfg.diff(ctx)
	case "invariants":
		return cfg.invariants()
	case "selftest":
		return cfg.selfTest()
	default:
		return fmt.Errorf("oracle: unknown verb %q; want diff, invariants, or selftest", args[0])
	}
}

// diff runs the command-level differential oracle when both --reference and --candidate
// are set, otherwise the built-in board oracle. Exactly one flag is a usage error.
func (cfg *Config) diff(ctx context.Context) error {
	switch {
	case cfg.Reference != "" && cfg.Candidate != "":
		return cfg.diffCommands(ctx)
	case cfg.Reference != "" || cfg.Candidate != "":
		return errors.New("oracle: diff command mode needs both --reference and --candidate")
	}
	boards := oraclelib.GenerateBoards(corpusSeed, corpusBoards, corpusRows, corpusCols, corpusHues)
	div := oraclelib.Diverges(oraclelib.React, oraclelib.Native, boards)
	rep := oraclelib.Report{Boards: len(boards), Divergent: div}
	_, _ = fmt.Fprintln(cfg.Stdout, rep.String())
	if div != nil {
		return root.ExitError(oracleGateCode)
	}
	return nil
}

// diffCommands runs the two repository commands, compares their output, and confirms a
// divergence (§2.1, §19.2): a general "two implementations grade each other" for any
// repo. It exits with the oracle gate code on divergence, so a §13 tool wrapping it
// confirms an oracle finding.
func (cfg *Config) diffCommands(ctx context.Context) error {
	refOut, err := cfg.capture(ctx, cfg.Reference)
	if err != nil {
		return err
	}
	candOut, err := cfg.capture(ctx, cfg.Candidate)
	if err != nil {
		return err
	}
	div := oraclelib.DiffOutputs(refOut, candOut)
	if div == nil {
		_, _ = fmt.Fprintln(
			cfg.Stdout,
			"differential oracle: reference and candidate outputs match",
		)
		return nil
	}
	_, _ = fmt.Fprintf(cfg.Stdout,
		"DIVERGENCE at line %d:\n  reference: %q\n  candidate: %q\n",
		div.Line, div.Reference, div.Candidate)
	return root.ExitError(oracleGateCode)
}

// capture runs a repository command and returns its stdout. A command that could not
// start is an error (the oracle cannot compare it); any exit code otherwise yields its
// output — a crash that changes the output is itself a divergence.
func (cfg *Config) capture(ctx context.Context, command string) (string, error) {
	var out bytes.Buffer
	exit, ran := shell.Runner{}.RunIO(ctx, command, cfg.repoDir(), &out, cfg.Stderr)
	if shell.NotRun(exit, ran) {
		return "", fmt.Errorf("oracle: command could not run: %s", command)
	}
	return out.String(), nil
}

// repoDir is the repository the commands run in — the --repo global, or the current
// directory.
func (cfg *Config) repoDir() string {
	if cfg.Repo != "" {
		return cfg.Repo
	}
	return "."
}

func (cfg *Config) invariants() error {
	boards := oraclelib.GenerateBoards(corpusSeed, corpusBoards, corpusRows, corpusCols, corpusHues)
	for _, board := range boards {
		if !oraclelib.InvariantsHold(board, oraclelib.Native(board)) {
			_, _ = fmt.Fprintf(cfg.Stderr, "invariant violated at board %v\n", board)
			return root.ExitError(6)
		}
	}
	_, _ = fmt.Fprintf(cfg.Stdout, "invariants hold over %d boards\n", len(boards))
	return nil
}

func (cfg *Config) selfTest() error {
	if err := oraclelib.SelfTest(corpusSeed); err != nil {
		_, _ = fmt.Fprintf(cfg.Stderr, "%s\n", err)
		return root.ExitError(15)
	}
	_, _ = fmt.Fprintln(cfg.Stdout, "gate self-test passed: both nets catch the planted defect")
	return nil
}
