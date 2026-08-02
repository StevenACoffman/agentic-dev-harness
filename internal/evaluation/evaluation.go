// Package evaluation is the deterministic Evaluation stage (SPEC-ADDITIONS §19.2):
// it adjudicates the cold critic's findings by running the repository artifact
// each one names, then disposes of the arc. A finding is never trusted on the
// critic's text — a confirmed one (its artifact ran and failed) returns the arc to
// Execution and records a failure-registry entry; an unconfirmed one becomes a §11
// lesson candidate and does not block. It is shared by `adh eval`, `run`, and
// `step` so evaluation is deterministic on every path, never a model step.
package evaluation

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/critic"
	"github.com/StevenACoffman/agentic-dev-harness/internal/device"
	"github.com/StevenACoffman/agentic-dev-harness/internal/failures"
	"github.com/StevenACoffman/agentic-dev-harness/internal/nfr"
	"github.com/StevenACoffman/agentic-dev-harness/internal/oracle"
	"github.com/StevenACoffman/agentic-dev-harness/internal/proof"
	"github.com/StevenACoffman/agentic-dev-harness/internal/shell"
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

// RepoAdjudicator runs the concrete repository artifacts a finding can name. An
// oracle/invariant/device finding runs the declared §13 tool its ref names (the real
// domain target the repository provides) or, when it names none, adh's built-in check
// for that kind; a contract finding names a proof manifest verified against it; an NFR
// finding names a declared check in the tool registry, run by runner. dir is the
// repository root the checks run in. The zero value is usable (dir defaults to ".", a
// nil runner makes tool-backed findings unrunnable), so a caller with no registry
// configured needs no setup.
type RepoAdjudicator struct {
	dir    string
	checks toolreg.Registry
	specs  []nfr.Spec
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
	// Measure runs a Meter tool and returns the numeric value it emits (§10.5), so an
	// NFR finding can gate on a declarative Fail threshold rather than an exit code.
	// ran is false when the command could not start or emitted no parseable number.
	Measure(ctx context.Context, command, dir string) (value float64, ran bool)
}

// ShellRunner runs a check as `sh -c <command>` in dir through the shared
// internal/shell edge. RepoAdjudicator holds it behind CheckRunner so tests inject
// a fake.
type ShellRunner struct{}

// NewRepoAdjudicator builds an adjudicator rooted at dir, resolving NFR findings
// against checks and running them with runner. An empty dir means the current
// directory; an empty registry or nil runner leaves NFR findings unrunnable.
func NewRepoAdjudicator(
	dir string,
	checks toolreg.Registry,
	specs []nfr.Spec,
	runner CheckRunner,
) RepoAdjudicator {
	return RepoAdjudicator{dir: dir, checks: checks, specs: specs, runner: runner}
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
	specs, err := nfr.Load(filepath.Join(repoDir, nfr.DefaultDir))
	if err != nil {
		return RepoAdjudicator{}, &adh.Error{Op: "evaluation.RepoAdjudicatorFor", Err: err}
	}
	return NewRepoAdjudicator(repoDir, checks, specs, ShellRunner{}), nil
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
	code, ran := shell.Runner{}.Run(ctx, command, dir)
	return code == 0, ran
}

// Measure runs a Meter tool capturing its stdout and parses the value it emits
// (§10.5): the last whitespace-separated float token. The tool's exit code is not
// the signal — a Meter that exits non-zero but prints a number still measures — so
// only a command that could not start (not found) or emits no parseable number is
// unmeasurable (ran=false), leaving the NFR finding unconfirmed rather than a false
// gate (§19.2).
func (ShellRunner) Measure(ctx context.Context, command, dir string) (value float64, ran bool) {
	var out bytes.Buffer
	code, started := shell.Runner{}.RunIO(ctx, command, dir, &out, nil)
	if shell.NotRun(code, started) {
		return 0, false
	}
	return parseMeasurement(out.String())
}

// parseMeasurement extracts the measured value from a Meter tool's output — the last
// whitespace-separated token parsed as a float. ok is false when there is no
// parseable number, so the Meter contract is "print the measurement as the final
// token" and anything else is unmeasurable.
func parseMeasurement(out string) (value float64, ok bool) {
	fields := strings.Fields(out)
	if len(fields) == 0 {
		return 0, false
	}
	v, err := strconv.ParseFloat(fields[len(fields)-1], 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

// Decide picks the arc's disposition (SPEC §4.1, §19.2): a clean verdict advances to
// the ops gate; a confirmed structural finding fails terminally at once (a design
// change the rework loop cannot close — escalate to a human); an ordinary confirmed
// finding returns the arc to Execution to rework until the budget is spent, after
// which it fails terminally rather than looping. It is pure — the caller (Apply)
// mutates the arc.
func Decide(verdict *critic.Verdict, reworks, maxReworks int) Disposition {
	if !verdict.ReturnsToExecution() {
		return AdvanceToOps
	}
	if verdict.HasStructural() || reworks >= maxReworks {
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
// the rework and the terminal path, and every disposed class is stamped into the
// failure-record log (stratum + scope + root cause) for the §11 accretion gate. It
// clears the arc's findings. It mutates arc and writes the registries; it neither
// saves the arc nor sets an exit code — those stay with the caller. stratum is the
// year-month the shell computes, kept out of the core.
func Apply(
	arc *adh.Arc,
	verdict *critic.Verdict,
	recordLessons bool,
	maxReworks int,
	stratum string,
) error {
	const op = "evaluation.Apply"
	if err := recordStrata(arc, verdict, stratum); err != nil {
		return &adh.Error{Op: op, Err: err}
	}
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

// recordStrata stamps every class the verdict disposed into the failure-record log
// (§19.2): each carries the stratum, the arc's routing scope, and the root cause
// derived from whether the attempt was grounded — the evidence the §11 accretion gate
// reads. Recording nothing when there are no classes keeps the log from growing on a
// clean review.
func recordStrata(arc *adh.Arc, verdict *critic.Verdict, stratum string) error {
	const op = "evaluation.recordStrata"
	classes := verdict.Classes()
	if len(classes) == 0 {
		return nil
	}
	rootCause := failures.ClassifyRootCause(len(arc.Context) > 0)
	recs := make([]failures.Record, len(classes))
	for i, class := range classes {
		recs[i] = failures.Record{
			Class:     class,
			Stratum:   stratum,
			Labels:    arc.Labels,
			Paths:     arc.Paths,
			RootCause: rootCause,
		}
	}
	if err := failures.AppendRecords(failures.RecordsFile, recs...); err != nil {
		return &adh.Error{Op: op, Err: err}
	}
	return nil
}

// Adjudicate runs the artifact a finding names (§19.2). An oracle/invariant/device
// finding runs the repository's declared §13 tool when its ref names one (the real
// domain target), else adh's built-in check for that kind; a contract finding names a
// proof manifest and fails when the packet is missing or does not verify; an NFR
// finding names a declared check or Planguage spec and fails when it breaches. A
// finding whose artifact cannot be run — a declared check the repository does not hold,
// an unstartable tool — is unrunnable (ran=false) and disposes as unconfirmed, so the
// gate drops a prior the repository does not hold.
func (a *RepoAdjudicator) Adjudicate(
	ctx context.Context,
	finding adh.Finding,
) (ran, failed bool, err error) {
	switch finding.Kind {
	case adh.FindingOracle:
		if ran, failed, ok := a.runDeclaredTool(ctx, finding.Ref); ok {
			return ran, failed, nil
		}
		return true, a.builtinOracle(), nil // the built-in oracle always runs
	case adh.FindingInvariant:
		if ran, failed, ok := a.runDeclaredTool(ctx, finding.Ref); ok {
			return ran, failed, nil
		}
		return true, a.builtinInvariant(), nil // the built-in invariant check always runs
	case adh.FindingDevice:
		if ran, failed, ok := a.runDeclaredTool(ctx, finding.Ref); ok {
			return ran, failed, nil
		}
		return a.builtinDevice(ctx)
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

// builtinOracle runs adh's in-package differential oracle — the React/Native pair
// grade each other (§2.1) — and reports whether they diverged (a confirmation). It
// always runs, so it returns only the failed signal; the fallback when a finding names
// no declared oracle tool.
func (a *RepoAdjudicator) builtinOracle() (failed bool) {
	boards := oracle.GenerateBoards(oracleSeed, oracleBoards, oracleDim, oracleDim, oracleHues)
	return oracle.Diverges(oracle.React, oracle.Native, boards) != nil
}

// builtinInvariant runs adh's in-package property checks over the native engine's
// output and reports whether an invariant broke (a confirmation). It always runs; the
// fallback for an invariant finding that names no declared tool.
func (a *RepoAdjudicator) builtinInvariant() (failed bool) {
	boards := oracle.GenerateBoards(oracleSeed, oracleBoards, oracleDim, oracleDim, oracleHues)
	for _, board := range boards {
		if !oracle.InvariantsHold(board, oracle.Native(board)) {
			return true
		}
	}
	return false
}

// builtinDevice runs adh's healthy device mock; an unhealthy report confirms the
// finding. The fallback for a device finding that names no declared tool (a real adb
// adapter is provided as a §13 tool).
func (a *RepoAdjudicator) builtinDevice(ctx context.Context) (ran, failed bool, err error) {
	report, verr := (device.Mock{Healthy: true}).Validate(ctx)
	if verr != nil {
		return false, false, fmt.Errorf("device validate: %w", verr)
	}
	return true, !report.OK, nil
}

// runDeclaredTool runs the §13 tool a finding's ref names, if the repository declares
// one (§19.2): the tool's exit code is the signal — the domain-specific artifact the
// repository provides rather than an adh built-in. ok is false when ref is empty, no
// runner is wired, or ref names no declared tool, so the caller falls back to adh's
// built-in check for that kind. A declared tool that cannot start is ran=false
// (unconfirmed), the same as any unrunnable artifact — never a false confirmation.
func (a *RepoAdjudicator) runDeclaredTool(ctx context.Context, ref string) (ran, failed, ok bool) {
	if ref == "" || a.runner == nil {
		return false, false, false
	}
	tool, found := a.checks.FindByID(ref)
	if !found {
		return false, false, false
	}
	passed, ranCheck := a.runner.RunCheck(ctx, tool.Run, a.repoRoot())
	return ranCheck, ranCheck && !passed, true
}

// adjudicateNFR runs the executable check an NFR finding names (§10, §19.2). The
// finding's ref is a Planguage spec or a tool ID in the registry; the check's own
// command is run in the repository. A finding that names no check, names one the
// registry does not declare, or has no runner wired is unrunnable — an invented
// requirement the repository does not hold, which the gate drops as unconfirmed. A
// declared check that exits non-zero confirms the finding.
func (a *RepoAdjudicator) adjudicateNFR(ctx context.Context, ref string) (ran, failed bool) {
	if ref == "" || a.runner == nil {
		return false, false
	}
	// A ref that names a Planguage spec gates on the declarative Fail threshold: run
	// the spec's Meter tool, measure, and confirm when the value breaches Fail (§10.5)
	// — adh owns the threshold, not the tool. Otherwise ref is a tool id whose own
	// exit code is the pass/fail signal (backward compatible).
	if spec, ok := nfr.ByID(a.specs, ref); ok {
		return a.adjudicateSpec(ctx, &spec)
	}
	ran, failed, _ = a.runDeclaredTool(ctx, ref)
	return ran, failed
}

// adjudicateSpec measures a Planguage spec's Meter and confirms the finding when the
// measured value breaches the spec's Fail bar (§10.5, §19.2). A spec whose Meter is
// not a declared §13 tool, or whose tool emits no parseable measurement, is
// unrunnable (unconfirmed) — the gate drops a requirement it cannot measure.
func (a *RepoAdjudicator) adjudicateSpec(ctx context.Context, spec *nfr.Spec) (ran, failed bool) {
	tool, ok := a.checks.FindByID(spec.Meter)
	if !ok {
		return false, false
	}
	value, measured := a.runner.Measure(ctx, tool.Run, a.repoRoot())
	if !measured {
		return false, false
	}
	return true, !spec.Meets(value)
}

// adjudicateContract verifies the proof manifest a contract finding names. A
// finding that names no manifest is unrunnable; a manifest that is missing,
// unreadable, or fails verification confirms the finding (the named proof does not
// hold).
func (a *RepoAdjudicator) adjudicateContract(ref string) (ran, failed bool) {
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
func (a *RepoAdjudicator) repoRoot() string {
	if a.dir == "" {
		return "."
	}
	return a.dir
}
