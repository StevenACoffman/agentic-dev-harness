package rubric_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/internal/rubric"
)

func TestEvaluateFullyStructured(t *testing.T) {
	doc := "# Skill\nDo the thing.\nIf the build fails, open an early PR.\n## Boundary\nNot for zero-to-one work.\n"
	rep := rubric.Evaluate(doc)
	if rep.DetScore != 100 {
		t.Errorf(
			"DetScore = %v, want 100 (both deterministic dims satisfied, judge dims assumed perfect)",
			rep.DetScore,
		)
	}
	if len(rep.NeedsJudge) != 3 {
		t.Errorf("NeedsJudge = %v, want 3 judge dimensions", rep.NeedsJudge)
	}
}

func TestEvaluateMissingSections(t *testing.T) {
	doc := "# Skill\nJust do the thing and ship it.\n"
	rep := rubric.Evaluate(doc)
	// failure-handling (20) and boundary-section (15) both dock to 0.
	if rep.DetScore != 65 {
		t.Errorf("DetScore = %v, want 65 (100 - 20 - 15)", rep.DetScore)
	}
}

// dimFactor returns the deterministic factor for one dimension key.
func dimFactor(rep rubric.Report, key string) float64 {
	for _, d := range rep.Dims {
		if d.Key == key {
			return d.Deterministic
		}
	}
	return -1
}

// TestFailureRejectsProseNotABranch pins the de-false-positive from consuming skilllens:
// "a stage fails when X" is prose, not an "if X fails" branch. adh's old loose line-scan
// (any line with "when"/"if" and a failure word) counted it; skilllens's bounded regex
// does not, and there is no failure-mode section here.
func TestFailureRejectsProseNotABranch(t *testing.T) {
	doc := "# Skill\n\nDo the thing. A stage fails when the routed input is malformed.\n"
	if f := dimFactor(rubric.Evaluate(doc), rubric.KeyFailure); f != 0 {
		t.Errorf("prose 'fails when …' must not satisfy failure-handling; factor = %v", f)
	}
}

// TestBoundaryRecognizesRicherVocab pins the convergence: "## Pitfalls" is a boundary /
// counter-example section skilllens recognizes but adh's old heading set
// ({boundary, anti-pattern, do not use, quality red line, common failures}) missed.
func TestBoundaryRecognizesRicherVocab(t *testing.T) {
	doc := "# Skill\n\ntext\n\n## Pitfalls\n\n- do not do this\n- or this\n"
	if f := dimFactor(rubric.Evaluate(doc), rubric.KeyBoundary); f != 1 {
		t.Errorf("'## Pitfalls' should satisfy boundary-section; factor = %v", f)
	}
}

func TestDiagnoseNamesWeakestDeterministicDim(t *testing.T) {
	doc := "# Skill\nJust do the thing and ship it.\n"
	got := rubric.Diagnose(rubric.Evaluate(doc))
	// failure-handling (weight 20) outranks boundary-section (weight 15).
	if got != "fix "+rubric.KeyFailure+" first" {
		t.Errorf("Diagnose = %q, want fix failure-handling first", got)
	}
}

func TestDiagnoseDefersToJudgeWhenClean(t *testing.T) {
	doc := "# Skill\nIf a step errors, retry.\n## Anti-pattern\nAvoid X.\n"
	got := rubric.Diagnose(rubric.Evaluate(doc))
	if !strings.HasPrefix(got, "no deterministic weakness") {
		t.Errorf("Diagnose = %q, want a defer-to-judge message", got)
	}
}
