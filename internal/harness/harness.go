// Package harness holds the self-optimization decisions (SPEC-ADDITIONS §18):
// classify a miss as a harness defect or an execution lapse (§18.3), and accept
// a candidate only on a strict held-out improvement via the gate ratchet
// (§18.2). It composes internal/gate as a leaf; it performs no effect.
package harness

import "github.com/StevenACoffman/agentic-dev-harness/internal/gate"

// Disposition values classify a miss.
const (
	Defect Disposition = "defect" // the artifact is wrong/missing → edit it
	Lapse  Disposition = "lapse"  // a correct rule was not followed → do not churn it
)

// Disposition is how the reflect step treats a miss.
type Disposition string

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
