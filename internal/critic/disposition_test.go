package critic_test

import (
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
	results := []critic.Adjudicated{
		{Finding: adh.Finding{Summary: "real", Kind: adh.FindingContract}, Ran: true, Failed: true},
		{
			Finding: adh.Finding{Summary: "passed", Kind: adh.FindingOracle},
			Ran:     true,
			Failed:  false,
		},
		{Finding: adh.Finding{Summary: "unrunnable", Kind: adh.FindingNFR}, Ran: false},
	}
	v := critic.Dispose(results)
	if len(v.Confirmed) != 1 || v.Confirmed[0].Summary != "real" {
		t.Fatalf("confirmed = %+v, want just the ran+failed finding", v.Confirmed)
	}
	if len(v.Unconfirmed) != 2 {
		t.Errorf("unconfirmed = %d, want 2 (passed + unrunnable)", len(v.Unconfirmed))
	}
	if !v.ReturnsToExecution() {
		t.Error("a confirmed finding must return the arc to execution")
	}
	if v.BlockingKind() != adh.FindingContract {
		t.Errorf("blocking kind = %q, want contract", v.BlockingKind())
	}
	if got := v.FailureNotes(); len(got) != 1 || got[0] != "contract: real" {
		t.Errorf("failure notes = %v, want [contract: real]", got)
	}
	if got := v.LessonNotes(); len(got) != 2 {
		t.Errorf("lesson notes = %v, want 2", got)
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
