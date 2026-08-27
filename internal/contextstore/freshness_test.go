package contextstore_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/contextstore"
)

// write puts one file in dir, failing the test rather than the code under test.
func write(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

// wantFreshness applies the records to one unit and asserts the derived state. The
// helper exists so the table below reads as the cases it covers rather than as setup.
func wantFreshness(
	t *testing.T,
	unit *contextstore.Unit,
	records []contextstore.IntegrityRecord,
	want contextstore.Freshness,
	wantTier contextstore.TrustTier,
) {
	t.Helper()
	got := []contextstore.Unit{*unit}
	contextstore.ApplyFreshness(got, records)
	if got[0].Fresh != want {
		t.Errorf("freshness = %q, want %q", got[0].Fresh, want)
	}
	if tier := got[0].EffectiveTier(); tier != wantTier {
		t.Errorf("effective tier = %q, want %q", tier, wantTier)
	}
	if got[0].Verified.Tier() != unit.Verified.Tier() {
		t.Errorf("authored tier changed to %q, want %q left alone",
			got[0].Verified.Tier(), unit.Verified.Tier())
	}
}

func TestFreshnessStates(t *testing.T) {
	t.Parallel()
	checked := contextstore.Unit{
		ID:        "u1",
		Integrity: "drift-check",
		Verified:  contextstore.Trust{Stated: contextstore.HumanReviewed},
	}
	tests := []struct {
		name     string
		unit     contextstore.Unit
		records  []contextstore.IntegrityRecord
		want     contextstore.Freshness
		wantTier contextstore.TrustTier
	}{
		{
			// The zero value must assert nothing: a store nobody has verified routes
			// exactly as it did before this existed.
			name: "no record is unknown, and suppresses nothing",
			unit: checked, records: nil,
			want: contextstore.FreshnessUnknown, wantTier: contextstore.HumanReviewed,
		},
		{
			name: "a passing run is fresh",
			unit: checked,
			records: []contextstore.IntegrityRecord{
				{Unit: "u1", Result: contextstore.ResultOK},
			},
			want: contextstore.FreshnessFresh, wantTier: contextstore.HumanReviewed,
		},
		{
			// The defect this whole file exists for.
			name: "a drifted unit routes as unverified",
			unit: checked,
			records: []contextstore.IntegrityRecord{
				{Unit: "u1", Result: contextstore.ResultDrift},
			},
			want: contextstore.FreshnessDrifted, wantTier: contextstore.Unverified,
		},
		{
			// A tool that could not run has neither cleared nor condemned the unit, and
			// must not be read as either.
			name: "a tool that could not run leaves the unit unknown",
			unit: checked,
			records: []contextstore.IntegrityRecord{
				{Unit: "u1", Result: contextstore.ResultUnverified},
			},
			want: contextstore.FreshnessUnknown, wantTier: contextstore.HumanReviewed,
		},
		{
			// §12: a unit declaring no integrity tool is outside the check's judgement,
			// not failing it. Scoring it would make every hand-written unit look suspect.
			name: "no integrity tool is not applicable, not unknown",
			unit: contextstore.Unit{
				ID:       "u1",
				Verified: contextstore.Trust{Stated: contextstore.HumanReviewed},
			},
			records: []contextstore.IntegrityRecord{
				{Unit: "u1", Result: contextstore.ResultDrift},
			},
			want:     contextstore.FreshnessNotApplicable,
			wantTier: contextstore.HumanReviewed,
		},
		{
			// Append-only: the log's own order is the recency, so a re-verified unit
			// recovers with no edit to the authored field.
			name: "the newest record wins, so re-verifying restores standing",
			unit: checked,
			records: []contextstore.IntegrityRecord{
				{Unit: "u1", Result: contextstore.ResultDrift},
				{Unit: "u1", Result: contextstore.ResultOK},
			},
			want: contextstore.FreshnessFresh, wantTier: contextstore.HumanReviewed,
		},
		{
			name: "a record for another unit does not touch this one",
			unit: checked,
			records: []contextstore.IntegrityRecord{
				{Unit: "other", Result: contextstore.ResultDrift},
			},
			want: contextstore.FreshnessUnknown, wantTier: contextstore.HumanReviewed,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			wantFreshness(t, &tt.unit, tt.records, tt.want, tt.wantTier)
		})
	}
}

// TestDriftedUnitStopsOutrankingAClean one is the end-to-end statement of the defect:
// the tier is a routing tie-break, so suppression only matters if it changes the order.
func TestDriftedUnitStopsOutrankingAClean(t *testing.T) {
	t.Parallel()
	both := []contextstore.Unit{
		{
			ID: "reviewed", Labels: []string{"sec"},
			Integrity: "check", Verified: contextstore.Trust{Stated: contextstore.HumanReviewed},
		},
		{
			ID:       "plain",
			Labels:   []string{"sec"},
			Verified: contextstore.Trust{Stated: contextstore.MachineConfirmed},
		},
	}
	if got := contextstore.Route(both, []string{"sec"}, nil, 0); got[0].ID != "reviewed" {
		t.Fatalf("before any record, routed %q first, want reviewed", got[0].ID)
	}
	contextstore.ApplyFreshness(both, []contextstore.IntegrityRecord{
		{Unit: "reviewed", Result: contextstore.ResultDrift},
	})
	got := contextstore.Route(both, []string{"sec"}, nil, 0)
	if got[0].ID != "plain" {
		t.Fatalf("after recorded drift, routed %q first, want plain", got[0].ID)
	}
	if got[1].Verified.Tier() != contextstore.HumanReviewed {
		t.Errorf("drift destroyed the authored tier (%q); it must only be suppressed",
			got[1].Verified.Tier())
	}
}

// TestFreshIsNeverSerialised guards the structural half of the design: the suppression
// cannot be written back to disk, so a round-trip cannot bake it in.
func TestFreshIsNeverSerialised(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	unit := contextstore.Unit{
		ID:        "u1",
		Integrity: "check",
		Verified:  contextstore.Trust{Stated: contextstore.HumanReviewed},
	}
	contextstore.ApplyFreshness([]contextstore.Unit{unit}, nil)
	write(t, dir, "u1.json", `{"id":"u1","integrity":"check","verified":"human-reviewed"}`)
	loaded, err := contextstore.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded[0].Fresh != contextstore.FreshnessUnknown {
		t.Errorf("Load produced freshness %q; it must come only from the log",
			loaded[0].Fresh)
	}
}

func TestLoadFreshJoinsThePairedLog(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "context")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	write(t, dir, "u1.json", `{"id":"u1","integrity":"check","verified":"human-reviewed"}`)
	err := contextstore.AppendIntegrity(
		contextstore.IntegrityLogFor(dir),
		contextstore.IntegrityRecord{Unit: "u1", Result: contextstore.ResultDrift},
	)
	if err != nil {
		t.Fatalf("AppendIntegrity: %v", err)
	}
	units, err := contextstore.LoadFresh(dir)
	if err != nil {
		t.Fatalf("LoadFresh: %v", err)
	}
	if units[0].Fresh != contextstore.FreshnessDrifted {
		t.Errorf("LoadFresh freshness = %q, want drifted", units[0].Fresh)
	}
	if tier := units[0].EffectiveTier(); tier != contextstore.Unverified {
		t.Errorf("effective tier = %q, want unverified", tier)
	}
}

// TestIntegrityLogPairsWithTheDefaultStore is a ratchet over the two constants: the
// pairing is a naming convention, so changing either name without the other would
// silently split the writer from the reader.
func TestIntegrityLogPairsWithTheDefaultStore(t *testing.T) {
	t.Parallel()
	got := contextstore.IntegrityLogFor(contextstore.DefaultStoreDir)
	if got != contextstore.IntegrityFile {
		t.Errorf("IntegrityLogFor(%q) = %q, want %q — the constants have drifted apart",
			contextstore.DefaultStoreDir, got, contextstore.IntegrityFile)
	}
}

func TestLoadIntegrityAbsentAndCorrupt(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	got, err := contextstore.LoadIntegrity(filepath.Join(dir, "nope.jsonl"))
	if err != nil || len(got) != 0 {
		t.Fatalf("absent log = (%v, %v), want no records and no error", got, err)
	}
	write(t, dir, "bad.jsonl", "{not json}\n")
	_, err = contextstore.LoadIntegrity(filepath.Join(dir, "bad.jsonl"))
	if err == nil {
		t.Fatal("a corrupt line was swallowed; the log's integrity is its value")
	}
	var adhErr *adh.Error
	if !errors.As(err, &adhErr) {
		t.Errorf("corrupt line returned %T, want *adh.Error", err)
	}
}
