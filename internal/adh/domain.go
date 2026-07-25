package adh

// Stage values in canonical pipeline order (SPEC §1).
const (
	StageStrategy   Stage = "strategy"
	StageExecution  Stage = "execution"
	StageCritic     Stage = "critic"
	StageEvaluation Stage = "evaluation"
	StageOps        Stage = "ops"
)

// Status values.
const (
	StatusOpen    Status = "open"    // moving through the loop
	StatusBlocked Status = "blocked" // waiting at a human gate
	StatusClosed  Status = "closed"  // finished with proof
	StatusFailed  Status = "failed"  // returned to execution by a failed check
)

// Resolution values (SPEC-ADDITIONS §12); each carries its own proof contract.
const (
	ResolutionChange        Resolution = "change"        // a merged/deployed code change
	ResolutionInvestigation Resolution = "investigation" // an analysis or answer, no code
	ResolutionExperiment    Resolution = "experiment"    // an instrumented painted-door surface
	ResolutionDecision      Resolution = "decision"      // a recorded decision, often "do not build"
)

// Stage is one station of the five-stage arc loop.
type Stage string

// Status is an arc's lifecycle status.
type Status string

// Resolution is how an arc closes; a non-change arc need not run the build stages.
type Resolution string

// Arc is a unit of work driven through the loop.
type Arc struct {
	ID         string     `json:"id"`
	Title      string     `json:"title"`
	Stage      Stage      `json:"stage"`
	Status     Status     `json:"status"`
	Resolution Resolution `json:"resolution,omitempty"`
	History    []string   `json:"history,omitempty"`
}

// NextStage returns the stage after s in the change pipeline and whether one
// exists. Ops is terminal, so NextStage(StageOps) reports ("", false).
func NextStage(s Stage) (Stage, bool) {
	switch s {
	case StageStrategy:
		return StageExecution, true
	case StageExecution:
		return StageCritic, true
	case StageCritic:
		return StageEvaluation, true
	case StageEvaluation:
		return StageOps, true
	case StageOps:
		return "", false
	default:
		return "", false
	}
}

// ProofKind names the evidence a resolution must carry to close (SPEC-ADDITIONS
// §12). An empty return means the resolution is unset or unknown.
func (r Resolution) ProofKind() string {
	switch r {
	case ResolutionChange:
		return "oracle, invariant, and device proof"
	case ResolutionInvestigation:
		return "the sources inspected and the reproducible finding"
	case ResolutionExperiment:
		return "the instrumentation and the readout that answers the question"
	case ResolutionDecision:
		return "the evidence and the rationale behind the call"
	default:
		return ""
	}
}

// ParseResolution validates a resolution string, returning the typed value or
// EINVALID for an unknown one. The known resolutions are exactly the four that
// carry a proof contract (ProofKind).
func ParseResolution(s string) (Resolution, error) {
	res := Resolution(s)
	if res.ProofKind() == "" {
		return "", &Error{Code: EINVALID, Message: "unknown resolution: " + s}
	}
	return res, nil
}

// CanClose reports whether an arc may close under NO-PROOF-NO-CLOSE (SPEC §5.4):
// an arc closes only with resolution-matched proof present. It returns EINVALID
// when the resolution is unset or unknown and ECONFLICT when proof is missing.
func CanClose(arc *Arc, hasProof bool) error {
	kind := arc.Resolution.ProofKind()
	if kind == "" {
		return &Error{Code: EINVALID, Message: "arc has no valid resolution"}
	}
	if !hasProof {
		return &Error{
			Code:    ECONFLICT,
			Message: "no proof: " + string(arc.Resolution) + " requires " + kind,
		}
	}
	return nil
}
