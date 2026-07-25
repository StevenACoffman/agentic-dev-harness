package judge_test

import (
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/judge"
)

func TestScoreOperators(t *testing.T) {
	output := "## Key Risks\nConfidence: High\nThe tool ran gofmt on the file.\n"
	tests := []struct {
		name  string
		check judge.Check
		want  bool
	}{
		{
			name:  "section present",
			check: judge.Check{Op: judge.OpSectionPresent, Arg: "Key Risks"},
			want:  true,
		},
		{
			name:  "section absent",
			check: judge.Check{Op: judge.OpSectionPresent, Arg: "Appendix"},
			want:  false,
		},
		{
			name:  "regex match",
			check: judge.Check{Op: judge.OpRegex, Arg: `[Cc]onfidence:`},
			want:  true,
		},
		{name: "regex no match", check: judge.Check{Op: judge.OpRegex, Arg: `NOPE`}, want: false},
		{name: "bad regex fails", check: judge.Check{Op: judge.OpRegex, Arg: `(`}, want: false},
		{name: "contains", check: judge.Check{Op: judge.OpContains, Arg: "gofmt"}, want: true},
		{name: "tool called", check: judge.Check{Op: judge.OpToolCalled, Arg: "gofmt"}, want: true},
		{name: "max chars ok", check: judge.Check{Op: judge.OpMaxChars, Arg: "1000"}, want: true},
		{
			name:  "max chars exceeded",
			check: judge.Check{Op: judge.OpMaxChars, Arg: "5"},
			want:  false,
		},
		{name: "min chars ok", check: judge.Check{Op: judge.OpMinChars, Arg: "5"}, want: true},
		{
			name:  "bad length arg fails",
			check: judge.Check{Op: judge.OpMaxChars, Arg: "abc"},
			want:  false,
		},
		{
			name:  "unknown op fails",
			check: judge.Check{Op: judge.Op("bogus"), Arg: "x"},
			want:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := judge.Score(output, []judge.Check{tt.check})
			if err != nil {
				t.Fatalf("Score: %v", err)
			}
			got := res.Hard == 1.0
			if got != tt.want {
				t.Errorf(
					"check %+v hard=%v, want pass=%v (%v)",
					tt.check,
					res.Hard,
					tt.want,
					res.Why,
				)
			}
		})
	}
}

func TestScoreHardSoft(t *testing.T) {
	output := "## Risks\nok\n"
	checks := []judge.Check{
		{Op: judge.OpSectionPresent, Arg: "Risks"}, // pass
		{Op: judge.OpContains, Arg: "missing"},     // fail
	}
	res, err := judge.Score(output, checks)
	if err != nil {
		t.Fatalf("Score: %v", err)
	}
	if res.Hard != 0.0 {
		t.Errorf("hard = %v, want 0 (one check failed)", res.Hard)
	}
	if res.Soft != 0.5 {
		t.Errorf("soft = %v, want 0.5", res.Soft)
	}
}

func TestScoreNoChecks(t *testing.T) {
	if _, err := judge.Score("x", nil); adh.ErrorCode(err) != adh.EINVALID {
		t.Errorf("Score with no checks code = %q, want invalid", adh.ErrorCode(err))
	}
}
