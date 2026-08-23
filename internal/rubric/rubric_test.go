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
	// The fenced block is load-bearing: failure-handling docks only an artifact that runs
	// something. Without it this document executes nothing, there is no runtime failure to
	// encode, and 20 of the 35 points below would not be deducted.
	doc := "# Skill\nJust do the thing and ship it.\n\n```sh\ndeploy --prod\n```\n"
	rep := rubric.Evaluate(doc)
	// failure-handling (20) and boundary-section (15) both dock to 0.
	if rep.DetScore != 65 {
		t.Errorf("DetScore = %v, want 65 (100 - 20 - 15)", rep.DetScore)
	}
}

// TestEvaluateProseOnlyIsNotDockedForFailure is the other half: the same document without
// anything runnable keeps its 20 points, because "no failure mechanism" is a category error
// for an artifact that executes nothing rather than a defect in it.
func TestEvaluateProseOnlyIsNotDockedForFailure(t *testing.T) {
	doc := "# Skill\nJust do the thing and ship it.\n"
	if f := dimFactor(rubric.Evaluate(doc), rubric.KeyFailure); f != 1 {
		t.Errorf("prose-only artifact docked for failure-handling; factor = %v", f)
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
	// The fence makes the deduction applicable, so a factor of 0 means "the regex rejected
	// this prose" rather than "the artifact executes nothing" -- without it the assertion
	// would pass for the wrong reason and stop pinning anything.
	doc := "# Skill\n\nDo the thing. A stage fails when the routed input is malformed.\n\n" +
		"```sh\nrun --stage\n```\n"
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
	// The fenced block matters: failure-handling only docks an artifact that actually runs
	// something, so a prose-only document would score full there and this would name
	// boundary-section instead.
	doc := "# Skill\nJust do the thing and ship it.\n\n```sh\ndeploy --prod\n```\n"
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

// TestFailureAppliesOnlyToArtifactsThatExecute is the table for the whole rule: a prose
// branch always satisfies the dimension, and its absence is a defect only when the artifact
// runs something. A heading alone never satisfies it -- counting KindSection spans alongside
// KindProse is what let a bare "## Boundary" earn all 20 points.
func TestFailureAppliesOnlyToArtifactsThatExecute(t *testing.T) {
	const fence = "\n\n```sh\nrun --it\n```\n"
	cases := map[string]struct {
		doc  string
		want float64
	}{
		"branch, executes":         {"# S\n\nIf the call fails, retry." + fence, 1},
		"branch, executes nothing": {"# S\n\nIf the call fails, retry.\n", 1},
		// The case the split exists for: the heading matches the failure vocabulary, but
		// nothing under it says what to do when anything fails.
		"section only, executes":         {"# S\n\n## Boundary\n\n- scope note\n" + fence, 0},
		"section only, executes nothing": {"# S\n\n## Boundary\n\n- scope note\n", 1},
		"nothing, executes":              {"# S\n\nDo it." + fence, 0},
		"nothing, executes nothing":      {"# S\n\nDo it.\n", 1},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if f := dimFactor(rubric.Evaluate(tc.doc), rubric.KeyFailure); f != tc.want {
				t.Errorf("factor = %v, want %v", f, tc.want)
			}
		})
	}
}

// TestEvaluateIgnoresFrontmatter pins that a YAML header is discarded rather than scored.
// A description is metadata: it instructs nobody, so a failure word in it is not a failure
// mechanism, and a hedge in it is not a softened instruction.
func TestEvaluateIgnoresFrontmatter(t *testing.T) {
	// The real case that motivated this: the branch regex matched the *description* of
	// letsgo-form-validator, and the artifact kept full failure-handling credit while saying
	// nothing about what to do when anything fails.
	const withHeader = "---\n" +
		"name: form-validator\n" +
		"description: Use when validation errors need rendering, specifically when a field fails.\n" +
		"---\n\n# Guide\n\nRender the form.\n\n```sh\nrun --it\n```\n"
	if f := dimFactor(rubric.Evaluate(withHeader), rubric.KeyFailure); f != 0 {
		t.Errorf("a failure phrase in frontmatter earned failure-handling credit; factor = %v", f)
	}
	// The same words in the body are a real branch and must still count, so the test cannot
	// pass merely because the regex stopped matching.
	const inBody = "# Guide\n\nIf a field fails, re-render the form.\n\n```sh\nrun --it\n```\n"
	if f := dimFactor(rubric.Evaluate(inBody), rubric.KeyFailure); f != 1 {
		t.Errorf("a branch in the body must still satisfy failure-handling; factor = %v", f)
	}
}

func TestEvaluateWithoutFrontmatterIsUnchanged(t *testing.T) {
	// Split returns the whole document as body when there is no leading "---", so an
	// artifact without a header must score exactly as it did before.
	const doc = "# Guide\n\nIf the check fails, roll back.\n\n## Boundary\n\nNot here.\n"
	if got := rubric.Evaluate(doc).DetScore; got != 100 {
		t.Errorf("DetScore = %v, want 100 for a header-less artifact", got)
	}
}

func TestEvaluateFindsCRLFFrontmatter(t *testing.T) {
	// frontmatter.Split normalizes CRLF itself. A caller that normalized first, or that
	// hand-rolled the scan, would miss a Windows-written header and score it as body.
	// The phrase must be one the branch regex actually matches ("if X fails", not "fails
	// when X" -- skilllens rejects the latter as prose). Otherwise this passes whether or
	// not the header is split off, and pins nothing.
	const doc = "---\r\ndescription: if the input fails, re-render it\r\n---\r\n\r\n" +
		"# Guide\r\n\r\nRender it.\r\n\r\n```sh\r\nrun --it\r\n```\r\n"
	if f := dimFactor(rubric.Evaluate(doc), rubric.KeyFailure); f != 0 {
		t.Errorf("a CRLF header was scored as body; factor = %v", f)
	}
}

// scoreFor returns the deterministic reason string for one dimension.
func scoreFor(t *testing.T, key, body string) (float64, string) {
	t.Helper()
	for _, d := range rubric.Evaluate(body).Dims {
		if d.Key == key {
			return d.Deterministic, d.Reason
		}
	}
	t.Fatalf("no %s dimension in the result", key)
	return 0, ""
}

// TestTheBoundaryDimensionSkipsAnArtifactItCannotJudge. skilllens states the exposure
// and it is not symmetric: for a skill whose failure is "the output has the wrong
// shape", a prohibition list is the form its own head-to-head reports as worse than no
// guidance at all — so docking its absence recommends the change that harms the skill.
//
// The detectors were validated on the discipline skill, whose failure is skipping a
// rule under pressure, and a document that executes nothing has no rule under pressure.
func TestTheBoundaryDimensionSkipsAnArtifactItCannotJudge(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		body string
		want float64
	}{
		"executes nothing, no boundary": {
			"# Notes\n\nProse about when to prefer one approach.\n", 1.0,
		},
		"executes commands, no boundary": {
			"# Guide\n\n```sh\nmake verify\n```\n\nRun it.\n", 0.0,
		},
		"executes commands, has a boundary": {
			"# Guide\n\n```sh\nmake verify\n```\n\n## Boundary\n\nNot outside the loop.\n", 1.0,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, reason := scoreFor(t, rubric.KeyBoundary, tc.body)
			if got != tc.want {
				t.Errorf("score = %.1f, want %.1f (%s)", got, tc.want, reason)
			}
			// The reason has to say *which* of the two 1.0 cases it is, or a reader
			// cannot tell a passing artifact from an unjudged one.
			if tc.want == 1.0 && got == 1.0 && !strings.Contains(reason, "boundary") {
				t.Errorf("the reason does not mention the dimension: %q", reason)
			}
		})
	}
}

// TestTheDeterministicGraderIsSilentOnAProseDocument is the property the grader
// self-test's failure exposed, pinned so it reads as a decision rather than a gap.
//
// Both deterministic dimensions are inapplicable to an artifact that executes nothing,
// so for such a document they say nothing and the score must come from the judge
// dimensions. Before the boundary check existed, the only thing separating two prose
// documents was the deduction skilllens calls harmful.
func TestTheDeterministicGraderIsSilentOnAProseDocument(t *testing.T) {
	t.Parallel()
	rich := "# Guide\n\n## Failures\n\nIf it fails, roll back.\n\n## Boundary\n\nNot here.\n"
	bare := "# Guide\n\nProse only.\n"

	if a, b := rubric.Evaluate(rich).DetScore, rubric.Evaluate(bare).DetScore; a != b {
		t.Errorf("deterministic scores differ for two non-executing documents: %.2f vs %.2f;"+
			" the deterministic dimensions are not valid for either", a, b)
	}
}
