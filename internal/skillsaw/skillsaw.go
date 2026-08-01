// Package skillsaw decodes the eval JSON the external skillsaw CLI emits (SPEC-
// ADDITIONS §18, Loops B/C), so adh can feed skillsaw's score into its own strict-`>`
// ratchet (`harness gate`) — skillsaw as the cheap floor under adh's Evaluation bar,
// never a replacement for human approval or proof (§18.2). It is a parse-at-the-
// boundary decoder of the gate-relevant subset of skillsaw's `internal/rubric`
// Evaluation, tolerant of the fields adh does not read. It invokes nothing — the
// worker runs `adh tool run skillsaw-eval` (Loop B) and pipes the JSON here.
//
// The field names track skillsaw's real `eval --json` output (rubric.Evaluation /
// rubric.DimScore): the score is `deterministic_score` (or `full_score` once a judge
// has scored the judge-only dimensions), and dimensions are `dims` with an integer
// 1..10 `final` — not the {score, dimensions} shape a first cut might assume.
package skillsaw

import (
	"encoding/json"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
)

// Dimension is one rubric axis skillsaw scored: its number and name, its integer
// 1..10 final score, and whether its base quality still needs a model's judgment
// (the part adh relays, §18.2).
type Dimension struct {
	Num        int    `json:"num"`
	Name       string `json:"name"`
	Final      int    `json:"final"`
	NeedsJudge bool   `json:"needs_judge"`
}

// Eval is the gate-relevant subset of skillsaw's `eval --json` output adh consumes:
// the deterministic floor, the full score (present once the judge-only dimensions are
// scored), and the per-dimension results (with the needs-judge flags the worker
// adjudicates). Fields adh does not read (hash, bytes, weights, flags) are ignored.
type Eval struct {
	Skill              string      `json:"skill"`
	Dims               []Dimension `json:"dims"`
	DeterministicScore float64     `json:"deterministic_score"`
	FullScore          float64     `json:"full_score,omitempty"`
	HasFullScore       bool        `json:"has_full_score"`
}

// Score is the gate-relevant score fed to the ratchet: the full score once a judge
// has scored the judge-only dimensions, else the deterministic floor.
func (e Eval) Score() float64 {
	if e.HasFullScore {
		return e.FullScore
	}
	return e.DeterministicScore
}

// NeedsJudge returns the names of the dimensions skillsaw flagged as still needing a
// model's judgment — what the worker adjudicates before trusting the full score.
func (e Eval) NeedsJudge() []string {
	names := make([]string, 0, len(e.Dims))
	for i := range e.Dims {
		if e.Dims[i].NeedsJudge {
			names = append(names, e.Dims[i].Name)
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
