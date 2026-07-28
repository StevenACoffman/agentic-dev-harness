// Package evaluation is the deterministic Evaluation stage (SPEC-ADDITIONS §19.2):
// it adjudicates the cold critic's findings by running the repository artifact
// each one names, then disposes of the arc. A finding is never trusted on the
// critic's text — a confirmed one (its artifact ran and failed) returns the arc to
// Execution and records a failure-registry entry; an unconfirmed one becomes a §11
// lesson candidate and does not block. It is shared by `adh eval`, `run`, and
// `step` so evaluation is deterministic on every path, never a model step.
package evaluation

import (
	"context"
	"errors"
	"fmt"
	"os/exec"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/critic"
	"github.com/StevenACoffman/agentic-dev-harness/internal/device"
	"github.com/StevenACoffman/agentic-dev-harness/internal/failures"
	"github.com/StevenACoffman/agentic-dev-harness/internal/oracle"
	"github.com/StevenACoffman/agentic-dev-harness/internal/proof"
	"github.com/StevenACoffman/agentic-dev-harness/internal/toolreg"
)

// oracle corpus for adjudicating oracle/invariant findings; a small deterministic
// board set, enough to surface a divergence or invariant break when one exists.
const oracleSeed uint64 = 1234

const (
	oracleBoards = 300
	oracleDim    = 4
	oracleHues   = 3
)

// DefaultMaxReworks bounds the rework loop (SPEC §4.1, §19.3): the number of times
// Evaluation may confirm a finding and return an arc to Execution before the arc
// fails terminally instead of looping. A sensible default beats a config knob no
// one tunes; a caller that knows better passes its own budget.
const DefaultMaxReworks = 2

// Disposition values (see Disposition).
const (
	AdvanceToOps      Disposition = iota // no finding confirmed; on to the ops gate
	ReturnToExecution                    // a finding confirmed, within the rework budget
	Fail                                 // a finding confirmed, rework budget exhausted
)

// Disposition is what Evaluation does with an arc given a verdict and the arc's
// rework history (SPEC §4.1): advance to the ops gate, return to Execution to
// rework, or fail terminally once the rework budget is spent.
type Disposition int

// Adjudicator runs the repository artifact a critic finding names and reports
// whether it could be run and whether it failed (§19.2). It is the point-of-use
// seam: RepoAdjudicator is the real one; tests inject a fake.
type Adjudicator interface {
	Adjudicate(ctx context.Context, finding adh.Finding) (ran, failed bool, err error)
}

// RepoAdjudicator runs the concrete repository artifacts a finding can name. The
// oracle/invariant/device runs are self-contained checks; a contract finding names
// a proof manifest verified against it; an NFR finding names a declared check in
// the tool registry, run by runner. dir is the repository root the checks run in.
// The zero value is usable (dir defaults to ".", a nil runner makes NFR findings
// unrunnable), so a caller with no registry configured needs no setup.
type RepoAdjudicator struct {
	dir    string
	checks toolreg.Registry
	runner CheckRunner
}

// CheckRunner runs a repository-declared executable check (an NFR constraint, §10)
// and reports whether it passed (exited zero) and whether it ran at all. ran is
// false when the check could not start (the command is absent or the context was
// canceled). There is no error channel: a non-zero exit is a finding confirmation
// (§19.2), not an error, and an unstartable check is unconfirmed, not a failure of
// the Evaluation stage. The command is repository-owned config, never model input.
type CheckRunner interface {
	RunCheck(ctx context.Context, command, dir string) (passed, ran bool)
}

// ShellRunner runs a check as `sh -c <command>` in dir. It is the one effectful
// edge of adjudication; RepoAdjudicator holds it behind CheckRunner so tests
// inject a fake.
type ShellRunner struct{}

// NewRepoAdjudicator builds an adjudicator rooted at dir, resolving NFR findings
// against checks and running them with runner. An empty dir means the current
// directory; an empty registry or nil runner leaves NFR findings unrunnable.
func NewRepoAdjudicator(dir string, checks toolreg.Registry, runner CheckRunner) RepoAdjudicator {
	return RepoAdjudicator{dir: dir, checks: checks, runner: runner}
}

// RepoAdjudicatorFor is the real adjudicator for a repository: it loads the tool
// registry (§13) that resolves NFR findings and wires the shell runner. A repo
// with no registry file adjudicates NFR findings as unrunnable, not an error, so
// the common case needs no setup. It is the one call the `eval`, `run`, and `step`
// shells make.
func RepoAdjudicatorFor(repoDir string) (RepoAdjudicator, error) {
	checks, err := loadChecks(repoDir)
	if err != nil {
		return RepoAdjudicator{}, err
	}
	return NewRepoAdjudicator(repoDir, checks, ShellRunner{}), nil
}

// loadChecks reads the tool registry under repoDir (best-effort: an absent file is
// an empty registry, so a repo declaring no NFR checks is not an error).
func loadChecks(repoDir string) (toolreg.Registry, error) {
	reg, err := toolreg.LoadRepo(repoDir)
	if err != nil {
		return toolreg.Registry{}, &adh.Error{Op: "evaluation.loadChecks", Err: err}
	}
	return reg, nil
}

// RunCheck runs command via the shell in dir. A clean exit passes; a non-zero exit
// ran-and-failed; a command that could not start (not found, canceled) did not run.
func (ShellRunner) RunCheck(ctx context.Context, command, dir string) (passed, ran bool) {
	// The command is a repository-owned tool-registry entry (§13), authored by the
	// maintainer, not agent or critic input — the critic supplies only the tool ID
	// to select, so there is no injection path from the model.
	cmd := exec.CommandContext(ctx, "sh", "-c", command) //nolint:gosec // repo-owned config
	cmd.Dir = dir
	err := cmd.Run()
	if err == nil {
		return true, true
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return false, true // the check ran and reported failure
	}
	return false, false // could not start the check
}

// Decide picks the arc's disposition (SPEC §4.1): a clean verdict advances to the
// ops gate; a confirmed finding returns the arc to Execution to rework until the
// budget is spent, after which the arc fails terminally rather than looping. It is
// pure — the caller (Apply) mutates the arc.
func Decide(verdict *critic.Verdict, reworks, maxReworks int) Disposition {
	if !verdict.ReturnsToExecution() {
		return AdvanceToOps
	}
	if reworks >= maxReworks {
		return Fail
	}
	return ReturnToExecution
}

// Adjudicate runs each finding's artifact and disposes of the results (§19.2). It
// is pure with respect to the arc — the caller applies the verdict.
func Adjudicate(
	ctx context.Context,
	adjudicator Adjudicator,
	findings []adh.Finding,
) (critic.Verdict, error) {
	results := make([]critic.Adjudicated, 0, len(findings))
	for i := range findings {
		finding := findings[i]
		ran, failed, err := adjudicator.Adjudicate(ctx, finding)
		if err != nil {
			return critic.Verdict{}, fmt.Errorf("adjudicating %s finding: %w", finding.Kind, err)
		}
		results = append(results, critic.Adjudicated{Finding: finding, Ran: ran, Failed: failed})
	}
	return critic.Dispose(results), nil
}

// Apply records the verdict and moves the arc on (SPEC §4.1, §19.2): unconfirmed
// findings become lesson candidates (when recordLessons is set), and the arc's
// disposition follows Decide — advance to the ops gate, return to Execution to
// rework (within maxReworks, incrementing Reworks), or fail terminally once the
// budget is spent. A confirmed finding is appended to the failure registry on both
// the rework and the terminal path. It clears the arc's findings. It mutates arc
// and writes the registries; it neither saves the arc nor sets an exit code — those
// stay with the caller.
func Apply(arc *adh.Arc, verdict *critic.Verdict, recordLessons bool, maxReworks int) error {
	const op = "evaluation.Apply"
	if recordLessons {
		if err := failures.Append(failures.CandidatesFile, verdict.LessonNotes()...); err != nil {
			return &adh.Error{Op: op, Err: err}
		}
	}
	disposition := Decide(verdict, arc.Reworks, maxReworks)
	if disposition != AdvanceToOps { // both the rework and the terminal path failed a check
		if err := failures.Append(failures.RegistryFile, verdict.FailureNotes()...); err != nil {
			return &adh.Error{Op: op, Err: err}
		}
	}
	switch disposition {
	case ReturnToExecution:
		arc.Reworks++
		arc.Stage = adh.StageExecution
		arc.History = append(arc.History, fmt.Sprintf(
			"evaluation: %d finding(s) confirmed; returned to execution (rework %d/%d)",
			len(verdict.Confirmed), arc.Reworks, maxReworks,
		))
	case Fail:
		arc.Status = adh.StatusFailed
		arc.History = append(arc.History, fmt.Sprintf(
			"evaluation: %d finding(s) confirmed; failed after %d rework(s), escalate to a human",
			len(verdict.Confirmed), arc.Reworks,
		))
	case AdvanceToOps:
		arc.Stage = adh.StageOps
		arc.History = append(arc.History, fmt.Sprintf(
			"evaluation: no findings confirmed; %d lesson candidate(s)",
			len(verdict.Unconfirmed),
		))
	}
	arc.Findings = nil
	return nil
}

// Adjudicate runs the artifact finding names. oracle/invariant/device are
// self-contained checks; a contract finding names a proof manifest and fails when
// the packet is missing or does not verify; an NFR finding names a declared check
// in the tool registry and fails when that check exits non-zero. A finding whose
// artifact cannot be run (an NFR check the repository does not declare, an NFR
// finding with no runner) is unrunnable (ran=false) and disposes as unconfirmed —
// the gate drops a prior the repository does not hold (§19.2).
func (a RepoAdjudicator) Adjudicate(
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
		ran, failed = a.adjudicateContract(finding.Ref)
		return ran, failed, nil
	case adh.FindingNFR:
		ran, failed = a.adjudicateNFR(ctx, finding.Ref)
		return ran, failed, nil
	default:
		return false, false, nil
	}
}

// adjudicateNFR runs the executable check an NFR finding names (§10, §19.2). The
// finding's ref is a tool ID in the registry; the check's own command is run in
// the repository. A finding that names no check, names one the registry does not
// declare, or has no runner wired is unrunnable — an invented requirement the
// repository does not hold, which the gate drops as unconfirmed. A declared check
// that exits non-zero confirms the finding.
func (a RepoAdjudicator) adjudicateNFR(ctx context.Context, ref string) (ran, failed bool) {
	if ref == "" || a.runner == nil {
		return false, false
	}
	tool, ok := a.checks.FindByID(ref)
	if !ok {
		return false, false
	}
	passed, ranCheck := a.runner.RunCheck(ctx, tool.Run, a.repoRoot())
	if !ranCheck {
		return false, false
	}
	return true, !passed
}

// adjudicateContract verifies the proof manifest a contract finding names. A
// finding that names no manifest is unrunnable; a manifest that is missing,
// unreadable, or fails verification confirms the finding (the named proof does not
// hold).
func (a RepoAdjudicator) adjudicateContract(ref string) (ran, failed bool) {
	if ref == "" {
		return false, false // unrunnable → unconfirmed
	}
	pkt, err := proof.Load(ref)
	if err != nil {
		return true, true // named proof missing or unreadable = failed
	}
	return true, proof.Verify(a.repoRoot(), &pkt) != nil
}

// repoRoot is the directory the checks run in, defaulting to the current directory
// so the zero-value adjudicator stays usable.
func (a RepoAdjudicator) repoRoot() string {
	if a.dir == "" {
		return "."
	}
	return a.dir
}
