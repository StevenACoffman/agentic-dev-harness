package contextstore

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
)

// MissFile is the conventional repo path the routing-miss log lives under. It is
// kept distinct from the evidence trail (§10.3): a miss is a learning signal — an
// arc that declared a footprint the store could not route — not an audit record.
const MissFile = ".adh/context-misses.jsonl"

// minStrata is how many distinct time strata (year-month) a key must be missed
// across before a route is proposed (§10.3): a temporal independence axis orthogonal
// to the count threshold, so a one-day burst of misses does not earn a route — only a
// pattern sustained across time does.
const minStrata = 2

// Miss is one recorded routing miss: an arc whose labels/paths routed no context
// (the §19.1 routing gap), stamped with the time Stratum (year-month) it occurred in.
// Accumulated misses are the signal that the store is missing a route the arcs keep
// asking for.
type Miss struct {
	Arc     string   `json:"arc"`
	Labels  []string `json:"labels,omitempty"`
	Paths   []string `json:"paths,omitempty"`
	Stratum string   `json:"stratum,omitempty"`
}

// RouteProposal is a proposed deterministic route the miss log has earned: a label
// or path the arcs missed on at least a threshold number of times across at least
// minStrata distinct time strata, so authoring a unit for it would convert the
// sustained miss into a route. Kind is "label" or "path"; Count is how many recorded
// misses named Key; Strata is how many distinct strata they spanned.
type RouteProposal struct {
	Key    string `json:"key"`
	Kind   string `json:"kind"`
	Count  int    `json:"count"`
	Strata int    `json:"strata"`
}

// keyStat accumulates a candidate route's miss count and the distinct time strata it
// was missed across, for the temporal-stratum gate.
type keyStat struct {
	count  int
	strata map[string]bool
}

// Stratum is the year-month time stratum a miss belongs to (§10.3). The shell passes
// time.Now(); keeping the format here and the clock in the shell leaves the routing
// core clock-free.
func Stratum(t time.Time) string {
	return t.UTC().Format("2006-01")
}

// AppendMiss records one routing miss to the append-only log at path, creating the
// parent directory and file if needed. It is the I/O shell to ProposeRoutes' pure
// core; a caller records misses so the router can later learn from them.
func AppendMiss(path string, miss *Miss) error {
	const op = "contextstore.AppendMiss"
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return &adh.Error{Op: op, Err: err}
	}
	line, err := json.Marshal(miss)
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

// LoadMisses reads the routing-miss log at path. An absent file is no misses, not
// an error. A corrupt line is a hard error — the log's integrity is its value.
func LoadMisses(path string) ([]Miss, error) {
	const op = "contextstore.LoadMisses"
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return []Miss{}, nil
	}
	if err != nil {
		return nil, &adh.Error{Op: op, Err: err}
	}
	misses := make([]Miss, 0)
	for _, line := range splitLines(data) {
		var miss Miss
		if err := json.Unmarshal(line, &miss); err != nil {
			return nil, &adh.Error{Op: op, Err: err}
		}
		misses = append(misses, miss)
	}
	return misses, nil
}

// ProposeRoutes turns the accumulated misses into route proposals: any label or
// path missed on at least threshold times AND across at least minStrata distinct time
// strata (§10.3 — a sustained pattern, not a one-day burst), ranked by recurrence
// (then kind, then key). It is pure — the caller decides what to do with a proposal
// (author a unit, gated at §11); nothing is auto-routed.
func ProposeRoutes(misses []Miss, threshold int) []RouteProposal {
	labels := make(map[string]*keyStat)
	paths := make(map[string]*keyStat)
	for i := range misses {
		for _, label := range misses[i].Labels {
			tally(labels, label, misses[i].Stratum)
		}
		for _, path := range misses[i].Paths {
			tally(paths, path, misses[i].Stratum)
		}
	}
	proposals := make([]RouteProposal, 0)
	proposals = appendOverThreshold(proposals, labels, "label", threshold)
	proposals = appendOverThreshold(proposals, paths, "path", threshold)
	sort.Slice(proposals, func(i, j int) bool {
		switch {
		case proposals[i].Count != proposals[j].Count:
			return proposals[i].Count > proposals[j].Count
		case proposals[i].Kind != proposals[j].Kind:
			return proposals[i].Kind < proposals[j].Kind
		default:
			return proposals[i].Key < proposals[j].Key
		}
	})
	return proposals
}

// tally records one miss against a candidate key: its count and its distinct
// non-empty time strata.
func tally(stats map[string]*keyStat, key, stratum string) {
	stat := stats[key]
	if stat == nil {
		stat = &keyStat{strata: make(map[string]bool)}
		stats[key] = stat
	}
	stat.count++
	if stratum != "" {
		stat.strata[stratum] = true
	}
}

// appendOverThreshold adds a proposal for each key that reaches the count threshold
// across at least minStrata distinct strata.
func appendOverThreshold(
	proposals []RouteProposal,
	stats map[string]*keyStat,
	kind string,
	threshold int,
) []RouteProposal {
	for key, stat := range stats {
		if stat.count >= threshold && len(stat.strata) >= minStrata {
			proposals = append(proposals, RouteProposal{
				Key: key, Kind: kind, Count: stat.count, Strata: len(stat.strata),
			})
		}
	}
	return proposals
}

// splitLines splits a JSONL byte slice into its non-empty lines.
func splitLines(data []byte) [][]byte {
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
