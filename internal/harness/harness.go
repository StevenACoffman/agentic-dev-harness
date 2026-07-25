// Package harness holds the self-optimization decisions (SPEC-ADDITIONS §18):
// classify a miss as a harness defect or an execution lapse (§18.3), and accept
// a candidate only on a strict held-out improvement via the gate ratchet
// (§18.2). It composes internal/gate as a leaf; it performs no effect.
package harness

import (
	"fmt"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/gate"
	"github.com/StevenACoffman/agentic-dev-harness/internal/judge"
	"github.com/StevenACoffman/agentic-dev-harness/internal/rubric"
)

// Disposition values classify a miss.
const (
	Defect Disposition = "defect" // the artifact is wrong/missing → edit it
	Lapse  Disposition = "lapse"  // a correct rule was not followed → do not churn it
)

// Disposition is how the reflect step treats a miss.
type Disposition string

// EvalReport is the outcome of scoring a harness guiding artifact (§18.2): the
// rubric's deterministic floor and judge-only dimensions, the recommendation of
// what to fix first, and an optional behavioral rule-judge result. Behavioral is
// nil when no checks were supplied.
type EvalReport struct {
	Rubric     rubric.Report `json:"rubric"`
	Diagnosis  string        `json:"diagnosis"`
	Behavioral *judge.Result `json:"behavioral,omitempty"`
}

// Classify decides whether a miss is a harness defect or an execution lapse
// (§18.3). When a correct rule already exists but was not followed, it is a
// lapse; the artifact is protected. When uncertain, callers pass ruleExists as
// false only if the rule is genuinely absent — otherwise treat it as a lapse.
func Classify(ruleExists bool) Disposition {
	if ruleExists {
		return Lapse
	}
	return Defect
}

// Accept applies the comparative ratchet on a held-out score (§18.2): a
// candidate is kept only if it strictly beats the current held-out score and
// becomes the best only if it also beats the best. bestStep/globalStep are 0
// for a single-shot consolidation.
func Accept(candidate, current, best float64) gate.Result {
	return gate.Evaluate(candidate, current, best, 0, 0)
}

// Eval scores a harness guiding artifact: the rubric's deterministic floor and
// judge-only dimensions (§18.2), the fix-this-first diagnosis, and — when checks
// are supplied — a behavioral rule-judge pass over output (§11). It is pure; the
// command shell reads the artifact and checks and passes them in.
func Eval(doc, output string, checks []judge.Check) (EvalReport, error) {
	report := rubric.Evaluate(doc)
	result := EvalReport{Rubric: report, Diagnosis: rubric.Diagnose(report)}
	if len(checks) > 0 {
		behavioral, err := judge.Score(output, checks)
		if err != nil {
			return EvalReport{}, fmt.Errorf("harness.Eval: %w", err)
		}
		result.Behavioral = &behavioral
	}
	return result, nil
}

// SelfTest is the negative control for the self-optimization gate
// (SPEC-ADDITIONS §18.4), mirroring SkillOpt's planted-harmful-edit probe: a
// candidate that does not strictly beat the held-out baseline must be rejected.
// It feeds a regression and a tie through the ratchet and returns EINTERNAL if
// either is accepted — proof the gate has teeth before the loop is trusted.
func SelfTest() error {
	const current, best = 0.7, 0.7
	for _, planted := range []float64{0.6, current} { // a regression and a tie: neither improves
		if Accept(planted, current, best).Action != gate.Reject {
			return &adh.Error{
				Code:    adh.EINTERNAL,
				Message: "gate self-test: the ratchet accepted a non-improving candidate",
			}
		}
	}
	return nil
}
