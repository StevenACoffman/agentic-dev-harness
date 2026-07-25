// Package authority holds the pure authority decisions of the harness: the
// autonomy ladder (SPEC §6), the model-gate (SPEC §5.1), and the human safety
// gate (SPEC §5.2). Capability and authority are separate contracts; this
// package decides authority and never performs an effect.
package authority

import "github.com/StevenACoffman/agentic-dev-harness/internal/adh"

// Level values of the autonomy ladder, lowest to highest (SPEC §6). Higher
// levels auto-launch more safe steps (remove clicks) but never remove a gate.
const (
	L0 Level = iota // manual: every stage transition is explicit
	L1              // assisted: suggest the next stage, human confirms
	L2              // hands-off relay up to the next gate (default)
	L3              // auto-advance safe launches; gates still stop the loop
	L4              // lights-out between gates
)

// ModelClass values (SPEC §5.1).
const (
	ClassReasoning ModelClass = "reasoning" // strong, cold; judgment roles
	ClassFast      ModelClass = "fast"      // routine execution
)

// Level is a rung of the autonomy ladder.
type Level int

// ModelClass is the capability tier a stage's model belongs to.
type ModelClass string

// JudgmentRoles is the set of stages that must run on a reasoning-class model
// (SPEC §5.1). It is config-driven so a deployment can widen or narrow the set.
type JudgmentRoles map[adh.Stage]bool

// String renders the ladder rung as "L0".."L4".
func (l Level) String() string {
	switch l {
	case L0:
		return "L0"
	case L1:
		return "L1"
	case L2:
		return "L2"
	case L3:
		return "L3"
	case L4:
		return "L4"
	default:
		return "L?"
	}
}

// ParseLevel parses "L0".."L4" (case-sensitive). It returns EINVALID otherwise.
func ParseLevel(s string) (Level, error) {
	switch s {
	case "L0":
		return L0, nil
	case "L1":
		return L1, nil
	case "L2":
		return L2, nil
	case "L3":
		return L3, nil
	case "L4":
		return L4, nil
	default:
		return L0, &adh.Error{Code: adh.EINVALID, Message: "unknown autonomy level: " + s}
	}
}

// RaiseIsGated reports whether moving from cur to next is itself a human-gated
// action (SPEC §6: raising the level is gated; lowering is always allowed).
func RaiseIsGated(cur, next Level) bool { return next > cur }

// Requires reports whether role must run on a reasoning-class model.
func (j JudgmentRoles) Requires(role adh.Stage) bool { return j[role] }

// DefaultJudgmentRoles is the built-in judgment set (SPEC §5.1), derived from
// JudgmentRole so the default has a single source of truth.
func DefaultJudgmentRoles() JudgmentRoles {
	roles := make(JudgmentRoles)
	for _, role := range []adh.Stage{
		adh.StageStrategy, adh.StageExecution, adh.StageCritic,
		adh.StageEvaluation, adh.StageOps,
	} {
		if JudgmentRole(role) {
			roles[role] = true
		}
	}
	return roles
}

// JudgmentRole reports whether a stage must run on a reasoning-class model.
// Strategy, Critic, and Evaluation are judgment roles; Execution and Ops are not.
func JudgmentRole(role adh.Stage) bool {
	switch role {
	case adh.StageStrategy, adh.StageCritic, adh.StageEvaluation:
		return true
	case adh.StageExecution, adh.StageOps:
		return false
	default:
		return false
	}
}

// ModelGate enforces SPEC §5.1: a role in the judgment set must run on a
// reasoning-class model. The set is supplied by the caller (config-driven), so a
// deployment can widen or narrow it. Returns EUNAUTHORIZED when a judgment role
// would run on a fast model.
func ModelGate(role adh.Stage, class ModelClass, judgment JudgmentRoles) error {
	if judgment.Requires(role) && class != ClassReasoning {
		return &adh.Error{
			Code:    adh.EUNAUTHORIZED,
			Message: "judgment role " + string(role) + " must run on a reasoning-class model",
		}
	}
	return nil
}

// RequiredApprovalPhrase is the phrase a human must type to satisfy an arc's
// safety gate (SPEC §5.2), derived from the arc id. It is deliberately NOT
// sourced from config or the environment: a config- or env-settable phrase would
// be a self-grant route, since the agent can read both. The gate is structural.
func RequiredApprovalPhrase(arcID string) string { return arcID }

// GateSatisfied reports whether a human safety gate (SPEC §5.2) is satisfied.
// Only a non-empty phrase that exactly matches the required phrase satisfies it,
// and never under a dry run. The agent has no code route to self-grant: callers
// must never source provided from an environment variable or a --yes flag — the
// phrase comes only from an interactive --phrase typed by a human.
func GateSatisfied(required, provided string, dryRun bool) bool {
	if dryRun {
		return false
	}
	return required != "" && provided == required
}
