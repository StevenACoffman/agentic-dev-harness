// Package gate is the comparative validation ratchet at the heart of harness
// self-optimization (SPEC-ADDITIONS §18.2), a port of SkillOpt's pure
// evaluate_gate decision. A candidate is accepted only if it strictly beats the
// current score; it becomes the new best only if it also strictly beats the best
// score. Ties reject and do not promote, so the ratchet is monotonic and noise
// never accumulates. The package is pure: value-in, value-out, no I/O.
package gate

import "fmt"

// Metric values. Mixed blends hard and soft with a caller-supplied weight.
const (
	Hard  Metric = "hard"
	Soft  Metric = "soft"
	Mixed Metric = "mixed"
)

// Action values are the possible gate outcomes.
const (
	AcceptNewBest Action = "accept_new_best"
	Accept        Action = "accept"
	Reject        Action = "reject"
)

// Metric selects how a (hard, soft) score pair projects onto one comparable
// number.
type Metric string

// Action is the gate outcome.
type Action string

// Result is the immutable gate outcome plus the resulting state. The command
// shell decides what to do with it (print, mutate state, set an exit code).
type Result struct {
	Action       Action  `json:"action"`
	CurrentScore float64 `json:"current_score"`
	BestScore    float64 `json:"best_score"`
	BestStep     int     `json:"best_step"`
}

// SelectScore projects a (hard, soft) pair onto a single metric. Mixed clamps
// weight into [0,1] and returns (1-weight)*hard + weight*soft. It returns an
// EINVALID-shaped error for an unknown metric.
func SelectScore(hard, soft float64, metric Metric, weight float64) (float64, error) {
	switch metric {
	case Hard:
		return hard, nil
	case Soft:
		return soft, nil
	case Mixed:
		if weight < 0 {
			weight = 0
		}
		if weight > 1 {
			weight = 1
		}
		return (1.0-weight)*hard + weight*soft, nil
	default:
		return 0, fmt.Errorf("unknown gate metric %q; want hard, soft, or mixed", metric)
	}
}

// Evaluate compares an already-projected candidate score against the current and
// best scores using strict ">" at both comparisons. globalStep is recorded as
// the new best step when a new best is accepted; otherwise bestStep is preserved.
func Evaluate(candidate, current, best float64, bestStep, globalStep int) Result {
	switch {
	case candidate > current && candidate > best:
		return Result{
			Action:       AcceptNewBest,
			CurrentScore: candidate,
			BestScore:    candidate,
			BestStep:     globalStep,
		}
	case candidate > current:
		return Result{
			Action:       Accept,
			CurrentScore: candidate,
			BestScore:    best,
			BestStep:     bestStep,
		}
	default:
		return Result{
			Action:       Reject,
			CurrentScore: current,
			BestScore:    best,
			BestStep:     bestStep,
		}
	}
}
