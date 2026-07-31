// Package loop models maintenance loops (SPEC-ADDITIONS §15): a named invariant
// kept true by a sensor and an authorized action, with an explicit retirement
// condition. The registry and its validation are pure; running a sensor is a
// command-level seam.
package loop

import (
	"encoding/json"
	"os"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
)

// DefaultRegistryFile is the conventional repo path the loop registry lives under
// (SPEC-ADDITIONS §15). It is the single owner of that path across the CLI.
const DefaultRegistryFile = ".adh/loops.json"

// Loop is one maintenance loop: the invariant it keeps, how it senses a
// departure, the authorized action on a finding, when it retires, and its owner.
type Loop struct {
	ID         string `json:"id"`
	Goal       string `json:"goal"`
	Sensor     string `json:"sensor"`
	OnFinding  string `json:"on_finding,omitempty"`
	RetireWhen string `json:"retire_when"`
	Owner      string `json:"owner,omitempty"`
}

// Registry is the set of registered maintenance loops.
type Registry struct {
	Loops []Loop `json:"loops"`
}

// Validate checks that every loop names an invariant, a sensor, and a
// retirement condition, and that IDs are unique. A loop without a retirement
// condition can never be garbage-collected, so it is rejected (§15).
func (r Registry) Validate() error {
	seen := make(map[string]bool, len(r.Loops))
	for i := range r.Loops {
		loop := &r.Loops[i]
		switch {
		case loop.ID == "":
			return &adh.Error{Code: adh.EINVALID, Message: "loop with empty id"}
		case loop.Goal == "":
			return &adh.Error{Code: adh.EINVALID, Message: "loop " + loop.ID + " has no goal"}
		case loop.Sensor == "":
			return &adh.Error{Code: adh.EINVALID, Message: "loop " + loop.ID + " has no sensor"}
		case loop.RetireWhen == "":
			return &adh.Error{
				Code:    adh.EINVALID,
				Message: "loop " + loop.ID + " has no retirement condition",
			}
		case seen[loop.ID]:
			return &adh.Error{Code: adh.EINVALID, Message: "duplicate loop id: " + loop.ID}
		}
		seen[loop.ID] = true
	}
	return nil
}

// StarterRegistry is the set of standing maintenance loops that make accretion
// automatic instead of prompted (SPEC-ADDITIONS §15): each names an invariant the
// harness keeps true, a deterministic sensor (`adh` itself), and "open arc" as the
// authorized action, so a sensed departure becomes work an agent drives. `adh init`
// seeds it and then the operator tailors it. The sensors compose the other levers —
// context-integrity (Loop F, `context verify`), the tool registry (`tool doctor`),
// and the lesson backlog — so a correction is inherited by the next arc, not lost.
func StarterRegistry() Registry {
	return Registry{Loops: []Loop{
		{
			ID:         "context-drift",
			Goal:       "routed context still matches its canonical source",
			Sensor:     "adh context verify",
			OnFinding:  "open arc",
			RetireWhen: "no context units declare an integrity check",
			Owner:      "context",
		},
		{
			ID:         "harness-integrity",
			Goal:       "the tool registry is valid and every declared tool resolves",
			Sensor:     "adh tool doctor",
			OnFinding:  "open arc",
			RetireWhen: "the harness is retired",
			Owner:      "tools",
		},
		{
			ID:         "lesson-backlog",
			Goal:       "confirmed lessons are promoted, not left as candidates",
			Sensor:     "test ! -s .adh/lesson-candidates.json",
			OnFinding:  "open arc",
			RetireWhen: "lessons are promoted at the point of confirmation",
			Owner:      "lesson",
		},
	}}
}

// Marshal encodes the registry as indented JSON with a trailing newline — the
// on-disk form Load reads back, so the write and read sides agree in one place.
func Marshal(r Registry) ([]byte, error) {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return nil, &adh.Error{Op: "loop.Marshal", Err: err}
	}
	return append(data, '\n'), nil
}

// Find returns the loop with the given ID.
func (r Registry) Find(id string) (Loop, bool) {
	for i := range r.Loops {
		if r.Loops[i].ID == id {
			return r.Loops[i], true
		}
	}
	return Loop{}, false
}

// Load reads a loop registry from a JSON file. An absent file is an empty
// registry, not an error.
func Load(path string) (Registry, error) {
	const op = "loop.Load"
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return Registry{}, nil
	}
	if err != nil {
		return Registry{}, &adh.Error{Op: op, Err: err}
	}
	var reg Registry
	if err := json.Unmarshal(data, &reg); err != nil {
		return Registry{}, &adh.Error{Op: op, Err: err}
	}
	return reg, nil
}
