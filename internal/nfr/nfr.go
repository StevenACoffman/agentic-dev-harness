// Package nfr is the nonfunctional-requirement spec format (SPEC-ADDITIONS §10.5):
// a Planguage-quantified quality attribute named by an agreed taxonomy. It binds
// the four things adh otherwise scatters into one testable unit — category (Tag,
// FURPS+/ISO-25010), measurement (Meter → a §13 tool), gate (Fail → the Evaluation
// acceptance bar), and rationale (Ambition → a decision/ADR). The type and its
// validation are a pure core; Load is the thin I/O shell.
package nfr

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
)

// DefaultDir is the conventional repo path NFR specs live under (§10.5).
const DefaultDir = ".adh/nfr"

// Direction values: whether a larger measured value is better or worse, so the
// Fail/Goal/Stretch thresholds can be checked for a consistent quality order.
const (
	Higher Direction = "higher" // bigger is better (availability, throughput)
	Lower  Direction = "lower"  // smaller is better (latency, error rate)
)

// Direction is whether a larger measured value means better or worse quality.
type Direction string

// Spec is one Planguage-quantified nonfunctional requirement (§10.5). Tag names the
// quality attribute under an agreed taxonomy (its head is a FURPS+/ISO-25010
// category, e.g. "Performance.Latency"); Scale is the unit; Meter is how it is
// measured (a §13 tool id or a method); Fail is the acceptance bar the Evaluation
// gate enforces, Goal the target, Stretch (optional) the point past which more
// investment stops paying; Ambition is the rationale a decision/ADR records.
type Spec struct {
	ID        string    `json:"id"`
	Tag       string    `json:"tag"`
	Gist      string    `json:"gist,omitempty"`
	Ambition  string    `json:"ambition,omitempty"`
	Scale     string    `json:"scale"`
	Meter     string    `json:"meter"`
	Direction Direction `json:"direction"`
	Baseline  float64   `json:"baseline,omitempty"`
	Fail      float64   `json:"fail"`
	Goal      float64   `json:"goal"`
	Stretch   float64   `json:"stretch,omitempty"`

	// Guard marks a spec the change must not breach, as opposed to one it is trying
	// to improve (§10.5, §19.2).
	//
	// An arc's acceptance bar names what the change is *for*. Nothing named what it
	// must not cost, and **an objective without guards is hill-climbed by trading
	// away everything unmeasured** — `ruflo`'s loop raised a harness score 40 → 55 by
	// adding files a presence-counting metric rewarded, with no capability change,
	// and doubled a promotion rate by relaxing the predicate from AND to OR.
	//
	// It is a property of the **repository**, not of an arc, and that is the point of
	// putting it here rather than on the arc. A per-arc guard list would be more
	// expressive and would let the author of a change choose which guards apply to
	// it, which is the configurability that invites exactly the failure above: both
	// of ruflo's were a loop editing its own criteria.
	//
	// False is an ordinary objective, so a spec nobody marked does not silently
	// become a blocker.
	Guard bool `json:"guard,omitempty"`
}

// Valid reports whether the spec is a well-formed Planguage requirement: a
// non-empty id, a Tag led by a known taxonomy category, a Scale, a Meter, a valid
// Direction, and Fail/Goal/Stretch ordered so quality increases (§10.5). It returns
// EINVALID on the first defect. Baseline is informational (a failing baseline is
// legitimate), so it is not ordered-constrained.
func (s *Spec) Valid() error {
	head, _, _ := strings.Cut(s.Tag, ".")
	switch {
	case s.ID == "":
		return &adh.Error{Code: adh.EINVALID, Message: "nfr spec with empty id"}
	case s.Tag == "" || !knownCategory(head):
		return &adh.Error{Code: adh.EINVALID, Message: fmt.Sprintf(
			"nfr %s: tag %q must lead with a known category (FURPS+/ISO 25010)", s.ID, s.Tag)}
	case s.Scale == "":
		return &adh.Error{Code: adh.EINVALID, Message: "nfr " + s.ID + " has no scale"}
	case s.Meter == "":
		return &adh.Error{Code: adh.EINVALID, Message: "nfr " + s.ID + " has no meter (§13)"}
	case s.Direction != Higher && s.Direction != Lower:
		return &adh.Error{
			Code:    adh.EINVALID,
			Message: "nfr " + s.ID + " direction must be higher or lower",
		}
	case !s.ordered():
		return &adh.Error{Code: adh.EINVALID, Message: fmt.Sprintf(
			"nfr %s: fail/goal/stretch not ordered for %s-is-better", s.ID, s.Direction)}
	}
	return nil
}

// Meets reports whether a measured value satisfies the Fail acceptance bar — the
// gate threshold the Evaluation stage enforces (§SPEC 3.1). Higher-is-better needs
// value ≥ Fail; lower-is-better needs value ≤ Fail.
func (s *Spec) Meets(value float64) bool {
	if s.Direction == Higher {
		return value >= s.Fail
	}
	return value <= s.Fail
}

// ordered reports whether Fail → Goal (→ Stretch) increases in quality for the
// spec's direction. A zero Stretch means unset and is skipped.
func (s *Spec) ordered() bool {
	if s.Direction == Higher {
		return s.Goal >= s.Fail && (s.Stretch == 0 || s.Stretch >= s.Goal)
	}
	return s.Goal <= s.Fail && (s.Stretch == 0 || s.Stretch <= s.Goal)
}

// knownCategory reports whether head is a recognized top-level quality-attribute
// category (the FURPS+ and ISO/IEC 25010 union), so a Tag names an agreed taxonomy.
func knownCategory(head string) bool {
	switch head {
	case "Performance", "Reliability", "Usability", "Functionality", "Supportability",
		"Security", "Maintainability", "Portability", "Compatibility":
		return true
	default:
		return false
	}
}

// Guards returns the specs a change must not breach, in the order declared.
//
// Requires: nothing.
// Ensures: empty rather than nil when none is marked, so a caller need not
// distinguish "no guards declared" from "did not look". Pure.
func Guards(specs []Spec) []Spec {
	out := make([]Spec, 0, len(specs))
	for i := range specs {
		if specs[i].Guard {
			out = append(out, specs[i])
		}
	}
	return out
}

// ByID returns the spec with the given id, so a caller that already knows an NFR by
// name (a critic finding's Ref, §19.2) can resolve it to its Planguage thresholds.
func ByID(specs []Spec, id string) (Spec, bool) {
	for i := range specs {
		if specs[i].ID == id {
			return specs[i], true
		}
	}
	return Spec{}, false
}

// Load reads NFR specs from JSON files under dir. An absent directory yields no
// specs and no error, so a repo that declares no NFRs is not an error.
func Load(dir string) ([]Spec, error) {
	const op = "nfr.Load"
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return []Spec{}, nil
	}
	if err != nil {
		return nil, &adh.Error{Op: op, Err: err}
	}
	specs := make([]Spec, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		data, readErr := os.ReadFile(filepath.Join(dir, entry.Name()))
		if readErr != nil {
			return nil, &adh.Error{Op: op, Err: readErr}
		}
		var spec Spec
		if err := json.Unmarshal(data, &spec); err != nil {
			return nil, &adh.Error{Op: op, Err: err}
		}
		specs = append(specs, spec)
	}
	return specs, nil
}
