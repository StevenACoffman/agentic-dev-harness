package critic

import (
	"encoding/json"
	"os"
	"path/filepath"
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
	const op = "critic.AppendCoverage"
	if len(entry.Kinds) == 0 {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return &adh.Error{Op: op, Err: err}
	}
	line, err := json.Marshal(entry)
	if err != nil {
		return &adh.Error{Op: op, Err: err}
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return &adh.Error{Op: op, Err: err}
	}
	defer func() { _ = file.Close() }()
	if _, err := file.Write(append(line, '\n')); err != nil {
		return &adh.Error{Op: op, Err: err}
	}
	return nil
}

// LoadCoverage reads the coverage log at path. An absent file is no entries, not an
// error. A corrupt line is a hard error — the log's integrity is its value.
func LoadCoverage(path string) ([]CoverageEntry, error) {
	const op = "critic.LoadCoverage"
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return []CoverageEntry{}, nil
	}
	if err != nil {
		return nil, &adh.Error{Op: op, Err: err}
	}
	entries := make([]CoverageEntry, 0)
	for _, line := range splitJSONLines(data) {
		var entry CoverageEntry
		if err := json.Unmarshal(line, &entry); err != nil {
			return nil, &adh.Error{Op: op, Err: err}
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

// splitJSONLines splits a JSONL byte slice into its non-empty lines.
func splitJSONLines(data []byte) [][]byte {
	lines := make([][]byte, 0)
	start := 0
	for i, b := range data {
		if b == '\n' {
			if i > start {
				lines = append(lines, data[start:i])
			}
			start = i + 1
		}
	}
	if start < len(data) {
		lines = append(lines, data[start:])
	}
	return lines
}
