package critic_test

import (
	"path/filepath"
	"reflect"
	"strconv"
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/critic"
)

func TestVerdictKinds(t *testing.T) {
	t.Parallel()
	v := &critic.Verdict{
		Confirmed:   []adh.Finding{{Kind: adh.FindingOracle}, {Kind: adh.FindingNFR}},
		Unconfirmed: []adh.Finding{{Kind: adh.FindingOracle}},
	}
	confirmed, unconfirmed := critic.VerdictKinds(v)
	if want := []string{
		string(adh.FindingOracle),
		string(adh.FindingNFR),
	}; !reflect.DeepEqual(
		confirmed,
		want,
	) {
		t.Errorf("confirmed = %v, want %v", confirmed, want)
	}
	if want := []string{string(adh.FindingOracle)}; !reflect.DeepEqual(unconfirmed, want) {
		t.Errorf("unconfirmed = %v, want %v", unconfirmed, want)
	}
}

// TestNoisyKinds: a kind over the FPR threshold with enough samples is flagged; a
// low-FPR kind and an under-sampled kind are both spared.
func TestNoisyKinds(t *testing.T) {
	t.Parallel()
	oracle, nfr, device := string(
		adh.FindingOracle,
	), string(
		adh.FindingNFR,
	), string(
		adh.FindingDevice,
	)
	entries := []critic.PrecisionEntry{
		// oracle: 4 unconfirmed + 2 confirmed over 6 -> FPR .667 > .5, n>=5 -> noisy
		{
			Confirmed:   []string{oracle, oracle},
			Unconfirmed: []string{oracle, oracle, oracle, oracle},
		},
		// nfr: 2 unconfirmed + 4 confirmed -> FPR .333 -> not noisy
		{Confirmed: []string{nfr, nfr, nfr, nfr}, Unconfirmed: []string{nfr, nfr}},
		// device: 3 unconfirmed, 0 confirmed -> FPR 1.0 but n=3 < 5 -> spared
		{Unconfirmed: []string{device, device, device}},
	}
	got := critic.NoisyKinds(
		entries,
		critic.DefaultMinAdjudications,
		critic.DefaultMaxFalsePositiveRate,
	)
	want := []adh.FindingKind{adh.FindingOracle}
	if !reflect.DeepEqual(got, want) {
		t.Errorf(
			"NoisyKinds = %v, want %v (oracle over threshold; nfr low-FPR; device low-N)",
			got,
			want,
		)
	}
}

// TestPrecisionRoundTrip: a clean review (nothing adjudicated) is a no-op; a real
// entry round-trips through append and load.
func TestPrecisionRoundTrip(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "critic-precision.jsonl")
	if err := critic.AppendPrecision(path, &critic.PrecisionEntry{Arc: "arc-0000"}); err != nil {
		t.Fatalf("append empty: %v", err)
	}
	entry := &critic.PrecisionEntry{
		Arc:         "arc-0001",
		Confirmed:   []string{string(adh.FindingOracle)},
		Unconfirmed: []string{string(adh.FindingNFR)},
	}
	if err := critic.AppendPrecision(path, entry); err != nil {
		t.Fatalf("append: %v", err)
	}
	got, err := critic.LoadPrecision(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if want := []critic.PrecisionEntry{*entry}; !reflect.DeepEqual(got, want) {
		t.Errorf("round-trip = %v, want %v (empty entry skipped)", got, want)
	}
}

// TestAnUncheckedFindingIsNotAFalsePositive is the fix for the damage that made this
// Tier 1. Unconfirmed feeds PrecisionEntry, whose doc calls it a false positive, and
// NoisyKinds then condemns any kind whose rate is too high — so a missing or unbuilt
// artifact used to teach the harness to distrust an entire category of real defect,
// and the more broken the environment the more it distrusted.
func TestAnUncheckedFindingIsNotAFalsePositive(t *testing.T) {
	t.Parallel()
	v := critic.Dispose([]critic.Adjudicated{
		{Finding: adh.Finding{Summary: "s", Kind: adh.FindingNFR}, Ran: false},
	})

	confirmed, unconfirmed := critic.VerdictKinds(&v)
	// Neither slice, deliberately. Dropping it from the numerator alone would leave
	// it in the denominator and understate the rate instead of correcting it.
	if len(confirmed) != 0 || len(unconfirmed) != 0 {
		t.Fatalf("confirmed=%v unconfirmed=%v, want an unchecked finding in neither",
			confirmed, unconfirmed)
	}
}

// TestAnUnrunnableArtifactCannotCondemnItsKind is the same property at the level a
// person would notice: enough unchecked findings of one kind must not make NoisyKinds
// hold the next critic to a higher bar for it.
func TestAnUnrunnableArtifactCannotCondemnItsKind(t *testing.T) {
	t.Parallel()
	entries := make([]critic.PrecisionEntry, 0, 20)
	for i := range 20 {
		v := critic.Dispose([]critic.Adjudicated{
			{Finding: adh.Finding{Summary: "s", Kind: adh.FindingNFR}, Ran: false},
		})
		confirmed, unconfirmed := critic.VerdictKinds(&v)
		entries = append(entries, critic.PrecisionEntry{
			Arc: "a" + strconv.Itoa(i), Confirmed: confirmed, Unconfirmed: unconfirmed,
		})
	}

	if got := critic.NoisyKinds(entries, 5, 0.5); len(got) != 0 {
		t.Errorf("NoisyKinds = %v; twenty unrunnable artifacts condemned the kind", got)
	}
}
