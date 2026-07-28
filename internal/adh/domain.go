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
	StatusFailed  Status = "failed"  // failed evaluation past its rework budget; terminal, awaiting a human
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
	// Context is the IDs of the context units the last stage loaded as its working
	// set (SPEC-ADDITIONS §10.3), recorded so a self-eval can tell a missed
	// requirement that was never routed (a context gap) from one routed and ignored.
	Context []string `json:"context,omitempty"`
	// Proof is the repository-relative path to the proof packet manifest the
	// builder left (SPEC §5.4). Empty until Execution records one; the critic
	// reviews against it when present (§19.1).
	Proof string `json:"proof,omitempty"`
	// Findings are the cold critic's hypotheses awaiting adjudication by the
	// Evaluation stage (§19.2). Set when the critic turn resumes; cleared once
	// Evaluation has disposed of them.
	Findings []Finding `json:"findings,omitempty"`
	// Reworks counts the times Evaluation confirmed a finding and returned this arc
	// to Execution (SPEC §4.1). It bounds the rework loop: once it reaches the
	// evaluation budget the arc fails terminally (StatusFailed) rather than looping.
	Reworks int `json:"reworks,omitempty"`
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

// Valid reports whether r is a known resolution (SPEC-ADDITIONS §12). Validity is
// a domain fact; the *acceptance-bar text* a resolution requires is deployment
// policy (config `[proof.contract]`, overriding the ProofKind default), not the
// domain's concern.
func (r Resolution) Valid() bool {
	switch r {
	case ResolutionChange, ResolutionInvestigation, ResolutionExperiment, ResolutionDecision:
		return true
	default:
		return false
	}
}

// ProofKind is the built-in default acceptance-bar text a resolution's proof must
// satisfy to close (SPEC-ADDITIONS §12). It is generic — the `change` bar is
// code-level, not domain-specific — and a deployment overrides it per resolution
// via config `[proof.contract]` (§SPEC 3.1). An unknown resolution has no text.
func (r Resolution) ProofKind() string {
	switch r {
	case ResolutionChange:
		return "the change's tests pass and its review/CI checks are green"
	case ResolutionInvestigation:
		return "the sources inspected and the reproducible finding"
	case ResolutionExperiment:
		return "the instrumentation and the readout that answers the product question"
	case ResolutionDecision:
		return "the evidence and the rationale behind the call"
	default:
		return ""
	}
}

// ParseResolution validates a resolution string, returning the typed value or
// EINVALID for an unknown one.
func ParseResolution(s string) (Resolution, error) {
	res := Resolution(s)
	if !res.Valid() {
		return "", &Error{Code: EINVALID, Message: "unknown resolution: " + s}
	}
	return res, nil
}

// CanClose reports whether an arc may close under NO-PROOF-NO-CLOSE (SPEC §5.4):
// an arc closes only with resolution-matched proof present. It returns EINVALID
// when the resolution is unset or unknown and ECONFLICT when proof is missing.
func CanClose(arc *Arc, hasProof bool) error {
	if !arc.Resolution.Valid() {
		return &Error{Code: EINVALID, Message: "arc has no valid resolution"}
	}
	if !hasProof {
		return &Error{
			Code:    ECONFLICT,
			Message: "no proof: " + string(arc.Resolution) + " requires matching, verified proof",
		}
	}
	return nil
}
