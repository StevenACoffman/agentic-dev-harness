package critic_test

import (
	"slices"
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/critic"
)

func TestParseFindingsValid(t *testing.T) {
	reply := `{"findings":[
		{"summary":"clears differ from the reference","kind":"oracle","ref":"board-corpus"},
		{"summary":"proof misses the new path","kind":"contract","class":"structural"}
	]}`
	findings, err := critic.ParseFindings(reply)
	if err != nil {
		t.Fatalf("ParseFindings: %v", err)
	}
	if len(findings) != 2 {
		t.Fatalf("findings = %d, want 2", len(findings))
	}
	if findings[0].Kind != adh.FindingOracle || findings[1].Kind != adh.FindingContract {
		t.Errorf("kinds = %q,%q; want oracle,contract", findings[0].Kind, findings[1].Kind)
	}
	if findings[0].Class != "" || findings[1].Class != adh.StructuralFinding {
		t.Errorf("classes = %q,%q; want (default),structural", findings[0].Class, findings[1].Class)
	}
}

func TestVerdictClasses(t *testing.T) {
	v := critic.Verdict{
		Confirmed: []adh.Finding{
			{Kind: adh.FindingOracle}, {Kind: adh.FindingContract},
		},
		Unconfirmed: []adh.Finding{
			{Kind: adh.FindingOracle}, {Kind: adh.FindingNFR},
		},
	}
	got := v.Classes()
	want := []string{"contract", "nfr", "oracle"}
	if len(got) != len(want) {
		t.Fatalf("Classes() = %v, want %v (distinct, sorted)", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Classes()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestVerdictHasStructural(t *testing.T) {
	fixable := critic.Verdict{Confirmed: []adh.Finding{{Kind: adh.FindingOracle}}}
	if fixable.HasStructural() {
		t.Error("a default-class confirmed finding is fixable, not structural")
	}
	structural := critic.Verdict{Confirmed: []adh.Finding{
		{Kind: adh.FindingOracle},
		{Kind: adh.FindingContract, Class: adh.StructuralFinding},
	}}
	if !structural.HasStructural() {
		t.Error("a structural confirmed finding must be reported")
	}
	// A structural finding that is only unconfirmed does not escalate.
	unconfirmed := critic.Verdict{Unconfirmed: []adh.Finding{
		{Kind: adh.FindingContract, Class: adh.StructuralFinding},
	}}
	if unconfirmed.HasStructural() {
		t.Error("HasStructural reads confirmed findings only")
	}
}

func TestParseFindingsEmptyIsCleanReview(t *testing.T) {
	for _, reply := range []string{`{"findings":[]}`, `{}`} {
		findings, err := critic.ParseFindings(reply)
		if err != nil {
			t.Fatalf("ParseFindings(%q): %v", reply, err)
		}
		if len(findings) != 0 {
			t.Errorf("ParseFindings(%q) = %v, want none", reply, findings)
		}
	}
}

func TestParseFindingsRejectsMalformed(t *testing.T) {
	tests := map[string]string{
		"free text":     "looks fine to me",
		"bare array":    `[{"summary":"x","kind":"oracle"}]`,
		"missing kind":  `{"findings":[{"summary":"x"}]}`,
		"unknown kind":  `{"findings":[{"summary":"x","kind":"vibes"}]}`,
		"empty summary": `{"findings":[{"summary":"  ","kind":"oracle"}]}`,
		"unknown field": `{"findings":[],"trust_me":true}`,
		"unknown class": `{"findings":[{"summary":"x","kind":"oracle","class":"vibes"}]}`,
	}
	for name, reply := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := critic.ParseFindings(reply); adh.ErrorCode(err) != adh.EINVALID {
				t.Errorf("ParseFindings(%q) = %v, want EINVALID", reply, err)
			}
		})
	}
}

func TestDispose(t *testing.T) {
	t.Parallel()
	// One finding of each outcome. TestDisposePartitions covers the bucketing
	// exhaustively; this covers what the buckets are then reported as, which is where
	// the collapse actually did its damage.
	v := critic.Dispose([]critic.Adjudicated{
		{Finding: adh.Finding{Summary: "real", Kind: adh.FindingContract}, Ran: true, Failed: true},
		{Finding: adh.Finding{Summary: "passed", Kind: adh.FindingOracle}, Ran: true},
		{Finding: adh.Finding{Summary: "unrunnable", Kind: adh.FindingNFR}, Ran: false},
	})

	if got := v.FailureNotes(); !slices.Equal(got, []string{"contract: real"}) {
		t.Errorf("failure notes = %v, want the confirmed finding", got)
	}
	// A lesson is drawn from a finding that was checked and did not reproduce; one
	// nobody could check has taught nothing yet.
	if got := v.LessonNotes(); !slices.Equal(got, []string{"oracle: passed"}) {
		t.Errorf("lesson notes = %v, want just the checked-and-passed finding", got)
	}
	if got := v.UncheckedNotes(); !slices.Equal(got, []string{"nfr: unrunnable"}) {
		t.Errorf("unchecked notes = %v, want the unrunnable finding", got)
	}
	// Recurrence is the question Classes feeds, and "this kind keeps naming an
	// artifact we cannot run" is a recurrence worth surfacing.
	if got := v.Classes(); !slices.Equal(got, []string{"contract", "nfr", "oracle"}) {
		t.Errorf("classes = %v, want all three kinds", got)
	}
	if v.BlockingKind() != adh.FindingContract {
		t.Errorf("blocking kind = %q, want contract", v.BlockingKind())
	}
}

// TestDisposePartitions. Every finding lands in exactly one bucket, whatever the
// combination -- including Ran:false with Failed:true, which an adjudicator should
// never produce and which must not be read as a confirmation if it does.
func TestDisposePartitions(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		in   critic.Adjudicated
		want string
	}{
		"ran and failed": {critic.Adjudicated{Ran: true, Failed: true}, "confirmed"},
		"ran and passed": {critic.Adjudicated{Ran: true, Failed: false}, "unconfirmed"},
		"did not run":    {critic.Adjudicated{Ran: false, Failed: false}, "unchecked"},
		"the zero value": {critic.Adjudicated{}, "unchecked"},
		"did not run, but reported failed": {
			critic.Adjudicated{Ran: false, Failed: true}, "unchecked",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			tc.in.Finding = adh.Finding{Summary: "s", Kind: adh.FindingOracle}
			v := critic.Dispose([]critic.Adjudicated{tc.in})
			got := map[string]int{
				"confirmed":   len(v.Confirmed),
				"unconfirmed": len(v.Unconfirmed),
				"unchecked":   len(v.Unchecked),
			}
			for bucket, n := range got {
				want := 0
				if bucket == tc.want {
					want = 1
				}
				if n != want {
					t.Errorf("%s = %d, want %d (all: %+v)", bucket, n, want, got)
				}
			}
		})
	}
}

// TestAnUncheckedFindingDoesNotBlock pins the decision rather than the accident.
// vac-gate argues the honest refusal should fail the gate, and it is not adopted here
// because Ran:false cannot yet distinguish a broken tool from a critic naming a tool
// that never existed -- see ReturnsToExecution for the trigger that would change it.
func TestAnUncheckedFindingDoesNotBlock(t *testing.T) {
	t.Parallel()
	v := critic.Dispose([]critic.Adjudicated{
		{Finding: adh.Finding{Summary: "s", Kind: adh.FindingNFR}, Ran: false},
	})
	if v.ReturnsToExecution() {
		t.Error("an unchecked finding blocked the arc")
	}
	if v.BlockingKind() != "" {
		t.Errorf("blocking kind = %q, want empty", v.BlockingKind())
	}
}

func TestDisposeCleanReviewDoesNotBlock(t *testing.T) {
	v := critic.Dispose(nil)
	if v.ReturnsToExecution() {
		t.Error("no findings must not block the arc")
	}
	if v.BlockingKind() != "" {
		t.Errorf("blocking kind = %q, want empty", v.BlockingKind())
	}
}
