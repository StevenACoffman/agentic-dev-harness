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

// FindingKind values name the repository artifact whose run would confirm a
// critic finding (SPEC-ADDITIONS §19.2). The Evaluation stage runs that artifact;
// a finding is never trusted on the critic's text alone.
const (
	FindingOracle    FindingKind = "oracle"    // a differential-oracle divergence
	FindingInvariant FindingKind = "invariant" // a property/invariant check
	FindingDevice    FindingKind = "device"    // an on-device (adb) check (§2.4)
	FindingNFR       FindingKind = "nfr"       // a nonfunctional-requirement check (§10)
	FindingContract  FindingKind = "contract"  // a named local contract (e.g. a proof packet)
)

// Stage is one station of the five-stage arc loop.
type Stage string

// Status is an arc's lifecycle status.
type Status string

// Resolution is how an arc closes; a non-change arc need not run the build stages.
type Resolution string

// FindingKind is the kind of repository artifact a critic finding names (§19.2).
type FindingKind string

// Finding is a critic's hypothesis about the change under review (§19.2): a
// summary and the repository artifact (Kind + Ref) whose run would confirm it.
// The Evaluation stage adjudicates it; the harness never blocks on its text.
type Finding struct {
	Summary string      `json:"summary"`
	Kind    FindingKind `json:"kind"`
	Ref     string      `json:"ref,omitempty"`
}

// Arc is a unit of work driven through the loop.
type Arc struct {
	ID         string     `json:"id"`
	Title      string     `json:"title"`
	Stage      Stage      `json:"stage"`
	Status     Status     `json:"status"`
	Resolution Resolution `json:"resolution,omitempty"`
	History    []string   `json:"history,omitempty"`
	Pending    *Pending   `json:"pending,omitempty"`
	// Labels and Paths are the arc's routing footprint (SPEC-ADDITIONS §10, §19.1):
	// its labels and the repository paths it touches. They ground the cold critic
	// by selecting the context units routed to the review; an arc that declares
	// them but routes nothing records a routing gap (§19.1).
	Labels []string `json:"labels,omitempty"`
	Paths  []string `json:"paths,omitempty"`
	// Proof is the repository-relative path to the proof packet manifest the
	// builder left (SPEC §5.4). Empty until Execution records one; the critic
	// reviews against it when present (§19.1).
	Proof string `json:"proof,omitempty"`
	// Findings are the cold critic's hypotheses awaiting adjudication by the
	// Evaluation stage (§19.2). Set when the critic turn resumes; cleared once
	// Evaluation has disposed of them.
	Findings []Finding `json:"findings,omitempty"`
}

// Pending is an outstanding model turn: a stage's prompt emitted to an operator
// whose reply is supplied out-of-band (the relay, SPEC §1-2), awaiting a resume.
// It is nil unless the arc is parked mid-turn between an emit and its response.
type Pending struct {
	Stage  Stage  `json:"stage"`
	Prompt string `json:"prompt"`
}

// Valid reports whether k is a known finding kind (§19.2). A validated kind lets
// callers switch on it without a default-panic path.
func (k FindingKind) Valid() bool {
	switch k {
	case FindingOracle, FindingInvariant, FindingDevice, FindingNFR, FindingContract:
		return true
	default:
		return false
	}
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
