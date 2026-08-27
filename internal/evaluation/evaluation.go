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
	"time"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/contextstore"
	"github.com/StevenACoffman/agentic-dev-harness/internal/critic"
	"github.com/StevenACoffman/agentic-dev-harness/internal/device"
	"github.com/StevenACoffman/agentic-dev-harness/internal/failures"
	"github.com/StevenACoffman/agentic-dev-harness/internal/nfr"
	"github.com/StevenACoffman/agentic-dev-harness/internal/oracle"
	"github.com/StevenACoffman/agentic-dev-harness/internal/shell"
	"github.com/StevenACoffman/agentic-dev-harness/internal/toolreg"
	"github.com/StevenACoffman/agentic-dev-harness/internal/toolrun"
	"github.com/StevenACoffman/skillet/proof"
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

	// AdvanceWithReservation: nothing confirmed, and something measured worse than
	// its baseline while still clearing the acceptance bar.
	//
	// **It advances.** A regression inside the bar is information, not a breach —
	// blocking on it would make Fail and Baseline the same threshold and would refuse
	// changes the repository declared acceptable. What it is not is silent: an arc
	// that advances having cost something says so.
	//
	// A disposition rather than a field on AdvanceToOps, because Decide's result is
	// what the shell branches on to report, and a caller that only ever sees
	// AdvanceToOps will not think to look for a field.
	AdvanceWithReservation
)

// Disposition is what Evaluation does with an arc given a verdict and the arc's
// rework history (SPEC §4.1): advance to the ops gate — with or without a recorded
// reservation — return to Execution to rework, or fail terminally once the rework
// budget is spent.
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
	dir     string
	checks  toolreg.Registry
	specs   []nfr.Spec
	runner  CheckRunner
	logPath string // tool-run log to append to; empty disables logging (the test path)
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
	adj := NewRepoAdjudicator(repoDir, checks, specs, ShellRunner{})
	// The real adjudicator logs each declared-tool run to the tool-run log (§16/§18) so
	// `adh kpi` sees adjudication-time behavior; the test constructor leaves it disabled.
	adj.logPath = filepath.Join(repoDir, toolrun.RunFile)
	return adj, nil
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
		if len(verdict.Reservations) > 0 {
			return AdvanceWithReservation
		}
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
	specs []nfr.Spec,
	findings []adh.Finding,
) (critic.Verdict, error) {
	// **Guards are assembled here, not by the caller.** The rule -- a declared guard is
	// adjudicated whether or not the critic raised it (§10.5) -- lived in `adh eval`,
	// and the other two callers, `run` and `step`, silently did not apply it. So a
	// guard breach passed unnoticed on the paths most likely to be automated, and the
	// stated bound on the silence gap did not hold there at all.
	//
	// specs is a parameter rather than something read from the adjudicator because a
	// caller must *decide*: passing nil is an explicit statement that this call has no
	// guards, which the compiler makes them make. That is the difference between a rule
	// three callers have to remember and one they cannot omit.
	all := append(GuardFindings(specs), findings...)
	results := make([]critic.Adjudicated, 0, len(all))
	for i := range all {
		finding := all[i]
		res, err := adjudicateOne(ctx, adjudicator, finding)
		if err != nil {
			return critic.Verdict{}, fmt.Errorf("adjudicating %s finding: %w", finding.Kind, err)
		}
		results = append(results, res)
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
	if err := recordPrecision(arc, verdict); err != nil {
		return &adh.Error{Op: op, Err: err}
	}
	if recordLessons {
		if err := failures.Append(failures.CandidatesFile, verdict.LessonNotes()...); err != nil {
			return &adh.Error{Op: op, Err: err}
		}
	}
	disposition := Decide(verdict, arc.Reworks, maxReworks)
	// Both the rework and the terminal path failed a check. An advance with a
	// reservation did not: nothing was confirmed, so there is no failure to register —
	// the reservation is a cost, not a defect.
	if disposition == ReturnToExecution || disposition == Fail {
		if err := failures.Append(failures.RegistryFile, verdict.FailureNotes()...); err != nil {
			return &adh.Error{Op: op, Err: err}
		}
	}
	recordDisposition(arc, verdict, disposition, maxReworks)
	arc.Findings = nil
	// Cleared with the findings: a gap declared by the review just disposed of must
	// not be read as a gap in the next one.
	arc.Unexamined = nil
	return nil
}

// recordDisposition moves the arc and writes what happened into its history.
//
// Requires: disposition is Decide's answer for verdict.
// Ensures: the arc's stage and status reflect the disposition, and its history records
// the reason in the tense a later reader needs — the arc outlives the command that
// disposed of it, and "why did this advance" is asked from the arc rather than from a
// terminal that has scrolled.
//
// Extracted from Apply when a fourth disposition pushed it past the complexity limit,
// and it is the better shape independently: this is the policy, and Apply is the
// bookkeeping around it.
func recordDisposition(
	arc *adh.Arc, verdict *critic.Verdict, disposition Disposition, maxReworks int,
) {
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
	case AdvanceWithReservation:
		arc.Stage = adh.StageOps
		// The reservation goes in the history rather than only in the report,
		// because the arc outlives the command that disposed of it and "what did
		// this cost" is asked later, by somebody reading the arc.
		arc.History = append(arc.History, fmt.Sprintf(
			"evaluation: no findings confirmed; advanced with %d reservation(s): %s",
			len(verdict.Reservations), strings.Join(verdict.ReservationNotes(), "; "),
		))
	}
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

// recordPrecision stamps the adjudication's confirmed and unconfirmed finding kinds
// into the critic precision log (§19) — the false-positive-rate evidence NoisyKinds
// reads to hold over-flagging kinds to a higher bar. A clean review adjudicates
// nothing, so AppendPrecision skips it and the log does not grow.
func recordPrecision(arc *adh.Arc, verdict *critic.Verdict) error {
	const op = "evaluation.recordPrecision"
	confirmed, unconfirmed := critic.VerdictKinds(verdict)
	entry := critic.PrecisionEntry{Arc: arc.ID, Confirmed: confirmed, Unconfirmed: unconfirmed}
	if err := critic.AppendPrecision(critic.PrecisionFile, &entry); err != nil {
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
	res, err := a.adjudicate(ctx, finding)
	return res.Ran, res.Failed, err
}

// AdjudicateWhy is Adjudicate, and also says why an artifact did not run (§19.2).
//
// Requires: nothing.
// Ensures: Unrunnable is meaningful only when Ran is false, and is
// UnrunnableToolFailed only for a **registered** tool that could not start — the one
// case that is evidence about the repository rather than about the finding.
//
// A second method rather than a widened Adjudicate, because Adjudicator is an
// interface with test implementations and this reason is the RepoAdjudicator's to
// know: a mock has no registry to have missed.
func (a *RepoAdjudicator) AdjudicateWhy(
	ctx context.Context,
	finding adh.Finding,
) (critic.Adjudicated, error) {
	res, err := a.adjudicate(ctx, finding)
	res.Finding = finding
	return res, err
}

func (a *RepoAdjudicator) adjudicate(
	ctx context.Context,
	finding adh.Finding,
) (critic.Adjudicated, error) {
	switch finding.Kind {
	case adh.FindingOracle:
		if res, ok := a.runDeclaredTool(ctx, finding.Ref); ok {
			return res, nil
		}
		// The built-in always runs, so a ref that named nothing declared is not an
		// unrunnable outcome here — it is a check that happened anyway.
		return critic.Adjudicated{Ran: true, Failed: a.builtinOracle()}, nil
	case adh.FindingInvariant:
		if res, ok := a.runDeclaredTool(ctx, finding.Ref); ok {
			return res, nil
		}
		return critic.Adjudicated{Ran: true, Failed: a.builtinInvariant()}, nil
	case adh.FindingDevice:
		if res, ok := a.runDeclaredTool(ctx, finding.Ref); ok {
			return res, nil
		}
		ran, failed, err := a.builtinDevice(ctx)
		return critic.Adjudicated{Ran: ran, Failed: failed}, err
	case adh.FindingContract:
		return a.adjudicateContract(finding.Ref), nil
	case adh.FindingNFR:
		return a.adjudicateNFR(ctx, finding.Ref), nil
	default:
		// ParseFindings validates Kind, so this is unreachable from a relayed reply;
		// a caller constructing a finding in Go can still get here.
		return critic.Adjudicated{Unrunnable: adh.UnrunnableUnknownRef}, nil
	}
}

// adjudicateOne asks the adjudicator for a result, taking the reason an artifact did
// not run when the adjudicator can supply one.
//
// The type switch is the seam between an interface with test implementations and the
// one implementation that has a registry to have missed. A mock cannot say *why* a
// tool was unrunnable because it has no tools; widening the interface would make every
// implementation answer a question only one of them can.
func adjudicateOne(
	ctx context.Context, adjudicator Adjudicator, finding adh.Finding,
) (critic.Adjudicated, error) {
	if why, ok := adjudicator.(interface {
		AdjudicateWhy(context.Context, adh.Finding) (critic.Adjudicated, error)
	}); ok {
		res, err := why.AdjudicateWhy(ctx, finding)
		if err != nil {
			return critic.Adjudicated{}, fmt.Errorf("adjudicate %s: %w", finding.Kind, err)
		}
		return res, nil
	}
	ran, failed, err := adjudicator.Adjudicate(ctx, finding)
	if err != nil {
		return critic.Adjudicated{}, fmt.Errorf("adjudicate %s: %w", finding.Kind, err)
	}
	return critic.Adjudicated{Finding: finding, Ran: ran, Failed: failed}, nil
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
func (a *RepoAdjudicator) runDeclaredTool(
	ctx context.Context, ref string,
) (critic.Adjudicated, bool) {
	if ref == "" || a.runner == nil {
		return critic.Adjudicated{}, false
	}
	tool, found := a.checks.FindByID(ref)
	if !found {
		return critic.Adjudicated{}, false
	}
	start := time.Now()
	passed, ranCheck := a.runner.RunCheck(ctx, tool.Run, a.repoRoot())
	// A registered tool that could not start is the trustworthy refusal: the
	// repository declared this check and the environment cannot perform it, which is
	// evidence about the repository rather than about the finding that named it.
	res := critic.Adjudicated{Ran: ranCheck, Failed: ranCheck && !passed}
	if !ranCheck {
		res.Unrunnable = adh.UnrunnableToolFailed
	}
	// Logged with the reason, so the accumulated record answers *why* the
	// deterministic path missed rather than only how often.
	a.logRun(tool.ID, ranCheck, ranCheck && !passed, time.Since(start), res.Unrunnable)
	return res, true
}

// logRun records a declared-tool adjudication run to the tool-run log (§16/§18) when
// logging is enabled (the real adjudicator; disabled for tests). Best-effort — a
// log-write failure never changes the adjudication result. The clock stays in this
// shell-side engine; the log stores only the duration and the opaque stratum.
func (a *RepoAdjudicator) logRun(
	id string, ran, failed bool, took time.Duration, why adh.Unrunnable,
) {
	if a.logPath == "" {
		return
	}
	_ = toolrun.AppendOutcome(
		a.logPath, id, contextstore.Stratum(time.Now()),
		ran, failed, int(took.Milliseconds()), string(why),
	)
}

// adjudicateNFR runs the executable check an NFR finding names (§10, §19.2). The
// finding's ref is a Planguage spec or a tool ID in the registry; the check's own
// command is run in the repository. A finding that names no check, names one the
// registry does not declare, or has no runner wired is unrunnable — an invented
// requirement the repository does not hold, which the gate drops as unconfirmed. A
// declared check that exits non-zero confirms the finding.
func (a *RepoAdjudicator) adjudicateNFR(ctx context.Context, ref string) critic.Adjudicated {
	if ref == "" || a.runner == nil {
		return critic.Adjudicated{Unrunnable: adh.UnrunnableNoRef}
	}
	// A ref that names a Planguage spec gates on the declarative Fail threshold: run
	// the spec's Meter tool, measure, and confirm when the value breaches Fail (§10.5)
	// — adh owns the threshold, not the tool. Otherwise ref is a tool id whose own
	// exit code is the pass/fail signal (backward compatible).
	if spec, ok := nfr.ByID(a.specs, ref); ok {
		return a.adjudicateSpec(ctx, &spec)
	}
	res, ok := a.runDeclaredTool(ctx, ref)
	if !ok {
		// Unlike the oracle kinds there is no built-in fallback here, so a ref naming
		// nothing declared ends the adjudication rather than being run anyway.
		return critic.Adjudicated{Unrunnable: adh.UnrunnableUnknownRef}
	}
	return res
}

// adjudicateSpec measures a Planguage spec's Meter and confirms the finding when the
// measured value breaches the spec's Fail bar (§10.5, §19.2). A spec whose Meter is
// not a declared §13 tool, or whose tool emits no parseable measurement, is
// unrunnable (unconfirmed) — the gate drops a requirement it cannot measure.
func (a *RepoAdjudicator) adjudicateSpec(ctx context.Context, spec *nfr.Spec) critic.Adjudicated {
	tool, ok := a.checks.FindByID(spec.Meter)
	if !ok {
		// The spec names a meter the registry does not declare. That is the
		// repository's own requirement pointing at a tool it did not provide, so it
		// is closer to a broken declaration than to a critic's invention — but the
		// tool was never registered, so it cannot be the tool-failed refusal either.
		return critic.Adjudicated{Unrunnable: adh.UnrunnableUnknownRef}
	}
	value, measured := a.runner.Measure(ctx, tool.Run, a.repoRoot())
	if !measured {
		// A registered meter that produced no measurement: the same refusal a
		// registered tool that could not start is.
		return critic.Adjudicated{Unrunnable: adh.UnrunnableToolFailed}
	}
	// The measurement travels rather than being reduced to a boolean here. A spec that
	// cleared its Fail bar and still moved the wrong way from Baseline is admissible
	// *and* cost something, and collapsing to Meets discarded the second fact.
	return critic.Adjudicated{
		Ran: true, Failed: !spec.Meets(value),
		Measured: value, HasMeasure: true, Regressed: spec.Regressed(value),
	}
}

// adjudicateContract verifies the proof manifest a contract finding names. A
// finding that names no manifest is unrunnable; a manifest that is missing,
// unreadable, or fails verification confirms the finding (the named proof does not
// hold).
func (a *RepoAdjudicator) adjudicateContract(ref string) critic.Adjudicated {
	if ref == "" {
		// The critic asserted a contract violation and named no packet. Noise rather
		// than a refusal: there was never anything to run.
		return critic.Adjudicated{Unrunnable: adh.UnrunnableNoRef}
	}
	pkt, err := proof.Load(ref)
	if err != nil {
		return critic.Adjudicated{Ran: true, Failed: true} // named proof missing = failed
	}
	return critic.Adjudicated{Ran: true, Failed: proof.Verify(a.repoRoot(), &pkt) != nil}
}

// repoRoot is the directory the checks run in, defaulting to the current directory
// so the zero-value adjudicator stays usable.
func (a *RepoAdjudicator) repoRoot() string {
	if a.dir == "" {
		return "."
	}
	return a.dir
}
