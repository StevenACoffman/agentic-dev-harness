// Package failures owns the append-only failure registry (SPEC §4.1): a JSON
// array of failure notes under a repository path, distilled into lessons (§11).
// From §19.2 it also backs the lesson-candidate list — findings the critic raised
// that no artifact confirmed, kept to detect a recurring class. Load and Append
// are the thin I/O shell; a missing file reads as empty. Writes are atomic.
package failures

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/skillet/atomicfile"
)

const (
	// RegistryFile is the confirmed failure registry (§4.1): a deterministic check
	// that a critic finding named actually failed.
	RegistryFile = ".adh/failures.json"
	// CandidatesFile is the §11 lesson-candidate list (§19.2): a critic finding no
	// artifact confirmed, recorded so a class can be promoted once it recurs.
	CandidatesFile = ".adh/lesson-candidates.json"
	// RecordsFile is the stamped failure-record log (§19.2): one record per disposed
	// finding carrying its stratum, the arc's routing scope, and its root cause — the
	// evidence the §11 temporal gate, scope-tagging, and root-cause triage read.
	RecordsFile = ".adh/failure-records.json"
)

// Load reads the note list at path. A missing file is empty, not an error; a
// malformed one is EINVALID.
func Load(path string) ([]string, error) {
	const op = "failures.Load"
	data, err := os.ReadFile(path)
	switch {
	case os.IsNotExist(err):
		return nil, nil
	case err != nil:
		return nil, &adh.Error{Op: op, Err: err}
	}
	var notes []string
	if err := json.Unmarshal(data, &notes); err != nil {
		return nil, &adh.Error{Op: op, Err: err}
	}
	return notes, nil
}

// Append adds notes to the registry at path, creating it and its directory if
// absent, and rewrites the file atomically. Appending nothing is a no-op, so it
// never creates an empty registry.
func Append(path string, notes ...string) error {
	const op = "failures.Append"
	if len(notes) == 0 {
		return nil
	}
	existing, err := Load(path)
	if err != nil {
		return err
	}
	existing = append(existing, notes...)
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
