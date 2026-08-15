// Package rubric scores a harness guiding artifact against adh's own five-dimension
// rubric (SPEC-ADDITIONS §18.2) — distinct from the external skillsaw 9-dimension rubric
// adh consumes at its gate. It adapts skillsaw's rubric *pattern*: a deterministic floor
// (DetScore) that assumes a perfect base for judge-only dimensions and docks only
// detectable defects, an explicit list of dimensions that still need a model, and
// Diagnose naming the weakest deterministic dimension.
//
// Three of the five dimensions — failure-handling, actionable-specificity, and
// boundary-section — are the microsoft/SkillLens quality dimensions (arXiv:2605.23899),
// each with published tests and anti-examples. Their deterministic detectors are
// skillet/skilllens (FailureMechanisms, BlacklistSections), which skillsaw scores too, so
// the two tools cannot disagree about whether an artifact encodes a failure mode. adh's
// weights, its 0..1 Deterministic factor, and its five-dimension set stay here: they grade
// a harness artifact, not a SKILL.md, which is why these three are worth 60 here and 35 in
// skillsaw. actionable-specificity is NeedsJudge, so its base is a model's to supply,
// against SkillLens's stated definition.
//
// Consuming skilllens means this package now parses Markdown (via skillet/markdown); it
// still runs no runtime-neutrality scan, which stays skillsaw-specific.
package rubric

import (
	"strings"

	"github.com/StevenACoffman/skillet/markdown"
	"github.com/StevenACoffman/skillet/skilllens"
)

// Dimension keys.
const (
	KeyOutcome      = "outcome-clarity"
	KeyFailure      = "failure-handling"
	KeySpecificity  = "actionable-specificity"
	KeyBoundary     = "boundary-section"
	KeyArchitecture = "architecture"
)

// Dimension is one rubric axis. NeedsJudge marks a dimension whose base quality
// is an irreducible textual judgment a model must supply.
type Dimension struct {
	Key        string `json:"key"`
	Name       string `json:"name"`
	Weight     int    `json:"weight"`
	NeedsJudge bool   `json:"needs_judge"`
}

// DimScore is a dimension's outcome: a 0..1 deterministic factor (1.0 = no
// detectable defect) and the reason.
type DimScore struct {
	Dimension
	Deterministic float64 `json:"deterministic"`
	Reason        string  `json:"reason"`
}

// Report is the rubric outcome: per-dimension scores, the weighted deterministic
// total (judge dims assumed perfect), and which dimensions still need a model.
type Report struct {
	Dims       []DimScore `json:"dims"`
	DetScore   float64    `json:"det_score"`
	NeedsJudge []string   `json:"needs_judge"`
}

// Evaluate scores doc. Judge dimensions assume a perfect base for the floor;
// deterministic dimensions dock detectable defects. DetScore is the weighted
// total in [0, 100].
func Evaluate(doc string) Report {
	md := markdown.Parse(doc)
	var rep Report
	for _, dim := range dimensions() {
		factor, reason := 1.0, "assumed perfect (needs a judge)"
		if dim.NeedsJudge {
			rep.NeedsJudge = append(rep.NeedsJudge, dim.Key)
		} else {
			factor, reason = deterministicScore(dim.Key, md)
		}
		rep.Dims = append(rep.Dims, DimScore{Dimension: dim, Deterministic: factor, Reason: reason})
		rep.DetScore += float64(dim.Weight) * factor
	}
	return rep
}

// Diagnose names the highest-weight deterministic dimension scored below full,
// or reports that only judgment remains.
func Diagnose(rep Report) string {
	worst, worstWeight := "", -1
	for i := range rep.Dims {
		dim := &rep.Dims[i]
		if !dim.NeedsJudge && dim.Deterministic < 1.0 && dim.Weight > worstWeight {
			worst, worstWeight = dim.Key, dim.Weight
		}
	}
	if worst == "" {
		return "no deterministic weakness — score the judge dimensions: " + strings.Join(
			rep.NeedsJudge,
			", ",
		)
	}
	return "fix " + worst + " first"
}

// dimensions returns the fixed axes (weights sum to 100). A function, not a
// package variable, to keep the package free of mutable global state.
func dimensions() []Dimension {
	return []Dimension{
		{Key: KeyOutcome, Name: "Outcome clarity", Weight: 25, NeedsJudge: true},
		{Key: KeyFailure, Name: "Failure handling", Weight: 20},
		{Key: KeySpecificity, Name: "Actionable specificity", Weight: 25, NeedsJudge: true},
		{Key: KeyBoundary, Name: "Boundary section", Weight: 15},
		{Key: KeyArchitecture, Name: "Overall architecture", Weight: 15, NeedsJudge: true},
	}
}

func deterministicScore(key string, md *markdown.Doc) (float64, string) {
	switch key {
	case KeyFailure:
		// Only KindProse -- an actual "if X fails, do Y" branch -- is evidence that a
		// failure mechanism is encoded. KindSection is a heading whose *title* matched, and
		// a heading with nothing under it encodes nothing; counting the two alike let a bare
		// "## Boundary" earn this dimension in full.
		for _, s := range skilllens.FailureMechanisms(md) {
			if s.Kind == skilllens.KindProse {
				return 1.0, "failure branch present"
			}
		}
		// Absent branches, whether that is a defect depends on the artifact. One that runs
		// commands and never says what to do when they fail is exactly what this dimension
		// exists to catch. One that executes nothing -- a selection or judgement document --
		// has no runtime failure to encode, and docking it would be a category error.
		//
		// This stays binary rather than scoring a section-only artifact somewhere in
		// between: no partial credit has been calibrated, and inventing one would put a
		// number nobody can defend into 20% of the total.
		if !md.HasCodeBlock {
			return 1.0, "no failure branch, and the artifact executes nothing to fail"
		}
		return 0.0, "the artifact executes commands but encodes no failure branch"
	case KeyBoundary:
		if len(skilllens.BlacklistSections(md)) > 0 {
			return 1.0, "boundary / counter-example section present"
		}
		return 0.0, "no boundary / counter-example section"
	default:
		return 1.0, "no deterministic check"
	}
}
