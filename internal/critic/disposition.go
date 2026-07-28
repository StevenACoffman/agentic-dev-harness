package critic

import (
	"encoding/json"
	"strings"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
)

// ConvergenceConstraint records SPEC-ADDITIONS §19.3: the critic runs once per
// arc, so it cannot widen a change across review rounds. A bounded critic/author
// revision loop is a deliberate future addition, not a runtime gate today.
const ConvergenceConstraint = "critic runs once per arc (§19.3); no revision loop"

// criticReply is the structured reply an operator returns for a critic turn: the
// findings it raises, each naming the artifact that would confirm it (§19.2).
type criticReply struct {
	Findings []adh.Finding `json:"findings"`
}

// Adjudicated is a finding paired with the outcome of running its named artifact
// (§19.2): Ran is whether the artifact could be run at all, Failed whether it
// failed when it ran.
type Adjudicated struct {
	Finding adh.Finding
	Ran     bool
	Failed  bool
}

// Verdict is the disposition of a critic's findings after adjudication (§19.2):
// confirmed findings (a named artifact that ran and failed) and unconfirmed ones
// (no artifact ran, or it passed).
type Verdict struct {
	Confirmed   []adh.Finding
	Unconfirmed []adh.Finding
}

// ParseFindings decodes a critic turn's reply into findings (§19.2). The reply is
// a JSON object {"findings":[{summary,kind,ref}...]}; an empty or absent list is a
// clean review. It validates each finding — a non-empty summary and a known kind
// — and rejects a malformed reply with EINVALID, so an unparseable critic answer
// never advances an arc on trust.
func ParseFindings(reply string) ([]adh.Finding, error) {
	var parsed criticReply
	dec := json.NewDecoder(strings.NewReader(reply))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&parsed); err != nil {
		return nil, &adh.Error{
			Code:    adh.EINVALID,
			Message: "critic reply is not findings JSON: " + err.Error(),
		}
	}
	for i := range parsed.Findings {
		f := parsed.Findings[i]
		if strings.TrimSpace(f.Summary) == "" {
			return nil, &adh.Error{Code: adh.EINVALID, Message: "finding is missing a summary"}
		}
		if !f.Kind.Valid() {
			return nil, &adh.Error{
				Code:    adh.EINVALID,
				Message: "finding names an unknown kind: " + string(f.Kind),
			}
		}
	}
	return parsed.Findings, nil
}

// Dispose classifies each adjudicated finding (§19.2): a finding is confirmed
// only when its artifact ran and failed; every other case — it passed, or no
// artifact ran — is unconfirmed. It is pure; the caller runs the artifacts and
// records the effects.
func Dispose(results []Adjudicated) Verdict {
	var v Verdict
	for i := range results {
		r := results[i]
		if r.Ran && r.Failed {
			v.Confirmed = append(v.Confirmed, r.Finding)
			continue
		}
		v.Unconfirmed = append(v.Unconfirmed, r.Finding)
	}
	return v
}

// ReturnsToExecution reports whether the verdict blocks the arc: any confirmed
// finding is a deterministic Evaluation failure that returns the arc to Execution
// (§19.2).
func (v *Verdict) ReturnsToExecution() bool { return len(v.Confirmed) > 0 }

// BlockingKind is the kind of the first confirmed finding, which the Evaluation
// gate maps to an exit code. It is empty when nothing is confirmed.
func (v *Verdict) BlockingKind() adh.FindingKind {
	if len(v.Confirmed) == 0 {
		return ""
	}
	return v.Confirmed[0].Kind
}

// FailureNotes renders the confirmed findings as failure-registry entries (§4.1),
// each classed by kind so a recurring one groups under lesson.Distill.
func (v *Verdict) FailureNotes() []string { return notesFor(v.Confirmed) }

// LessonNotes renders the unconfirmed findings as lesson candidates (§11): kept to
// detect a recurring class, never a blocker (§19.2).
func (v *Verdict) LessonNotes() []string { return notesFor(v.Unconfirmed) }

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
