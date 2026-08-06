package critic

import (
	"slices"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
)

// PrecisionFile is the conventional repo path the critic precision log lives under
// (§19): an append-only record of each adjudication's confirmed and unconfirmed
// finding kinds. It is the precision mirror of the coverage log — where coverage
// steers the critic toward kinds it under-covers, precision surfaces the kinds it
// over-flags (a high false-positive rate), so the next critic is held to a higher
// bar on them.
const PrecisionFile = ".adh/critic-precision.jsonl"

// Default thresholds for NoisyKinds: a kind is judged noisy only after at least
// DefaultMinAdjudications findings of that kind have been adjudicated (so a small
// sample cannot condemn it), and only when more than DefaultMaxFalsePositiveRate of
// them were unconfirmed.
const (
	DefaultMinAdjudications     = 5
	DefaultMaxFalsePositiveRate = 0.5
)

// PrecisionEntry is one adjudication's outcome by finding kind: the kinds confirmed
// (the named artifact ran and failed — a real defect) and the kinds unconfirmed
// (surfaced but not reproduced — a false positive). Kinds are recorded with
// multiplicity, one per finding, so the false-positive rate counts findings.
type PrecisionEntry struct {
	Arc         string   `json:"arc"`
	Confirmed   []string `json:"confirmed,omitempty"`
	Unconfirmed []string `json:"unconfirmed,omitempty"`
}

// VerdictKinds splits a disposed verdict into the confirmed and unconfirmed finding
// kinds, one entry per finding (with multiplicity). Pure.
func VerdictKinds(v *Verdict) (confirmed, unconfirmed []string) {
	confirmed = make([]string, 0, len(v.Confirmed))
	for i := range v.Confirmed {
		confirmed = append(confirmed, string(v.Confirmed[i].Kind))
	}
	unconfirmed = make([]string, 0, len(v.Unconfirmed))
	for i := range v.Unconfirmed {
		unconfirmed = append(unconfirmed, string(v.Unconfirmed[i].Kind))
	}
	return confirmed, unconfirmed
}

// NoisyKinds returns the finding kinds whose adjudicated false-positive rate is too
// high to trust (§19): a kind adjudicated at least minSamples times whose unconfirmed
// share exceeds maxFPR. The next critic holds these to a higher bar. A kind below the
// sample floor is never condemned. Sorted, deterministic. Pure.
func NoisyKinds(entries []PrecisionEntry, minSamples int, maxFPR float64) []adh.FindingKind {
	type tally struct{ confirmed, unconfirmed int }
	byKind := make(map[string]*tally)
	count := func(kinds []string, unconfirmed bool) {
		for _, k := range kinds {
			t, ok := byKind[k]
			if !ok {
				t = &tally{}
				byKind[k] = t
			}
			if unconfirmed {
				t.unconfirmed++
			} else {
				t.confirmed++
			}
		}
	}
	for i := range entries {
		count(entries[i].Confirmed, false)
		count(entries[i].Unconfirmed, true)
	}
	noisy := make([]adh.FindingKind, 0, len(byKind))
	for kind, t := range byKind {
		total := t.confirmed + t.unconfirmed
		if total < minSamples {
			continue
		}
		if float64(t.unconfirmed)/float64(total) > maxFPR {
			noisy = append(noisy, adh.FindingKind(kind))
		}
	}
	slices.Sort(noisy)
	return noisy
}

// AppendPrecision records one adjudication's confirmed/unconfirmed kinds to the
// append-only log at path. An entry with nothing adjudicated (a clean review) is a
// no-op, so it does not pollute the history.
func AppendPrecision(path string, entry *PrecisionEntry) error {
	if len(entry.Confirmed) == 0 && len(entry.Unconfirmed) == 0 {
		return nil
	}
	return appendJSONL("critic.AppendPrecision", path, entry)
}

// LoadPrecision reads the precision log at path. An absent file is no entries, not
// an error; a corrupt line is a hard error.
func LoadPrecision(path string) ([]PrecisionEntry, error) {
	return loadJSONL[PrecisionEntry]("critic.LoadPrecision", path)
}
