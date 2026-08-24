package contextstore_test

import (
	"encoding/json"
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/internal/contextstore"
)

// wantTier unmarshals a stored `verified` value and asserts the tier it derives, plus
// whether the record is inside the taxonomy. Extracted before the table so each case
// reads as the claim it makes about a stored shape.
func wantTier(
	t *testing.T,
	stored string,
	wantTier contextstore.TrustTier,
	wantValid bool,
) {
	t.Helper()
	var got contextstore.Trust
	if err := json.Unmarshal([]byte(stored), &got); err != nil {
		t.Fatalf("unmarshal %s: %v", stored, err)
	}
	if tier := got.Tier(); tier != wantTier {
		t.Errorf("%s -> tier %q, want %q", stored, tier, wantTier)
	}
	if valid := got.Valid(); valid != wantValid {
		t.Errorf("%s -> valid %v, want %v", stored, valid, wantValid)
	}
}

func TestTrustReadsBothStoredShapes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		stored string
		tier   contextstore.TrustTier
		valid  bool
	}{{
		// The legacy shape. Reading it is permanent, not transitional: nothing has
		// ever written this field, so there is no migration after which it could go.
		name:   "a legacy bare tier keeps asserting what it asserted",
		stored: `"human-reviewed"`, tier: contextstore.HumanReviewed, valid: true,
	}, {
		name:   "an unrecognised tier survives parsing so lint can report it",
		stored: `"sideways"`, tier: "sideways", valid: false,
	}, {
		// The zero value must assert nothing.
		name:   "an empty string is unverified",
		stored: `""`, tier: contextstore.Unverified, valid: true,
	}, {
		name:   "a machine event confirms but does not elevate",
		stored: `[{"by":"ci:nightly","at":"2026-08-01"}]`,
		tier:   contextstore.MachineConfirmed, valid: true,
	}, {
		name:   "a human event elevates",
		stored: `[{"by":"human:steve","at":"2026-08-01"}]`,
		tier:   contextstore.HumanReviewed, valid: true,
	}, {
		// The case a stored tier cannot represent, and the reason for the change:
		// §5.2's list exists so a sign-off and an automated pass stay distinct.
		name: "a human sign-off and a machine re-confirmation are both kept",
		stored: `[{"by":"human:steve","at":"2026-07-01"},` +
			`{"by":"ci:nightly","at":"2026-08-01"}]`,
		tier: contextstore.HumanReviewed, valid: true,
	}, {
		name:   "a bare actor class counts",
		stored: `[{"by":"human"}]`, tier: contextstore.HumanReviewed, valid: true,
	}, {
		// An actorless event is the shape a fabricated one would take. Treating it as
		// evidence would make the fold worth defeating.
		name:   "an event with no actor confirms nothing and is invalid",
		stored: `[{"at":"2026-08-01"}]`, tier: contextstore.Unverified, valid: false,
	}, {
		name:   "an empty event list is unverified",
		stored: `[]`, tier: contextstore.Unverified, valid: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			wantTier(t, tt.stored, tt.tier, tt.valid)
		})
	}
}

// TestTrustRoundTripsTheShapeItRead guards against a writer added later silently
// rewriting every unit in the store into the other form.
func TestTrustRoundTripsTheShapeItRead(t *testing.T) {
	t.Parallel()
	for _, stored := range []string{
		`"human-reviewed"`,
		`""`,
		`[{"by":"human:steve","at":"2026-08-01"}]`,
		`[{"by":"ci:nightly"}]`,
	} {
		t.Run(stored, func(t *testing.T) {
			t.Parallel()
			var got contextstore.Trust
			if err := json.Unmarshal([]byte(stored), &got); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			out, err := json.Marshal(got)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			if string(out) != stored {
				t.Errorf("round trip = %s, want %s", out, stored)
			}
		})
	}
}

// TestTrustJoinsWithFreshness is the whole point in one assertion: a human-signed unit
// that the integrity log has condemned routes as unverified, and neither the events nor
// the authored judgement is destroyed to make that happen.
func TestTrustJoinsWithFreshness(t *testing.T) {
	t.Parallel()
	units := []contextstore.Unit{{
		ID:        "signed",
		Integrity: "check",
		Verified: contextstore.Trust{Events: []contextstore.Verification{
			{By: "human:steve", At: "2026-07-01"},
		}},
	}}
	if tier := units[0].EffectiveTier(); tier != contextstore.HumanReviewed {
		t.Fatalf("before any record, effective tier = %q, want human-reviewed", tier)
	}
	contextstore.ApplyFreshness(units, []contextstore.IntegrityRecord{
		{Unit: "signed", Result: contextstore.ResultDrift},
	})
	if tier := units[0].EffectiveTier(); tier != contextstore.Unverified {
		t.Errorf("after recorded drift, effective tier = %q, want unverified", tier)
	}
	if tier := units[0].Verified.Tier(); tier != contextstore.HumanReviewed {
		t.Errorf("the events were destroyed (tier now %q); drift only suppresses", tier)
	}
}
