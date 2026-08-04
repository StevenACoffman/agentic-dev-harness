package harness_test

import (
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/internal/harness"
	"github.com/StevenACoffman/agentic-dev-harness/internal/judge"
	gate "github.com/StevenACoffman/skillet/ratchet"
)

const evalDoc = "# Skill\nIf the build fails, open an early PR.\n## Boundary\nNot for zero-to-one work.\n"

func TestEvalRubricOnly(t *testing.T) {
	rep, err := harness.Eval(evalDoc, "", nil)
	if err != nil {
		t.Fatalf("Eval: %v", err)
	}
	if rep.Behavioral != nil {
		t.Errorf("Behavioral = %+v, want nil with no checks", rep.Behavioral)
	}
	if rep.Rubric.DetScore != 100 {
		t.Errorf("DetScore = %v, want 100", rep.Rubric.DetScore)
	}
}

func TestEvalPassingChecks(t *testing.T) {
	checks := []judge.Check{{Op: judge.OpSectionPresent, Arg: "Boundary"}}
	rep, err := harness.Eval(evalDoc, evalDoc, checks)
	if err != nil {
		t.Fatalf("Eval: %v", err)
	}
	if rep.Behavioral == nil || rep.Behavioral.Hard != 1.0 {
		t.Errorf("Behavioral = %+v, want hard 1", rep.Behavioral)
	}
}

func TestEvalFailingChecks(t *testing.T) {
	checks := []judge.Check{{Op: judge.OpContains, Arg: "zzz"}}
	rep, err := harness.Eval(evalDoc, "abc", checks)
	if err != nil {
		t.Fatalf("Eval: %v", err)
	}
	if rep.Behavioral == nil || rep.Behavioral.Hard != 0.0 {
		t.Errorf("Behavioral = %+v, want hard 0", rep.Behavioral)
	}
}

func TestMeetsFloor(t *testing.T) {
	rep, err := harness.Eval(evalDoc, "", nil) // scores 100
	if err != nil {
		t.Fatalf("Eval: %v", err)
	}
	tests := []struct {
		name string
		min  float64
		want bool
	}{
		{name: "disabled by zero", min: 0, want: true},
		{name: "disabled by negative", min: -5, want: true},
		{name: "at the floor", min: 100, want: true},
		{name: "below the floor", min: 100.1, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := rep.MeetsFloor(tt.min); got != tt.want {
				t.Errorf("MeetsFloor(%v) = %v, want %v", tt.min, got, tt.want)
			}
		})
	}
}

func TestClassify(t *testing.T) {
	if got := harness.Classify(true); got != harness.Lapse {
		t.Errorf("Classify(ruleExists=true) = %q, want lapse", got)
	}
	if got := harness.Classify(false); got != harness.Defect {
		t.Errorf("Classify(ruleExists=false) = %q, want defect", got)
	}
}

func TestSelfTest(t *testing.T) {
	if err := harness.SelfTest(); err != nil {
		t.Errorf("SelfTest should pass (the ratchet rejects non-improving candidates): %v", err)
	}
}

func TestGraderSelfTest(t *testing.T) {
	if err := harness.GraderSelfTest(); err != nil {
		t.Errorf("GraderSelfTest should pass (the rubric discriminates strong from weak): %v", err)
	}
}

func TestAccept(t *testing.T) {
	tests := []struct {
		name                string
		cand, current, best float64
		want                gate.Action
	}{
		{
			name:    "strict improvement is new best",
			cand:    0.8,
			current: 0.7,
			best:    0.7,
			want:    gate.AcceptNewBest,
		},
		{name: "tie rejects", cand: 0.7, current: 0.7, best: 0.7, want: gate.Reject},
		{name: "regression rejects", cand: 0.6, current: 0.7, best: 0.7, want: gate.Reject},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := harness.Accept(tt.cand, tt.current, tt.best); got.Action != tt.want {
				t.Errorf("Accept = %q, want %q", got.Action, tt.want)
			}
		})
	}
}

func TestCalibrateJudge(t *testing.T) {
	cases := []harness.JudgeCase{
		{
			Name:     "good passes",
			Output:   "the deadline was respected",
			Checks:   []judge.Check{{Op: "contains", Arg: "deadline"}},
			WantPass: true,
		},
		{
			Name:     "bad fails",
			Output:   "nothing relevant here",
			Checks:   []judge.Check{{Op: "contains", Arg: "deadline"}},
			WantPass: false,
		},
		{
			Name:     "mislabeled — judge disagrees",
			Output:   "no match",
			Checks:   []judge.Check{{Op: "contains", Arg: "deadline"}},
			WantPass: true, // labeled pass but the check fails → disagreement
		},
	}
	cal, err := harness.CalibrateJudge(cases)
	if err != nil {
		t.Fatalf("CalibrateJudge: %v", err)
	}
	if cal.Cases != 3 || cal.Agree != 2 {
		t.Fatalf("calibration = %+v, want 2/3 agree", cal)
	}
	if len(cal.Disagree) != 1 || cal.Disagree[0] != "mislabeled — judge disagrees" {
		t.Errorf("disagree = %v, want the mislabeled case", cal.Disagree)
	}
	// An empty check set on a case is a caller error.
	if _, err := harness.CalibrateJudge([]harness.JudgeCase{{Name: "x"}}); err == nil {
		t.Error("a case with no checks should error")
	}
}
