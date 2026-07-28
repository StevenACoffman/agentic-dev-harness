// Package contextstore holds the just-in-time context units the harness routes
// to stages (SPEC-ADDITIONS §10): a large navigable store and a small active
// working set selected by an arc's labels and touched paths. Routing is a pure
// function; Load is the thin I/O shell.
package contextstore

import (
	"encoding/json"
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
// repository paths it governs.
type Unit struct {
	ID     string   `json:"id"`
	Kind   string   `json:"kind"`
	Labels []string `json:"labels,omitempty"`
	Paths  []string `json:"paths,omitempty"`
	Owner  string   `json:"owner,omitempty"`
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
	for _, unit := range units {
		if s := matchScore(&unit, want, paths); s > 0 {
			ranked = append(ranked, scored{unit: unit, score: s})
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
	for i, r := range ranked {
		out[i] = r.unit
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
