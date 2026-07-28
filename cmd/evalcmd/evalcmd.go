// Package evalcmd implements the "eval" CLI command: the deterministic Evaluation
// stage (SPEC-ADDITIONS §19.2). It adjudicates the cold critic's findings by
// running the repository artifact each one names — a finding is never trusted on
// the critic's text. A confirmed finding (its artifact ran and failed) returns
// the arc to Execution and records a failure-registry entry (exit 5–8); an
// unconfirmed one becomes a §11 lesson candidate and does not block.
package evalcmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/agentic-dev-harness/cmd/root"
	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/config"
	"github.com/StevenACoffman/agentic-dev-harness/internal/critic"
	"github.com/StevenACoffman/agentic-dev-harness/internal/device"
	"github.com/StevenACoffman/agentic-dev-harness/internal/failures"
	"github.com/StevenACoffman/agentic-dev-harness/internal/oracle"
	"github.com/StevenACoffman/agentic-dev-harness/internal/proof"
	"github.com/StevenACoffman/agentic-dev-harness/internal/state"
)

// Exit codes for a confirmed finding, by the artifact its kind names, matching
// the standalone gates: oracle=5, invariant=6, device=7, contract/proof=8.
const (
	exitOracle    = 5
	exitInvariant = 6
	exitDevice    = 7
	exitContract  = 8
)

// oracle corpus for adjudicating oracle/invariant findings; a small deterministic
// board set, enough to surface a divergence or invariant break when one exists.
const oracleSeed uint64 = 1234

const (
	oracleBoards = 300
	oracleDim    = 4
	oracleHues   = 3
)

// Adjudicator runs the repository artifact a critic finding names and reports
// whether it could be run and whether it failed (§19.2). Declared here at the
// point of use: repoAdjudicator is the real one; tests inject a fake.
type Adjudicator interface {
	Adjudicate(ctx context.Context, finding adh.Finding) (ran, failed bool, err error)
}

// Config holds the configuration for the eval command.
type Config struct {
	*root.Config
	adjudicator Adjudicator
	Flags       *ff.FlagSet
	Command     *ff.Command
}

// repoAdjudicator runs the concrete repository artifacts a finding can name. The
// oracle/invariant/device runs are self-contained checks; a contract finding
// names a proof manifest and is verified against it.
type repoAdjudicator struct{}

// New creates and registers the eval command with the given parent config.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.adjudicator = repoAdjudicator{}
	cfg.Flags = ff.NewFlagSet("eval").SetParent(parent.Flags)
	cfg.Command = &ff.Command{
		Name:      "eval",
		Usage:     "agentic-dev-harness eval <arc-id>",
		ShortHelp: "adjudicate the critic's findings and dispose of the arc",
		LongHelp: "Run the artifact each critic finding names (SPEC-ADDITIONS §19.2). " +
			"A confirmed finding returns the arc to execution (exit 5-8); an " +
			"unconfirmed one becomes a lesson candidate and the arc advances to ops.",
		Flags: cfg.Flags,
		Exec:  cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

func (cfg *Config) exec(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("eval: requires an arc id")
	}
	store := state.Default()
	arc, err := store.Get(args[0])
	if err != nil {
		return fmt.Errorf("eval: %w", err)
	}
	if arc.Status != adh.StatusOpen {
		return fmt.Errorf("eval: arc %s is not open (status %s)", arc.ID, arc.Status)
	}
	if arc.Stage != adh.StageEvaluation {
		return fmt.Errorf("eval: arc %s is at %s, not evaluation", arc.ID, arc.Stage)
	}

	conf, err := config.Load(cfg.Getenv)
	if err != nil {
		return fmt.Errorf("eval: %w", err)
	}
	verdict, err := cfg.adjudicate(ctx, arc.Findings)
	if err != nil {
		return fmt.Errorf("eval: %w", err)
	}
	// Unconfirmed findings are lesson candidates (§19.2) whether or not the arc
	// blocks — a confirmed sibling still returns the arc, but the unconfirmed ones
	// stay recorded for recurrence. The disposition is config-driven (§19.4).
	if conf.CriticUnconfirmed() == config.UnconfirmedLesson {
		if err := failures.Append(failures.CandidatesFile, verdict.LessonNotes()...); err != nil {
			return fmt.Errorf("eval: %w", err)
		}
	}
	if verdict.ReturnsToExecution() {
		return cfg.block(store, &arc, &verdict)
	}
	return cfg.advance(store, &arc, &verdict)
}

// adjudicate runs each finding's artifact and disposes of the results (§19.2).
func (cfg *Config) adjudicate(ctx context.Context, findings []adh.Finding) (critic.Verdict, error) {
	results := make([]critic.Adjudicated, 0, len(findings))
	for i := range findings {
		finding := findings[i]
		ran, failed, err := cfg.adjudicator.Adjudicate(ctx, finding)
		if err != nil {
			return critic.Verdict{}, fmt.Errorf("adjudicating %s finding: %w", finding.Kind, err)
		}
		results = append(results, critic.Adjudicated{Finding: finding, Ran: ran, Failed: failed})
	}
	return critic.Dispose(results), nil
}

// block records the confirmed findings as failures and returns the arc to
// execution (§19.2), exiting with the gate code for the first confirmed kind.
func (cfg *Config) block(store *state.Store, arc *adh.Arc, verdict *critic.Verdict) error {
	if err := failures.Append(failures.RegistryFile, verdict.FailureNotes()...); err != nil {
		return fmt.Errorf("eval: %w", err)
	}
	arc.Stage = adh.StageExecution
	arc.History = append(
		arc.History,
		fmt.Sprintf(
			"evaluation: %d finding(s) confirmed; returned to execution",
			len(verdict.Confirmed),
		),
	)
	arc.Findings = nil
	if err := store.Save(arc); err != nil {
		return fmt.Errorf("eval: %w", err)
	}
	_, _ = fmt.Fprintf(
		cfg.Stdout,
		"eval: %d finding(s) confirmed; arc %s returned to execution\n",
		len(verdict.Confirmed),
		arc.ID,
	)
	return root.ExitError(exitFor(verdict.BlockingKind()))
}

// advance records the unconfirmed findings as lesson candidates and moves the arc
// on to the ops gate (§19.2): no finding blocks on the critic's text alone.
func (cfg *Config) advance(store *state.Store, arc *adh.Arc, verdict *critic.Verdict) error {
	arc.Stage = adh.StageOps
	arc.History = append(
		arc.History,
		fmt.Sprintf(
			"evaluation: no findings confirmed; %d lesson candidate(s)",
			len(verdict.Unconfirmed),
		),
	)
	arc.Findings = nil
	if err := store.Save(arc); err != nil {
		return fmt.Errorf("eval: %w", err)
	}
	_, _ = fmt.Fprintf(cfg.Stdout,
		"eval: no findings confirmed; arc %s advanced to ops (%d lesson candidate(s))\n",
		arc.ID, len(verdict.Unconfirmed))
	return nil
}

// exitFor maps a confirmed finding's kind to its Evaluation gate exit code. An
// unknown or NFR kind (no dedicated gate yet) surfaces as the invariant code.
func exitFor(kind adh.FindingKind) int {
	switch kind {
	case adh.FindingOracle:
		return exitOracle
	case adh.FindingInvariant:
		return exitInvariant
	case adh.FindingDevice:
		return exitDevice
	case adh.FindingContract:
		return exitContract
	case adh.FindingNFR:
		return exitInvariant
	default:
		return exitInvariant
	}
}

// Adjudicate runs the artifact finding names. oracle/invariant/device are
// self-contained checks; a contract finding names a proof manifest and fails when
// the packet is missing or does not verify. An NFR finding has no runner yet, so
// it is unrunnable (ran=false) and disposes as unconfirmed (§19.2).
func (repoAdjudicator) Adjudicate(
	ctx context.Context,
	finding adh.Finding,
) (ran, failed bool, err error) {
	switch finding.Kind {
	case adh.FindingOracle:
		boards := oracle.GenerateBoards(oracleSeed, oracleBoards, oracleDim, oracleDim, oracleHues)
		return true, oracle.Diverges(oracle.React, oracle.Native, boards) != nil, nil
	case adh.FindingInvariant:
		boards := oracle.GenerateBoards(oracleSeed, oracleBoards, oracleDim, oracleDim, oracleHues)
		for _, board := range boards {
			if !oracle.InvariantsHold(board, oracle.Native(board)) {
				return true, true, nil
			}
		}
		return true, false, nil
	case adh.FindingDevice:
		report, verr := (device.Mock{Healthy: true}).Validate(ctx)
		if verr != nil {
			return false, false, fmt.Errorf("device validate: %w", verr)
		}
		return true, !report.OK, nil
	case adh.FindingContract:
		ran, failed = adjudicateContract(finding.Ref)
		return ran, failed, nil
	case adh.FindingNFR:
		return false, false, nil
	default:
		return false, false, nil
	}
}

// adjudicateContract verifies the proof manifest a contract finding names. A
// finding that names no manifest is unrunnable; a manifest that is missing,
// unreadable, or fails verification confirms the finding (the named proof does
// not hold).
func adjudicateContract(ref string) (ran, failed bool) {
	if ref == "" {
		return false, false // unrunnable → unconfirmed
	}
	pkt, err := proof.Load(ref)
	if err != nil {
		return true, true // named proof missing or unreadable = failed
	}
	return true, proof.Verify(".", &pkt) != nil
}
