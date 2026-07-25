// Package rubric scores a harness guiding artifact against adh's own dimensions
// (SPEC-ADDITIONS §18.2), adapting skillsaw's rubric pattern: a deterministic
// floor (DetScore) that assumes a perfect base for judge-only dimensions and
// docks only detectable defects, an explicit list of dimensions that still need
// a model, and Diagnose naming the weakest deterministic dimension. It owns no
// markdown parser and no runtime-neutrality scan — those are skillsaw-specific.
package rubric

import "strings"

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
	Key        string
	Name       string
	Weight     int
	NeedsJudge bool
}

// DimScore is a dimension's outcome: a 0..1 deterministic factor (1.0 = no
// detectable defect) and the reason.
type DimScore struct {
	Dimension
	Deterministic float64
	Reason        string
}

// Report is the rubric outcome: per-dimension scores, the weighted deterministic
// total (judge dims assumed perfect), and which dimensions still need a model.
type Report struct {
	Dims       []DimScore
	DetScore   float64
	NeedsJudge []string
}

// Evaluate scores doc. Judge dimensions assume a perfect base for the floor;
// deterministic dimensions dock detectable defects. DetScore is the weighted
// total in [0, 100].
func Evaluate(doc string) Report {
	var rep Report
	for _, dim := range dimensions() {
		factor, reason := 1.0, "assumed perfect (needs a judge)"
		if dim.NeedsJudge {
			rep.NeedsJudge = append(rep.NeedsJudge, dim.Key)
		} else {
			factor, reason = deterministicScore(dim.Key, doc)
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

func deterministicScore(key, doc string) (float64, string) {
	switch key {
	case KeyFailure:
		if hasFailureHandling(doc) {
			return 1.0, "failure branch or Failures/Boundary section present"
		}
		return 0.0, "no failure branch and no Failures/Boundary section"
	case KeyBoundary:
		if hasHeading(
			doc,
			[]string{
				"boundary",
				"anti-pattern",
				"do not use",
				"quality red line",
				"common failures",
			},
		) {
			return 1.0, "boundary/anti-pattern section present"
		}
		return 0.0, "no Boundary/Anti-pattern/Do-Not-Use section"
	default:
		return 1.0, "no deterministic check"
	}
}

func hasFailureHandling(doc string) bool {
	if hasHeading(doc, []string{"failure", "failures", "boundary", "common failures"}) {
		return true
	}
	for _, line := range strings.Split(doc, "\n") {
		low := strings.ToLower(line)
		conditional := strings.Contains(low, "if ") || strings.Contains(low, "when ")
		failure := strings.Contains(low, "fail") || strings.Contains(low, "error") ||
			strings.Contains(low, "timeout") || strings.Contains(low, "missing")
		if conditional && failure {
			return true
		}
	}
	return false
}

func hasHeading(doc string, wants []string) bool {
	for _, line := range strings.Split(doc, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "#") {
			continue
		}
		low := strings.ToLower(trimmed)
		for _, want := range wants {
			if strings.Contains(low, want) {
				return true
			}
		}
	}
	return false
}
