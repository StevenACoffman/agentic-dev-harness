// Package worker models the adoption epoch and requalification gate (SPEC §14 /
// fixed-worker): a model or agent change opens a new epoch that must be
// requalified before normal runs resume. The comparison is pure.
package worker

import (
	"encoding/json"
	"os"
	"sort"
	"strings"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/skillet/identity"
)

// DefaultStateFile is the conventional path the recorded epoch lives under; it is
// the single owner of that location across the CLI.
const DefaultStateFile = ".adh/worker.json"

// Epoch records the per-role model binding for one adoption epoch.
type Epoch struct {
	ID     string            `json:"id"`
	Models map[string]string `json:"models"` // role -> model identifier
}

// EpochFor builds the epoch for a per-role model binding: its ID is the content
// hash of the canonical binding, so an identical binding hashes to the same epoch
// and any model change yields a new one (§14).
func EpochFor(models map[string]string) Epoch {
	return Epoch{ID: identity.Hash(canonical(models)), Models: models}
}

// canonical serializes the binding in a stable role order for hashing.
func canonical(models map[string]string) string {
	roles := make([]string, 0, len(models))
	for role := range models {
		roles = append(roles, role)
	}
	sort.Strings(roles)
	var b strings.Builder
	for _, role := range roles {
		b.WriteString(role + "=" + models[role] + ";")
	}
	return b.String()
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

// RequalifyNeeded reports whether a run must refuse pending requalification (§14):
// a baseline epoch is recorded at stateFile and the current worker differs from it.
// A never-requalified workspace (no recorded epoch) is not gated — like the §19.1
// routing gap, the refusal presupposes a recorded baseline to have departed from,
// so a fresh workspace runs ungated until its first `worker requalify`.
func RequalifyNeeded(stateFile string, current Epoch) (bool, error) {
	recorded, err := Load(stateFile)
	if err != nil {
		return false, err
	}
	return recorded.ID != "" && recorded.RequalRequired(current), nil
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
