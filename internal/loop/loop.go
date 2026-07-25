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
