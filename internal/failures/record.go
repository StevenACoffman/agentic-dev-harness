package failures

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/skillet/atomicfile"
)

// Root-cause categories (§19.2 triage) — derived deterministically from the arc's
// grounding at failure time, so a human reading the log sees whether to fix routing
// or fix the routed content.
const (
	RootUngrounded   = "ungrounded"    // failed with no routed context — fix routing
	RootGroundedMiss = "grounded-miss" // failed despite routed context — fix content
)

// Record is one disposed critic finding, stamped so accretion can gate on it (§19.2):
// Class is the finding kind it grouped under, Stratum the year-month it occurred in
// (the §11 temporal gate), Labels/Paths the arc's routing scope (so a promoted lesson
// routes to where it was learned), and RootCause the deterministic triage of the
// attempt's grounding.
type Record struct {
	Class     string   `json:"class"`
	Stratum   string   `json:"stratum,omitempty"`
	Labels    []string `json:"labels,omitempty"`
	Paths     []string `json:"paths,omitempty"`
	RootCause string   `json:"root_cause,omitempty"`
}

// ClassifyRootCause triages a failed attempt by whether it was grounded — whether any
// context was routed to it (§19.2). A grounded miss points at the content; an
// ungrounded one points at routing.
func ClassifyRootCause(grounded bool) string {
	if grounded {
		return RootGroundedMiss
	}
	return RootUngrounded
}

// LoadRecords reads the stamped failure-record log at path. A missing file is empty,
// not an error; a malformed one is EINVALID.
func LoadRecords(path string) ([]Record, error) {
	const op = "failures.LoadRecords"
	data, err := os.ReadFile(path)
	switch {
	case os.IsNotExist(err):
		return nil, nil
	case err != nil:
		return nil, &adh.Error{Op: op, Err: err}
	}
	var recs []Record
	if err := json.Unmarshal(data, &recs); err != nil {
		return nil, &adh.Error{Op: op, Err: err}
	}
	return recs, nil
}

// AppendRecords adds records to the log at path, creating it and its directory if
// absent, and rewrites the file atomically. Appending nothing is a no-op, so it never
// creates an empty log.
func AppendRecords(path string, recs ...Record) error {
	const op = "failures.AppendRecords"
	if len(recs) == 0 {
		return nil
	}
	existing, err := LoadRecords(path)
	if err != nil {
		return err
	}
	existing = append(existing, recs...)
	data, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return &adh.Error{Op: op, Err: err}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return &adh.Error{Op: op, Err: err}
	}
	if err := atomicfile.WriteFile(path, append(data, '\n'), 0o600); err != nil {
		return &adh.Error{Op: op, Err: err}
	}
	return nil
}

// StrataCount maps each class to the number of distinct time strata it recurred
// across (§11 temporal gate). Records with no stratum contribute nothing. Pure.
func StrataCount(recs []Record) map[string]int {
	strata := make(map[string]map[string]bool)
	for i := range recs {
		if recs[i].Stratum == "" {
			continue
		}
		set := strata[recs[i].Class]
		if set == nil {
			set = make(map[string]bool)
			strata[recs[i].Class] = set
		}
		set[recs[i].Stratum] = true
	}
	counts := make(map[string]int, len(strata))
	for class, set := range strata {
		counts[class] = len(set)
	}
	return counts
}

// ScopeFor returns the distinct labels and paths a class recurred under, sorted — the
// routing scope a promoted lesson should carry so it routes to where it was learned,
// not globally (§19.2 scope-tagging). Pure.
func ScopeFor(recs []Record, class string) (labels, paths []string) {
	labelSet := make(map[string]bool)
	pathSet := make(map[string]bool)
	for i := range recs {
		if recs[i].Class != class {
			continue
		}
		for _, label := range recs[i].Labels {
			labelSet[label] = true
		}
		for _, path := range recs[i].Paths {
			pathSet[path] = true
		}
	}
	return sortedKeys(labelSet), sortedKeys(pathSet)
}

// RootCauseCounts tallies a class's records by root cause (§19.2 triage), so a human
// deciding whether to promote sees whether the class is a routing or a content
// problem. Pure.
func RootCauseCounts(recs []Record, class string) map[string]int {
	counts := make(map[string]int)
	for i := range recs {
		if recs[i].Class == class && recs[i].RootCause != "" {
			counts[recs[i].RootCause]++
		}
	}
	return counts
}

// sortedKeys returns the set's keys in sorted order.
func sortedKeys(set map[string]bool) []string {
	keys := make([]string, 0, len(set))
	for k := range set {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
