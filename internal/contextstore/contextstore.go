// Package contextstore holds the just-in-time context units the harness routes
// to stages (SPEC-ADDITIONS §10): a large navigable store and a small active
// working set selected by an arc's labels and touched paths. Routing is a pure
// function; Load is the thin I/O shell.
package contextstore

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
)

const (
	// DefaultStoreDir is the conventional repo path the context store lives under
	// (SPEC-ADDITIONS §10).
	DefaultStoreDir = ".adh/context"
	// DefaultWorkingSet is the default cap on how many units Route returns for a
	// single stage's working set.
	DefaultWorkingSet = 8
)

// Trust tiers (§10.4, the OKF `verified` dimension): how a unit's content was earned,
// so routing can weight a human-reviewed unit over an unverified one. An empty tier is
// treated as Unverified.
const (
	Unverified       TrustTier = "unverified"        // agent-proposed, not confirmed
	MachineConfirmed TrustTier = "machine-confirmed" // a check/tool confirmed it
	HumanReviewed    TrustTier = "human-reviewed"    // a human reviewed and accepted it
)

// TrustTier records how a unit's content was earned (§10.4).
type TrustTier string

// Claim is one verifiable citation inside a unit (§10.4 receipt verification): a
// Quote the unit asserts and the Source it should be found in. When Source is a repo
// path, `context verify` traces the quote back to the file — the receipt half of
// provenance, beyond the path merely resolving.
type Claim struct {
	Quote  string `json:"quote"`
	Source string `json:"source"`
}

// Unit is one routable piece of context: a runbook, skill, domain note, or an
// executable nonfunctional-requirement check. It routes by its labels and the
// repository paths it governs. ContentPath is the store-relative route to the
// unit's text — routing previews the unit and the worker pulls the text just in
// time (§10.4); Provenance is a one-line summary and Sources are the proven origins
// it derives from; Integrity is the §13 tool id that proves the unit has not drifted
// (`context verify`, §10.4 anti-drift). Verified is the trust tier (weights routing);
// SupersededBy is the unit id that has replaced this one (a superseded unit no longer
// routes). Claims are verifiable quote/source citations traced back to their source
// (§10.4 receipt verification). The optional fields let a metadata-only unit route
// with no text of its own.
type Unit struct {
	ID           string    `json:"id"`
	Kind         string    `json:"kind"`
	Labels       []string  `json:"labels,omitempty"`
	Paths        []string  `json:"paths,omitempty"`
	Owner        string    `json:"owner,omitempty"`
	ContentPath  string    `json:"content_path,omitempty"`
	Provenance   string    `json:"provenance,omitempty"`
	Sources      []string  `json:"sources,omitempty"`
	Claims       []Claim   `json:"claims,omitempty"`
	Integrity    string    `json:"integrity,omitempty"`
	Verified     TrustTier `json:"verified,omitempty"`
	SupersededBy string    `json:"superseded_by,omitempty"`
}

// scored pairs a unit with its routing score for ranking.
type scored struct {
	unit  Unit
	score int
}

// Valid reports whether t is a known trust tier; an empty tier is valid (it defaults
// to Unverified).
func (t TrustTier) Valid() bool {
	switch t {
	case "", Unverified, MachineConfirmed, HumanReviewed:
		return true
	default:
		return false
	}
}

// Rank orders trust for routing tie-breaks (§10.4): human-reviewed (2) outranks
// machine-confirmed (1) outranks unverified/unset (0).
func (t TrustTier) Rank() int {
	switch t {
	case HumanReviewed:
		return 2
	case MachineConfirmed:
		return 1
	default:
		return 0
	}
}

// Route selects up to maxUnits whose labels or governed paths match the arc's
// labels or touched paths, most-specific (highest match count) first; ties break by
// trust tier (a human-reviewed unit outranks an unverified one, §10.4), then by ID. A
// superseded unit never routes — it has been replaced. It never mutates its inputs.
func Route(units []Unit, labels, paths []string, maxUnits int) []Unit {
	want := make(map[string]bool, len(labels))
	for _, l := range labels {
		want[l] = true
	}
	ranked := make([]scored, 0, len(units))
	for i := range units {
		if units[i].SupersededBy != "" {
			continue
		}
		if s := matchScore(&units[i], want, paths); s > 0 {
			ranked = append(ranked, scored{unit: units[i], score: s})
		}
	}
	sort.Slice(ranked, func(i, j int) bool {
		return rankBefore(&ranked[i], &ranked[j])
	})
	if maxUnits > 0 && len(ranked) > maxUnits {
		ranked = ranked[:maxUnits]
	}
	out := make([]Unit, len(ranked))
	for i := range ranked {
		out[i] = ranked[i].unit
	}
	return out
}

// rankBefore orders two routed units: higher match score first, then higher trust
// tier (§10.4), then lexical id — the total order Route sorts by.
func rankBefore(a, b *scored) bool {
	switch {
	case a.score != b.score:
		return a.score > b.score
	case a.unit.Verified.Rank() != b.unit.Verified.Rank():
		return a.unit.Verified.Rank() > b.unit.Verified.Rank()
	default:
		return a.unit.ID < b.unit.ID
	}
}

// matchScore counts label hits plus path hits for one unit.
func matchScore(unit *Unit, want map[string]bool, paths []string) int {
	score := 0
	for _, l := range unit.Labels {
		if want[l] {
			score++
		}
	}
	for _, up := range unit.Paths {
		for _, ap := range paths {
			if ap == up || strings.HasPrefix(ap, up+"/") {
				score++
			}
		}
	}
	return score
}

// DuplicateIDs returns the unit ids that appear more than once, sorted and each
// reported once. A duplicate id makes routing ambiguous (two units answer to the
// same name), so it is a cross-unit consistency defect (§10.4). It is pure.
func DuplicateIDs(units []Unit) []string {
	counts := make(map[string]int, len(units))
	for i := range units {
		counts[units[i].ID]++
	}
	dups := make([]string, 0)
	for id, n := range counts {
		if n > 1 {
			dups = append(dups, id)
		}
	}
	sort.Strings(dups)
	return dups
}

// AreaLabels derives coarse area labels from repository paths: the top-level
// directory of each (e.g. "cmd/step/x.go" -> "cmd"), deduplicated and sorted. A
// top-level file (no directory) yields no label. Execution labels an arc by the
// areas it touched so the cold critic's context can route (§19.1); this mirrors
// the top-level-dir keying that `adh init` scaffolds context units by.
func AreaLabels(paths []string) []string {
	seen := make(map[string]bool, len(paths))
	labels := make([]string, 0, len(paths))
	for _, path := range paths {
		area := topDir(path)
		if area == "" || seen[area] {
			continue
		}
		seen[area] = true
		labels = append(labels, area)
	}
	sort.Strings(labels)
	return labels
}

// topDir returns the first path segment when path is under a directory, or "" for
// a top-level file (which has no area).
func topDir(path string) string {
	path = strings.TrimPrefix(path, "./")
	if i := strings.IndexByte(path, '/'); i >= 0 {
		return path[:i]
	}
	return ""
}

// Content reads the unit's text from its ContentPath, resolved under dir (the
// store directory). It is the I/O shell to routing's pure core: an empty
// ContentPath yields no content and no error (a metadata-only unit), while a set
// path that does not resolve is an error — the routing promised text that is not
// there. The path is cleaned and kept within dir so a unit cannot route to a file
// outside the store.
func Content(dir string, unit *Unit) (string, error) {
	const op = "contextstore.Content"
	if unit.ContentPath == "" {
		return "", nil
	}
	clean := filepath.Clean(unit.ContentPath)
	if filepath.IsAbs(clean) || clean == ".." ||
		strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", &adh.Error{
			Op:  op,
			Err: fmt.Errorf("content_path %q escapes the store", unit.ContentPath),
		}
	}
	data, err := os.ReadFile(filepath.Join(dir, clean))
	if err != nil {
		return "", &adh.Error{Op: op, Err: err}
	}
	return string(data), nil
}

// Load reads context units from JSON files under dir. An absent directory
// yields no units and no error.
func Load(dir string) ([]Unit, error) {
	const op = "contextstore.Load"
	entries, err := os.ReadDir(dir)
	switch {
	case os.IsNotExist(err):
		return []Unit{}, nil
	case err != nil:
		return nil, &adh.Error{Op: op, Err: err}
	}
	units := make([]Unit, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		data, readErr := os.ReadFile(filepath.Join(dir, entry.Name()))
		if readErr != nil {
			return nil, &adh.Error{Op: op, Err: readErr}
		}
		var unit Unit
		if err := json.Unmarshal(data, &unit); err != nil {
			return nil, &adh.Error{Op: op, Err: err}
		}
		units = append(units, unit)
	}
	return units, nil
}
