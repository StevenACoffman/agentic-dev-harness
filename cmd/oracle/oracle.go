// Package oracle implements the "oracle" CLI command: the differential oracle
// (diff), the invariant checks (invariants), and the planted-defect self-test
// (selftest) of the eval layer (SPEC §4, SPEC-ADDITIONS §18).
package oracle

import (
	"bytes"
	"context"
	"fmt"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/agentic-dev-harness/cmd/root"
	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
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
		return root.MissingVerbError{
			Scope: "oracle",
			Verbs: []string{"diff", "invariants", "selftest"},
		}
	}
	switch args[0] {
	case "diff":
		return cfg.diff(ctx)
	case "invariants":
		return cfg.invariants()
	case "selftest":
		return cfg.selfTest()
	default:
		return root.UnknownVerbError{
			Scope: "oracle", Got: args[0],
			Verbs: []string{"diff", "invariants", "selftest"},
		}
	}
}

// diff runs the command-level differential oracle when both --reference and --candidate
// are set, otherwise the built-in board oracle. Exactly one flag is a usage error.
func (cfg *Config) diff(ctx context.Context) error {
	switch {
	case cfg.Reference != "" && cfg.Candidate != "":
		return cfg.diffCommands(ctx)
	case cfg.Reference != "" || cfg.Candidate != "":
		return &adh.Error{
			Code:    adh.EINVALID,
			Message: "oracle: diff command mode needs both --reference and --candidate",
		}
	}
	boards := oraclelib.GenerateBoards(corpusSeed, corpusBoards, corpusRows, corpusCols, corpusHues)
	div := oraclelib.Diverges(oraclelib.React, oraclelib.Native, boards)
	rep := oraclelib.Report{Boards: len(boards), Divergent: div}
	if cfg.JSONL {
		// Corpus mode answers "do the two engines agree over N generated boards".
		// The `mode` key exists because command mode answers a different question
		// under the same verb, and a consumer branching on `boards` must not silently
		// receive a command-mode result.
		data := map[string]any{"mode": "corpus", "boards": rep.Boards}
		if div == nil {
			if err := cfg.EmitOK(data); err != nil {
				return fmt.Errorf("oracle: %w", err)
			}
			return nil
		}
		// The divergent board is the finding. An exit code alone tells an agent that
		// something disagreed and not on what, which is the whole content.
		data["divergent_board"] = div
		if err := cfg.EmitError(oracleGateCode, "oracle-divergence",
			"the reference and candidate engines disagree"); err != nil {
			return fmt.Errorf("oracle: %w", err)
		}
		return root.ExitError(oracleGateCode)
	}
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
	if cfg.JSONL {
		return cfg.emitCommandDiff(div)
	}
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

// emitCommandDiff reports command-mode diff as an envelope.
//
// `mode` distinguishes it from corpus mode, which answers a different question under
// the same verb. The divergence travels whole -- line, reference, candidate -- because
// that triple is the finding, and an agent handed only an exit code knows something
// disagreed and not what.
func (cfg *Config) emitCommandDiff(div *oraclelib.CommandDivergence) error {
	if div == nil {
		if err := cfg.EmitOK(map[string]any{"mode": "command", "divergent": false}); err != nil {
			return fmt.Errorf("oracle: %w", err)
		}
		return nil
	}
	if err := cfg.EmitError(oracleGateCode, "oracle-divergence", fmt.Sprintf(
		"reference and candidate differ at line %d", div.Line)); err != nil {
		return fmt.Errorf("oracle: %w", err)
	}
	return root.ExitError(oracleGateCode)
}

func (cfg *Config) invariants() error {
	boards := oraclelib.GenerateBoards(corpusSeed, corpusBoards, corpusRows, corpusCols, corpusHues)
	for _, board := range boards {
		if !oraclelib.InvariantsHold(board, oraclelib.Native(board)) {
			if cfg.JSONL {
				// The violating board is the answer. A gate that failed is more
				// interesting than one that passed, so the failing path is the one
				// that must carry data rather than only an exit code.
				if err := cfg.EmitError(6, "invariant-violated", fmt.Sprintf(
					"an invariant does not hold at board %v", board)); err != nil {
					return fmt.Errorf("oracle: %w", err)
				}
				return root.ExitError(6)
			}
			_, _ = fmt.Fprintf(cfg.Stderr, "invariant violated at board %v\n", board)
			return root.ExitError(6)
		}
	}
	if cfg.JSONL {
		if err := cfg.EmitOK(map[string]any{"boards": len(boards), "hold": true}); err != nil {
			return fmt.Errorf("oracle: %w", err)
		}
		return nil
	}
	_, _ = fmt.Fprintf(cfg.Stdout, "invariants hold over %d boards\n", len(boards))
	return nil
}

func (cfg *Config) selfTest() error {
	if err := oraclelib.SelfTest(corpusSeed); err != nil {
		if cfg.JSONL {
			// A failed self-test means the gate cannot be trusted, which is the most
			// consequential thing this binary can report; it must not arrive as a
			// bare exit code.
			if eErr := cfg.EmitError(15, "gate-untrustworthy", err.Error()); eErr != nil {
				return fmt.Errorf("oracle: %w", eErr)
			}
			return root.ExitError(15)
		}
		_, _ = fmt.Fprintf(cfg.Stderr, "%s\n", err)
		return root.ExitError(15)
	}
	if cfg.JSONL {
		if err := cfg.EmitOK(map[string]any{"passed": true}); err != nil {
			return fmt.Errorf("oracle: %w", err)
		}
		return nil
	}
	_, _ = fmt.Fprintln(cfg.Stdout, "gate self-test passed: both nets catch the planted defect")
	return nil
}
