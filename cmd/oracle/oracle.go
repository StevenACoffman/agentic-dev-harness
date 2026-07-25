// Package oracle implements the "oracle" CLI command: the differential oracle
// (diff), the invariant checks (invariants), and the planted-defect self-test
// (selftest) of the eval layer (SPEC §4, SPEC-ADDITIONS §18).
package oracle

import (
	"context"
	"errors"
	"fmt"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/agentic-dev-harness/cmd/root"
	oraclelib "github.com/StevenACoffman/agentic-dev-harness/internal/oracle"
)

const corpusSeed uint64 = 1234

const (
	corpusRows = 3000
	corpusLen  = 6
	corpusHues = 3
)

// Config holds the configuration for the oracle command.
type Config struct {
	*root.Config
	Flags   *ff.FlagSet
	Command *ff.Command
}

// New creates and registers the oracle command with the given parent config.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("oracle").SetParent(parent.Flags)
	cfg.Command = &ff.Command{
		Name:      "oracle",
		Usage:     "agentic-dev-harness oracle <diff|invariants|selftest>",
		ShortHelp: "differential oracle, invariants, and gate self-test",
		LongHelp:  "Run the differential oracle (diff), the invariant checks (invariants), or the planted-defect self-test (selftest).",
		Flags:     cfg.Flags,
		Exec:      cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

func (cfg *Config) exec(_ context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("oracle: expected a verb: diff, invariants, or selftest")
	}
	switch args[0] {
	case "diff":
		return cfg.diff()
	case "invariants":
		return cfg.invariants()
	case "selftest":
		return cfg.selfTest()
	default:
		return fmt.Errorf("oracle: unknown verb %q; want diff, invariants, or selftest", args[0])
	}
}

func (cfg *Config) diff() error {
	rows := oraclelib.GenerateRows(corpusSeed, corpusRows, corpusLen, corpusHues)
	div := oraclelib.Diverges(oraclelib.React, oraclelib.Native, rows)
	rep := oraclelib.Report{Rows: len(rows), Divergent: div}
	_, _ = fmt.Fprintln(cfg.Stdout, rep.String())
	if div != nil {
		return root.ExitError(5)
	}
	return nil
}

func (cfg *Config) invariants() error {
	rows := oraclelib.GenerateRows(corpusSeed, corpusRows, corpusLen, corpusHues)
	for _, row := range rows {
		if !oraclelib.InvariantsHold(row, oraclelib.Native(row)) {
			_, _ = fmt.Fprintf(cfg.Stderr, "invariant violated at row %v\n", row)
			return root.ExitError(6)
		}
	}
	_, _ = fmt.Fprintf(cfg.Stdout, "invariants hold over %d rows\n", len(rows))
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
