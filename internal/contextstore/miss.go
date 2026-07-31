package contextstore

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
)

// MissFile is the conventional repo path the routing-miss log lives under. It is
// kept distinct from the evidence trail (§10.3): a miss is a learning signal — an
// arc that declared a footprint the store could not route — not an audit record.
const MissFile = ".adh/context-misses.jsonl"

// Miss is one recorded routing miss: an arc whose labels/paths routed no context
// (the §19.1 routing gap). Accumulated misses are the signal that the store is
// missing a route the arcs keep asking for.
type Miss struct {
	Arc    string   `json:"arc"`
	Labels []string `json:"labels,omitempty"`
	Paths  []string `json:"paths,omitempty"`
}

// RouteProposal is a proposed deterministic route the miss log has earned: a label
// or path the arcs missed on at least a threshold number of times, so authoring a
// unit for it would convert the recurring miss into a route. Kind is "label" or
// "path"; Count is how many recorded misses named Key.
type RouteProposal struct {
	Key   string `json:"key"`
	Kind  string `json:"kind"`
	Count int    `json:"count"`
}

// AppendMiss records one routing miss to the append-only log at path, creating the
// parent directory and file if needed. It is the I/O shell to ProposeRoutes' pure
// core; a caller records misses so the router can later learn from them.
func AppendMiss(path string, miss Miss) error {
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
// path named in at least threshold misses, ranked by recurrence (then kind, then
// key) so the most-asked-for route comes first. It is pure — the caller decides
// what to do with a proposal (author a unit, gated at §11); nothing is auto-routed.
func ProposeRoutes(misses []Miss, threshold int) []RouteProposal {
	labels := make(map[string]int)
	paths := make(map[string]int)
	for i := range misses {
		for _, label := range misses[i].Labels {
			labels[label]++
		}
		for _, path := range misses[i].Paths {
			paths[path]++
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

// appendOverThreshold adds a proposal for each key whose count reaches threshold.
func appendOverThreshold(
	proposals []RouteProposal,
	counts map[string]int,
	kind string,
	threshold int,
) []RouteProposal {
	for key, n := range counts {
		if n >= threshold {
			proposals = append(proposals, RouteProposal{Key: key, Kind: kind, Count: n})
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
