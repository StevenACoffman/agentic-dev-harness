package contextstore

import (
	"bytes"
	"encoding/json"
	"strings"
)

// humanActor is the actor-class prefix that earns HumanReviewed when folding events.
//
// Only a human actor does. Any other named actor is a machine confirmation, and an
// event with no actor at all confirms nothing -- it is the shape a fabricated event
// would take, and treating it as evidence would make the fold worth defeating.
const humanActor = "human"

// Verification is one verification event: who confirmed a unit, and when.
//
// OKF §5.2 stores these rather than the tier they imply, and the reason is what a stored
// tier cannot say. "human-reviewed" cannot name which human or on what date, and it
// cannot represent "reviewed by a person *and* re-confirmed by a nightly process" --
// which is the state a compounding knowledge base reaches most often, since §5.2's list
// exists precisely so a sign-off and an automated pass stay distinct events.
type Verification struct {
	By string `json:"by"`
	At string `json:"at,omitempty"`
}

// Trust is a unit's verification record: the events, with the tier derived from them.
//
// It accepts both stored shapes. A list of events is the OKF form. A bare string is the
// form adh wrote before events were expressible, and reading it is **permanent rather
// than transitional** -- nothing in this repository has ever written the field, so there
// is no migration event after which the legacy branch could be deleted. Both forms keep
// asserting exactly what they asserted before, so no store gains or loses standing by
// being read with this type.
type Trust struct {
	// Events is the OKF §5.2 list. Empty when the stored form was a bare tier.
	Events []Verification
	// Stated is a tier stored directly, including an unrecognised one: an unknown value
	// has to survive parsing so Valid reports it as a taxonomy problem, which is what
	// `context lint` and the wiki index already do with it. Rejecting it at parse time
	// would turn a lint finding into a load failure.
	Stated TrustTier
}

// Tier is the trust tier this record implies (OKF §5.3): derived from the events when
// there are any, otherwise the stored tier, otherwise Unverified.
//
// **Unverified is the zero value on purpose.** No events and no stored tier means
// nobody has confirmed the unit, never that a confirmation passed.
func (t Trust) Tier() TrustTier {
	if len(t.Events) == 0 {
		if t.Stated == "" {
			return Unverified
		}
		return t.Stated
	}
	tier := Unverified
	for i := range t.Events {
		switch {
		case actorClass(t.Events[i].By) == humanActor:
			return HumanReviewed
		case t.Events[i].By != "":
			tier = MachineConfirmed
		}
	}
	return tier
}

// Valid reports whether the record is inside the taxonomy. An empty record is valid and
// means unverified.
func (t Trust) Valid() bool {
	if len(t.Events) > 0 {
		for i := range t.Events {
			if t.Events[i].By == "" {
				return false
			}
		}
		return true
	}
	return t.Stated.Valid()
}

// MarshalJSON writes back whichever shape the record holds: the event list when it has
// events, else the bare tier.
//
// Nothing in adh writes a unit today, so this exists to keep the type honest rather than
// to serve a caller. Note that encoding/json cannot omit an empty struct field, so a
// writer added later will emit `"verified":""` for an unverified unit -- semantically the
// same as omitting it, and worth deciding deliberately at that point.
func (t Trust) MarshalJSON() ([]byte, error) {
	if len(t.Events) > 0 {
		data, err := json.Marshal(t.Events)
		if err != nil {
			return nil, err //nolint:wrapcheck // the caller is encoding/json itself
		}
		return data, nil
	}
	data, err := json.Marshal(string(t.Stated))
	if err != nil {
		return nil, err //nolint:wrapcheck // the caller is encoding/json itself
	}
	return data, nil
}

// UnmarshalJSON accepts either stored shape.
func (t *Trust) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) > 0 && trimmed[0] == '[' {
		var events []Verification
		if err := json.Unmarshal(trimmed, &events); err != nil {
			return err //nolint:wrapcheck // the caller is encoding/json itself
		}
		t.Events = events
		return nil
	}
	var stated string
	if err := json.Unmarshal(trimmed, &stated); err != nil {
		return err //nolint:wrapcheck // the caller is encoding/json itself
	}
	t.Stated = TrustTier(stated)
	return nil
}

// actorClass is the part of an actor id before its colon: "human:steve" is a human,
// "ci:nightly" is not. An id with no colon is its own class, so a bare "human" counts.
func actorClass(by string) string {
	class, _, _ := strings.Cut(by, ":")
	return class
}
