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

// Unit is one routable piece of context: a runbook, skill, domain note, or an
// executable nonfunctional-requirement check. It routes by its labels and the
// repository paths it governs. ContentPath is the store-relative route to the
// unit's text — routing previews the unit and the worker pulls the text just in
// time (§10.4); Provenance is the source it derives from. Both are optional: a
// metadata-only unit routes but carries no text of its own.
type Unit struct {
	ID          string   `json:"id"`
	Kind        string   `json:"kind"`
	Labels      []string `json:"labels,omitempty"`
	Paths       []string `json:"paths,omitempty"`
	Owner       string   `json:"owner,omitempty"`
	ContentPath string   `json:"content_path,omitempty"`
	Provenance  string   `json:"provenance,omitempty"`
}

// scored pairs a unit with its routing score for ranking.
type scored struct {
	unit  Unit
	score int
}

// Route selects up to maxUnits whose labels or governed paths match the arc's
// labels or touched paths, most-specific (highest match count) first, ties
// broken by ID. It never mutates its inputs.
func Route(units []Unit, labels, paths []string, maxUnits int) []Unit {
	want := make(map[string]bool, len(labels))
	for _, l := range labels {
		want[l] = true
	}
	ranked := make([]scored, 0, len(units))
	for i := range units {
		if s := matchScore(&units[i], want, paths); s > 0 {
			ranked = append(ranked, scored{unit: units[i], score: s})
		}
	}
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].score != ranked[j].score {
			return ranked[i].score > ranked[j].score
		}
		return ranked[i].unit.ID < ranked[j].unit.ID
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
