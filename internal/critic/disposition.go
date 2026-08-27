package critic

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/skillet/finding"
)

// ConvergenceConstraint records SPEC-ADDITIONS §19.3: the critic runs once per
// arc, so it cannot widen a change across review rounds. A bounded critic/author
// revision loop is a deliberate future addition, not a runtime gate today.
const ConvergenceConstraint = "critic runs once per arc (§19.3); no revision loop"

// criticReply is the structured reply an operator returns for a critic turn: the
// findings it raises, each naming the artifact that would confirm it (§19.2), and the
// aspects it declares it did not examine.
type criticReply struct {
	Findings   []adh.Finding        `json:"findings"`
	Unexamined []finding.Unexamined `json:"unexamined,omitempty"`
}

// Adjudicated is a finding paired with the outcome of running its named artifact
// (§19.2): Ran is whether the artifact could be run at all, Failed whether it
// failed when it ran.
type Adjudicated struct {
	Finding adh.Finding
	Ran     bool
	Failed  bool

	// Unrunnable says why the artifact did not run, and is meaningful only when Ran
	// is false.
	//
	// §19.2 records that an unchecked finding does not block, on the reasoning that
	// Ran:false covers both *the registered tool is broken* — an honest refusal worth
	// acting on — and *the critic named a tool that never existed*, which is noise,
	// and that blocking on the second would let one bad critic wedge every arc.
	//
	// **The adjudicator already knows which**, and threw the answer away one call up:
	// runDeclaredTool's `found` distinguishes them. Carrying it does not change the
	// blocking rule — that decision stands — it makes the decision revisitable on
	// evidence rather than on the guess that the two are inseparable.
	Unrunnable adh.Unrunnable

	// Measured is the value a meter produced, and HasMeasure whether one did.
	//
	// A bool rather than a sentinel: 0 is a legitimate measurement, and NaN would be a
	// second vocabulary for "absent" beside the one this struct already has.
	Measured   float64
	HasMeasure bool

	// Regressed reports a measurement worse than the spec's baseline that still
	// cleared the acceptance bar. It is the reservation §19.2 had no disposition for:
	// the change is admissible and it cost something, and both facts are true.
	Regressed bool
}

// Verdict is the disposition of a critic's findings after adjudication (§19.2).
//
// Three buckets, because the input carries three states and collapsing two of them
// corrupted the harness's own calibration. Ran:false and Ran:true,Failed:false used to
// land together in Unconfirmed — "nobody checked this" and "this was checked and did
// not reproduce" reported as one answer.
//
// That was not merely imprecise. Unconfirmed feeds PrecisionEntry, whose own doc calls
// it "surfaced but not reproduced — a false positive", and NoisyKinds then condemns any
// kind whose false-positive rate is too high: "the next critic holds these to a higher
// bar." So a missing or unbuilt artifact taught the harness to distrust an entire
// category of real defect, and the more broken the environment the more it distrusted.
//
// A finding nobody could check is evidence about that kind in neither direction, which
// is the rule quotecheck.Unchecked and gate.VerdictUnchecked follow one repo over.
type Verdict struct {
	// Confirmed: the named artifact ran and failed. A real defect; blocks.
	Confirmed []adh.Finding

	// Unconfirmed: the artifact ran and passed. A lesson candidate (§11), and the
	// only bucket that is evidence of a false positive.
	Unconfirmed []adh.Finding

	// Unchecked: the artifact could not run — missing, unbuilt, wrong commit.
	//
	// It does not block, and that is a decision rather than an oversight; see
	// ReturnsToExecution. It is also not a lesson candidate: a lesson is drawn from a
	// finding that was checked and did not reproduce, and this one was not checked.
	Unchecked []adh.Finding

	// Reservations are findings that passed adjudication with a measurement worse
	// than baseline. They do not block — a regression inside the bar is information,
	// and blocking on it would make Fail and Baseline the same threshold — but an arc
	// that advances carrying one has cost something, and that used to be invisible.
	Reservations []adh.Finding

	// Refused is the subset of Unchecked whose artifact was a **registered** tool
	// that could not start, as opposed to a finding naming nothing or naming a tool
	// the repository never declared.
	//
	// The distinction is §19.2's stated blocker: the decision not to block on an
	// unchecked finding rests on the two being inseparable, and they are not. Kept as
	// a subset rather than a fourth bucket so the partition above stays a partition —
	// every finding is still in exactly one of the three.
	Refused []adh.Finding
}

// ParseFindings decodes a critic turn's reply (§19.2): the findings it raises and the
// aspects it declares it did not examine.
//
// Requires: nothing; any string is a valid input and an invalid one is an error.
// Ensures: EINVALID for a reply that does not parse, a finding with no summary or an
// unknown kind or class, or an unexamined entry missing either field. Pure.
//
// **An empty findings list used to be "a clean review" and could equally be a critic
// that looked at nothing** — `evaluation` disposes of the arc on that silence, so the
// two reached Ops identically. `unexamined` is how a critic says which it was.
//
// It is **testimony, not a derived fact**: a critic claiming it did not examine
// something is a statement about its own behaviour, unverifiable from outside, and
// worth what the critic is worth. That is why an ungrounded review — which the harness
// knows mechanically — is reported through relay.Outcome instead of being written here
// on the critic's behalf. skillet keeps the two as separate types deliberately, and
// collapsing them would let a mechanical skip read as a critic's own admission.
//
// An invalid entry rejects the whole reply rather than being dropped. Dropping it is
// how a reply that says nothing passes for a reply that found nothing, which is the
// failure this field exists to end.
func ParseFindings(reply string) ([]adh.Finding, []finding.Unexamined, error) {
	var parsed criticReply
	dec := json.NewDecoder(strings.NewReader(reply))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&parsed); err != nil {
		return nil, nil, &adh.Error{
			Code:    adh.EINVALID,
			Message: "critic reply is not findings JSON: " + err.Error(),
		}
	}
	for i := range parsed.Findings {
		f := parsed.Findings[i]
		if strings.TrimSpace(f.Summary) == "" {
			return nil, nil, &adh.Error{Code: adh.EINVALID, Message: "finding is missing a summary"}
		}
		if !f.Kind.Valid() {
			return nil, nil, &adh.Error{
				Code:    adh.EINVALID,
				Message: "finding names an unknown kind: " + string(f.Kind),
			}
		}
		if !f.Class.Valid() {
			return nil, nil, &adh.Error{
				Code:    adh.EINVALID,
				Message: "finding names an unknown class: " + string(f.Class),
			}
		}
	}
	for i := range parsed.Unexamined {
		u := parsed.Unexamined[i]
		if !u.Valid() {
			return nil, nil, &adh.Error{
				Code: adh.EINVALID,
				Message: "unexamined entry needs both an aspect and a reason; got aspect=" +
					strconv.Quote(u.Aspect) + " reason=" + strconv.Quote(u.Reason),
			}
		}
	}
	return parsed.Findings, parsed.Unexamined, nil
}

// Dispose classifies each adjudicated finding (§19.2).
//
// Requires: nothing; an empty result set is a clean review.
// Ensures: the three buckets partition results — every finding lands in exactly one —
// and order within each is the order adjudicated. Refused is a subset of Unchecked and
// is not part of the partition. Pure; the caller runs the artifacts
// and records the effects.
//
// The artifact not running is tested first because it is the only case where Failed
// carries no information: an adjudicator that could not run a check has nothing to
// report about whether it passed, and reading Failed there would be reading a zero
// value as an answer.
func Dispose(results []Adjudicated) Verdict {
	var v Verdict
	for i := range results {
		r := results[i]
		switch {
		case !r.Ran:
			v.Unchecked = append(v.Unchecked, r.Finding)
			if r.Unrunnable.Trustworthy() {
				v.Refused = append(v.Refused, r.Finding)
			}
		case r.Failed:
			v.Confirmed = append(v.Confirmed, r.Finding)
		default:
			v.Unconfirmed = append(v.Unconfirmed, r.Finding)
			if r.Regressed {
				v.Reservations = append(v.Reservations, r.Finding)
			}
		}
	}
	return v
}

// ReturnsToExecution reports whether the verdict blocks the arc: any confirmed
// finding is a deterministic Evaluation failure that returns the arc to Execution
// (§19.2).
//
// **This one line is where "artifacts decide what blocks" is enforced**, so it is an
// invariant rather than an implementation detail: Confirmed requires an artifact that
// ran and failed, which is why a critic's assertion alone can never block an arc.
// Widening this to any other slice would let narration block, and that is a change to a
// stated rule (§19.2) rather than a change to a condition.
//
// The rule is narrow on purpose and the other direction is the known gap: a critic that
// finds nothing leaves nothing to adjudicate, so silence advances the arc, bounded only
// by the NFR guards that are adjudicated whether or not the critic mentioned them.
//
// **An unchecked finding does not block, and the decision is recorded rather than
// emergent.** `vac-gate`'s rule — "'cannot regrade' is not 'regraded'" — argues the
// honest refusal should fail the gate, and it is the right rule where the refusal is
// trustworthy. Here it is not yet: a finding's artifact comes from a model's reply, so
// Ran:false covers both "the tool is broken" and "the critic named a tool that never
// existed", and blocking on the second would let one bad critic wedge every arc.
//
// The trigger for revisiting: the §13 tool registry can distinguish the two, so once
// adjudication resolves a finding's ref against it, a registered-but-unrunnable
// artifact becomes a refusal worth blocking on and an unregistered one stays noise.
// Until then the state is reported rather than acted on — visible, which is the part
// that was missing, since it used to be folded into Unconfirmed and counted as a false
// positive.
func (v *Verdict) ReturnsToExecution() bool { return len(v.Confirmed) > 0 }

// HasStructural reports whether any confirmed finding is structural (§19.2) — one
// that needs a design change, so the arc escalates to a human rather than spending
// rework cycles on an edit that cannot close it.
func (v *Verdict) HasStructural() bool {
	for i := range v.Confirmed {
		if v.Confirmed[i].IsStructural() {
			return true
		}
	}
	return false
}

// BlockingKind is the kind of the first confirmed finding, which the Evaluation
// gate maps to an exit code. It is empty when nothing is confirmed.
func (v *Verdict) BlockingKind() adh.FindingKind {
	if len(v.Confirmed) == 0 {
		return ""
	}
	return v.Confirmed[0].Kind
}

// Classes returns the distinct finding kinds across the confirmed and unconfirmed
// findings, sorted — the classes a disposed arc contributes to the failure-record
// log, so a recurring one can be gated for promotion (§11, §19.2). Candidates count:
// an unconfirmed finding kept to detect recurrence is exactly what promotion gates on.
func (v *Verdict) Classes() []string {
	seen := make(map[string]bool)
	for i := range v.Confirmed {
		seen[string(v.Confirmed[i].Kind)] = true
	}
	for i := range v.Unconfirmed {
		seen[string(v.Unconfirmed[i].Kind)] = true
	}
	// Unchecked counts here and deliberately not in the precision ledger. Recurrence
	// is the question this feeds, and "this kind keeps naming an artifact we cannot
	// run" is exactly a recurrence worth surfacing — where the false-positive rate it
	// would corrupt is a different question with a different answer.
	for i := range v.Unchecked {
		seen[string(v.Unchecked[i].Kind)] = true
	}
	classes := make([]string, 0, len(seen))
	for c := range seen {
		classes = append(classes, c)
	}
	sort.Strings(classes)
	return classes
}

// FailureNotes renders the confirmed findings as failure-registry entries (§4.1),
// each classed by kind so a recurring one groups under lesson.Distill.
func (v *Verdict) FailureNotes() []string { return notesFor(v.Confirmed) }

// LessonNotes renders the unconfirmed findings as lesson candidates (§11): kept to
// detect a recurring class, never a blocker (§19.2).
//
// Unchecked findings are not candidates. A lesson is drawn from a finding that was
// checked and did not reproduce; one nobody could check has taught nothing yet.
func (v *Verdict) LessonNotes() []string { return notesFor(v.Unconfirmed) }

// UncheckedNotes renders the findings whose artifact could not run, so a caller can
// report them. Same shape as the other two, because a reader comparing the three lists
// should not have to notice a formatting difference.
func (v *Verdict) UncheckedNotes() []string { return notesFor(v.Unchecked) }

// ReservationNotes renders the findings that passed while measuring worse than
// baseline, so an advancing arc can say what it cost.
func (v *Verdict) ReservationNotes() []string { return notesFor(v.Reservations) }

// RefusedNotes renders the findings a registered tool refused to check.
//
// Separate from UncheckedNotes because the two ask different things of a reader: a
// finding naming nothing is the critic's problem, and a registered tool that will not
// start is the environment's.
func (v *Verdict) RefusedNotes() []string { return notesFor(v.Refused) }

func notesFor(findings []adh.Finding) []string {
	if len(findings) == 0 {
		return nil
	}
	notes := make([]string, len(findings))
	for i := range findings {
		notes[i] = string(findings[i].Kind) + ": " + findings[i].Summary
	}
	return notes
}
