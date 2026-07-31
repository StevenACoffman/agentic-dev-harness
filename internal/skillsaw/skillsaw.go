// Package skillsaw decodes the eval JSON the external skillsaw CLI emits (SPEC-
// ADDITIONS §18, Loops B/C), so adh can feed skillsaw's score into its own strict-`>`
// ratchet (`harness gate`) — skillsaw as the cheap floor under adh's Evaluation bar,
// never a replacement for human approval or proof (§18.2). It is a parse-at-the-
// boundary decoder of adh's documented consumption contract: the gate-relevant subset,
// tolerant of the fields adh does not read. It invokes nothing — the worker runs
// `adh tool run skillsaw-eval` (Loop B) and pipes the JSON here.
package skillsaw

import (
	"encoding/json"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
)

// Dimension is one rubric axis skillsaw scored: its name, its 0..1 score, and whether
// its base quality still needs a model's judgment (the part adh relays, §18.2).
type Dimension struct {
	Name       string  `json:"name"`
	Score      float64 `json:"score"`
	NeedsJudge bool    `json:"needs_judge"`
}

// Eval is the gate-relevant subset of skillsaw's `eval --json` output adh consumes:
// the overall score fed to the ratchet and the per-dimension scores (with the
// needs-judge flags the worker adjudicates). Fields adh does not read are ignored —
// this is adh's contract, not skillsaw's whole schema.
type Eval struct {
	Score      float64     `json:"score"`
	Dimensions []Dimension `json:"dimensions,omitempty"`
}

// NeedsJudge returns the names of the dimensions skillsaw flagged as still needing a
// model's judgment — what the worker adjudicates before trusting the score.
func (e Eval) NeedsJudge() []string {
	names := make([]string, 0, len(e.Dimensions))
	for i := range e.Dimensions {
		if e.Dimensions[i].NeedsJudge {
			names = append(names, e.Dimensions[i].Name)
		}
	}
	return names
}

// Decode parses skillsaw eval JSON into the subset adh consumes. Malformed JSON is
// EINVALID — the boundary rejects a reply it cannot trust.
func Decode(data []byte) (Eval, error) {
	var eval Eval
	if err := json.Unmarshal(data, &eval); err != nil {
		return Eval{}, &adh.Error{
			Code:    adh.EINVALID,
			Message: "skillsaw: decode eval: " + err.Error(),
		}
	}
	return eval, nil
}
