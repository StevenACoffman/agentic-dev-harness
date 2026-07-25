// Package worker models the adoption epoch and requalification gate (SPEC §14 /
// fixed-worker): a model or agent change opens a new epoch that must be
// requalified before normal runs resume. The comparison is pure.
package worker

import (
	"encoding/json"
	"os"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
)

// Epoch records the per-role model binding for one adoption epoch.
type Epoch struct {
	ID     string            `json:"id"`
	Models map[string]string `json:"models"` // role -> model identifier
}

// RequalRequired reports whether current opens a new epoch relative to e — that
// is, whether any per-role model binding changed. A changed worker invalidates
// the held-out baseline, so requalification must precede the next run (§14).
func (e Epoch) RequalRequired(current Epoch) bool {
	if len(e.Models) != len(current.Models) {
		return true
	}
	for role, model := range e.Models {
		if current.Models[role] != model {
			return true
		}
	}
	return false
}

// Load reads the recorded epoch from a JSON file. An absent file yields the zero
// Epoch and no error (a fresh, never-requalified workspace).
func Load(path string) (Epoch, error) {
	const op = "worker.Load"
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return Epoch{}, nil
	}
	if err != nil {
		return Epoch{}, &adh.Error{Op: op, Err: err}
	}
	var epoch Epoch
	if err := json.Unmarshal(data, &epoch); err != nil {
		return Epoch{}, &adh.Error{Op: op, Err: err}
	}
	return epoch, nil
}

// Save writes an epoch to a JSON file.
func Save(path string, epoch Epoch) error {
	const op = "worker.Save"
	data, err := json.MarshalIndent(epoch, "", "  ")
	if err != nil {
		return &adh.Error{Op: op, Err: err}
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o600); err != nil {
		return &adh.Error{Op: op, Err: err}
	}
	return nil
}
