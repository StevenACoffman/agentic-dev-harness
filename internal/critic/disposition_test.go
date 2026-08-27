package critic_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/critic"
	"github.com/StevenACoffman/skillet/finding"
)

func TestParseFindingsValid(t *testing.T) {
	reply := `{"findings":[
		{"summary":"clears differ from the reference","kind":"oracle","ref":"board-corpus"},
		{"summary":"proof misses the new path","kind":"contract","class":"structural"}
	]}`
	findings, _, err := critic.ParseFindings(reply)
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
		findings, _, err := critic.ParseFindings(reply)
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
			if _, _, err := critic.ParseFindings(reply); adh.ErrorCode(err) != adh.EINVALID {
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

// wantGaps asserts a parse outcome: a rejection naming the missing field, or the
// expected number of declared gaps.
func wantGaps(t *testing.T, got []finding.Unexamined, err error, want int, bad bool) {
	t.Helper()
	if bad {
		if err == nil {
			t.Fatal("an invalid unexamined entry was accepted")
		}
		if !strings.Contains(err.Error(), "aspect and a reason") {
			t.Errorf("the error does not say what is missing: %v", err)
		}
		return
	}
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(got) != want {
		t.Errorf("got %d unexamined, want %d", len(got), want)
	}
}

// TestParseUnexamined. An empty findings list used to be "a clean review" and could
// equally be a critic that looked at nothing; evaluation disposes of the arc on that
// silence, so the two reached Ops identically.
func TestParseUnexamined(t *testing.T) {
	t.Parallel()
	const ok = `{"findings":[],"unexamined":[{"aspect":"concurrency","reason":"no repro harness"}]}`
	cases := map[string]struct {
		reply string
		want  int
		bad   bool
	}{
		"a declared gap":    {ok, 1, false},
		"no unexamined key": {`{"findings":[]}`, 0, false},
		"an empty list":     {`{"findings":[],"unexamined":[]}`, 0, false},
		// An invalid entry rejects the whole reply. Dropping it silently is how a
		// reply that says nothing passes for a reply that found nothing.
		"a gap with no reason": {`{"unexamined":[{"aspect":"a","reason":""}]}`, 0, true},
		"a gap with no aspect": {`{"unexamined":[{"aspect":"","reason":"r"}]}`, 0, true},
		"a whitespace reason":  {`{"unexamined":[{"aspect":"a","reason":"  "}]}`, 0, true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, got, err := critic.ParseFindings(tc.reply)
			wantGaps(t, got, err, tc.want, tc.bad)
		})
	}
}

// TestADeclaredGapCannotChangeAVerdict is the advisory guarantee, checked here because
// this is where it would be broken. skillet keeps it structural — Result.HasBlocking
// iterates diagnostics only — but nothing stops adh from reading Unexamined in Dispose,
// and a critic that can block by declaring a gap learns to declare none.
func TestADeclaredGapCannotChangeAVerdict(t *testing.T) {
	t.Parallel()
	const reply = `{"findings":[{"summary":"s","kind":"oracle","class":"fixable"}],
		"unexamined":[{"aspect":"concurrency","reason":"no repro harness"}]}`
	withGap, gaps, err := critic.ParseFindings(reply)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(gaps) != 1 {
		t.Fatalf("got %d gaps, want 1", len(gaps))
	}
	without, _, err := critic.ParseFindings(
		`{"findings":[{"summary":"s","kind":"oracle","class":"fixable"}]}`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	adjudicate := func(fs []adh.Finding) critic.Verdict {
		results := make([]critic.Adjudicated, 0, len(fs))
		for _, f := range fs {
			results = append(results, critic.Adjudicated{Finding: f, Ran: true, Failed: true})
		}
		return critic.Dispose(results)
	}
	a, b := adjudicate(withGap), adjudicate(without)
	if a.ReturnsToExecution() != b.ReturnsToExecution() ||
		len(a.Confirmed) != len(b.Confirmed) {
		t.Error("a declared gap changed the verdict")
	}
}

// TestClearingFindingsClearsGaps guards both sites at once, by asserting the property
// rather than visiting each. A gap declared by the review just disposed of must not be
// read as a gap in the next one, and there are two places findings are cleared —
// evaluation.Apply and `adh reject` — so the risk is doing one and not the other.
func TestClearingFindingsClearsGaps(t *testing.T) {
	t.Parallel()
	// Derived from the source rather than asserted as a list: a third clear site added
	// later shows up here instead of silently leaking a stale gap.
	roots := []string{
		filepath.Join("..", "evaluation", "evaluation.go"),
		filepath.Join("..", "..", "cmd", "reject", "reject.go"),
	}
	for _, path := range roots {
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		text := string(src)
		if !strings.Contains(text, "arc.Findings = nil") {
			continue // this file no longer clears findings
		}
		if !strings.Contains(text, "arc.Unexamined = nil") {
			t.Errorf("%s clears Findings and leaves Unexamined; a stale gap survives", path)
		}
	}
}

// TestARegisteredToolThatWillNotStartIsARefusal. §19.2 defers the decision to block on
// an unchecked finding because Ran:false covers both a broken tool and a critic naming
// one that never existed. The adjudicator already knows which and used to throw the
// answer away; this is that answer surviving to the verdict.
func TestARegisteredToolThatWillNotStartIsARefusal(t *testing.T) {
	t.Parallel()
	f := func(s string) adh.Finding {
		return adh.Finding{Summary: s, Kind: adh.FindingOracle}
	}
	v := critic.Dispose([]critic.Adjudicated{
		{Finding: f("registered but broken"), Unrunnable: adh.UnrunnableToolFailed},
		{Finding: f("named nothing"), Unrunnable: adh.UnrunnableNoRef},
		{Finding: f("named a phantom"), Unrunnable: adh.UnrunnableUnknownRef},
		{Finding: f("ran and passed"), Ran: true},
	})

	// All three unrunnable findings stay in Unchecked: the partition is unchanged and
	// the blocking rule is unchanged with it.
	if len(v.Unchecked) != 3 {
		t.Fatalf("unchecked = %d, want 3", len(v.Unchecked))
	}
	if v.ReturnsToExecution() {
		t.Error("a refusal blocked the arc; §19.2's decision was supposed to stand")
	}
	// Only the registered-but-broken one is a refusal.
	if got := v.RefusedNotes(); len(got) != 1 ||
		!strings.Contains(got[0], "registered but broken") {
		t.Errorf("refused = %v, want only the registered tool that could not start", got)
	}
}

// TestTheZeroUnrunnableIsNotARefusal. An Adjudicated nobody populated must not claim
// the repository refused anything — the direction a mistake should fail in.
func TestTheZeroUnrunnableIsNotARefusal(t *testing.T) {
	t.Parallel()
	var u adh.Unrunnable
	if u.Trustworthy() {
		t.Error("the zero Unrunnable reads as a trustworthy refusal")
	}
	v := critic.Dispose([]critic.Adjudicated{
		{Finding: adh.Finding{Summary: "s", Kind: adh.FindingOracle}},
	})
	if len(v.Refused) != 0 {
		t.Errorf("an unpopulated reason produced a refusal: %v", v.Refused)
	}
}
