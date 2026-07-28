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
	"fmt"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/critic"
	"github.com/StevenACoffman/agentic-dev-harness/internal/device"
	"github.com/StevenACoffman/agentic-dev-harness/internal/failures"
	"github.com/StevenACoffman/agentic-dev-harness/internal/oracle"
	"github.com/StevenACoffman/agentic-dev-harness/internal/proof"
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
// whether it could be run and whether it failed (§19.2). It is the point-of-use
// seam: RepoAdjudicator is the real one; tests inject a fake.
type Adjudicator interface {
	Adjudicate(ctx context.Context, finding adh.Finding) (ran, failed bool, err error)
}

// RepoAdjudicator runs the concrete repository artifacts a finding can name. The
// oracle/invariant/device runs are self-contained checks; a contract finding names
// a proof manifest and is verified against it.
type RepoAdjudicator struct{}

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

// Apply records the verdict and moves the arc on (§19.2): confirmed findings are
// appended to the failure registry and return the arc to Execution; unconfirmed
// ones become lesson candidates (when recordLessons is set) and the arc advances
// to the ops gate. It clears the arc's findings. It mutates arc and writes the
// registries; it neither saves the arc nor sets an exit code — those stay with the
// caller.
func Apply(arc *adh.Arc, verdict *critic.Verdict, recordLessons bool) error {
	const op = "evaluation.Apply"
	if recordLessons {
		if err := failures.Append(failures.CandidatesFile, verdict.LessonNotes()...); err != nil {
			return &adh.Error{Op: op, Err: err}
		}
	}
	if verdict.ReturnsToExecution() {
		if err := failures.Append(failures.RegistryFile, verdict.FailureNotes()...); err != nil {
			return &adh.Error{Op: op, Err: err}
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
		return nil
	}
	arc.Stage = adh.StageOps
	arc.History = append(
		arc.History,
		fmt.Sprintf(
			"evaluation: no findings confirmed; %d lesson candidate(s)",
			len(verdict.Unconfirmed),
		),
	)
	arc.Findings = nil
	return nil
}

// Adjudicate runs the artifact finding names. oracle/invariant/device are
// self-contained checks; a contract finding names a proof manifest and fails when
// the packet is missing or does not verify. An NFR finding has no runner yet, so
// it is unrunnable (ran=false) and disposes as unconfirmed (§19.2).
func (RepoAdjudicator) Adjudicate(
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
// unreadable, or fails verification confirms the finding (the named proof does not
// hold).
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
