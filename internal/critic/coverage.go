package critic

import (
	"sort"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
)

// CoverageFile is the conventional repo path the critic coverage log lives under
// (§19/§10.3): an append-only record of which finding kinds each arc's critic
// surfaced, so the next critic can be steered toward the kinds it keeps missing —
// accretion applied to the critic lever, kept distinct from the evidence trail.
const CoverageFile = ".adh/critic-coverage.jsonl"

// CoverageEntry is one arc's critic coverage: the finding kinds it surfaced.
type CoverageEntry struct {
	Arc   string   `json:"arc"`
	Kinds []string `json:"kinds"`
}

// FindingKinds returns the sorted distinct kinds a critic turn's findings named, for
// recording an arc's coverage.
func FindingKinds(findings []adh.Finding) []string {
	seen := make(map[string]bool, len(findings))
	kinds := make([]string, 0, len(findings))
	for i := range findings {
		kind := string(findings[i].Kind)
		if kind != "" && !seen[kind] {
			seen[kind] = true
			kinds = append(kinds, kind)
		}
	}
	sort.Strings(kinds)
	return kinds
}

// UnderCovered returns the finding kinds the recorded critic history has covered
// least (§19): the kinds tied for the minimum surfaced-arc count across all entries,
// so an empty history returns every kind and a mature one returns the current blind
// spots. The next critic is steered to probe these. It is pure.
func UnderCovered(entries []CoverageEntry, all []adh.FindingKind) []adh.FindingKind {
	freq := make(map[adh.FindingKind]int, len(all))
	for _, kind := range all {
		freq[kind] = 0
	}
	for i := range entries {
		for _, name := range entries[i].Kinds {
			if _, ok := freq[adh.FindingKind(name)]; ok {
				freq[adh.FindingKind(name)]++
			}
		}
	}
	least := -1
	for _, kind := range all {
		if least < 0 || freq[kind] < least {
			least = freq[kind]
		}
	}
	under := make([]adh.FindingKind, 0, len(all))
	for _, kind := range all {
		if freq[kind] == least {
			under = append(under, kind)
		}
	}
	return under
}

// AppendCoverage records one arc's surfaced kinds to the append-only log at path,
// creating the parent directory. Recording nothing (no kinds) is a no-op, so an arc
// with no findings does not pollute the history.
func AppendCoverage(path string, entry *CoverageEntry) error {
	if len(entry.Kinds) == 0 {
		return nil
	}
	return appendJSONL("critic.AppendCoverage", path, entry)
}

// LoadCoverage reads the coverage log at path. An absent file is no entries, not an
// error. A corrupt line is a hard error — the log's integrity is its value.
func LoadCoverage(path string) ([]CoverageEntry, error) {
	return loadJSONL[CoverageEntry]("critic.LoadCoverage", path)
}
