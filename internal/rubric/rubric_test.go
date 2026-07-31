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
