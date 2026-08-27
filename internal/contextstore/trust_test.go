package contextstore_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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

// wantRecordRefused asserts RecordVerification refuses a stored unit, and why.
func wantRecordRefused(t *testing.T, stored, wantIn string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "u1.json")
	if err := os.WriteFile(path, []byte(stored), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	err := contextstore.RecordVerification(path, contextstore.Verification{By: "human:s"})
	if err == nil {
		t.Fatalf("RecordVerification(%s) succeeded, want a refusal", stored)
	}
	if !strings.Contains(err.Error(), wantIn) {
		t.Errorf("message %q does not contain %q", err, wantIn)
	}
}

func TestRecordVerificationAppendsAndPreserves(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "u1.json")
	// A field adh does not model. A store is a team's knowledge base, and a harness
	// that silently drops what it does not understand is one nobody can extend.
	stored := `{"id":"u1","kind":"note","house_field":{"a":1}}`
	if err := os.WriteFile(path, []byte(stored), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	first := contextstore.Verification{By: "human:steve", At: "2026-07-01"}
	if err := contextstore.RecordVerification(path, first); err != nil {
		t.Fatalf("RecordVerification: %v", err)
	}
	second := contextstore.Verification{By: "ci:nightly", At: "2026-08-01"}
	if err := contextstore.RecordVerification(path, second); err != nil {
		t.Fatalf("RecordVerification: %v", err)
	}
	var raw map[string]json.RawMessage
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := raw["house_field"]; !ok {
		t.Error("an unmodelled field was dropped by the write")
	}
	var trust contextstore.Trust
	if err := json.Unmarshal(raw["verified"], &trust); err != nil {
		t.Fatalf("verified: %v", err)
	}
	if len(trust.Events) != 2 {
		t.Fatalf("events = %d, want both appended", len(trust.Events))
	}
	// Both kept, and the human one still decides: the case a stored tier could not
	// represent at all.
	if tier := trust.Tier(); tier != contextstore.HumanReviewed {
		t.Errorf("tier = %q, want human-reviewed", tier)
	}
}

func TestRecordVerificationRefusesALegacyTier(t *testing.T) {
	t.Parallel()
	wantRecordRefused(t,
		`{"id":"u1","verified":"human-reviewed"}`,
		"convert it to a verification list")
}
