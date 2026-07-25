// Package state persists arcs to the .adh workspace as JSON and provides the
// lifecycle operations the arc commands drive. The pure state-machine logic
// lives in internal/adh (NextStage, CanClose, ProofKind); this package is the
// thin I/O shell around it.
package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
)

// DefaultArcsDir is the arc workspace path relative to the repo root.
const DefaultArcsDir = ".adh/arcs"

// Store reads and writes arcs under a directory (typically .adh/arcs). The
// directory is created on first write.
type Store struct{ dir string }

// NewStore returns a Store rooted at dir.
func NewStore(dir string) *Store { return &Store{dir: dir} }

// Default returns a Store rooted at the default workspace directory.
func Default() *Store { return NewStore(DefaultArcsDir) }

// Create writes a new arc, returning ECONFLICT if one with the same ID exists.
func (s *Store) Create(arc *adh.Arc) error {
	const op = "state.Store.Create"
	if _, err := os.Stat(s.path(arc.ID)); err == nil {
		return &adh.Error{Code: adh.ECONFLICT, Message: "arc already exists: " + arc.ID}
	}
	if err := os.MkdirAll(s.dir, 0o750); err != nil {
		return &adh.Error{Op: op, Err: err}
	}
	return s.write(op, arc)
}

// Save overwrites an arc, creating the workspace directory if needed.
func (s *Store) Save(arc *adh.Arc) error {
	const op = "state.Store.Save"
	if err := os.MkdirAll(s.dir, 0o750); err != nil {
		return &adh.Error{Op: op, Err: err}
	}
	return s.write(op, arc)
}

// Get returns an arc by ID, or ENOTFOUND if it does not exist.
func (s *Store) Get(id string) (adh.Arc, error) {
	const op = "state.Store.Get"
	data, err := os.ReadFile(s.path(id))
	switch {
	case os.IsNotExist(err):
		return adh.Arc{}, &adh.Error{Code: adh.ENOTFOUND, Message: "no such arc: " + id}
	case err != nil:
		return adh.Arc{}, &adh.Error{Op: op, Err: err}
	}
	var arc adh.Arc
	if err := json.Unmarshal(data, &arc); err != nil {
		return adh.Arc{}, &adh.Error{Op: op, Err: err}
	}
	return arc, nil
}

// List returns all arcs sorted by ID. An empty or absent workspace is not an
// error.
func (s *Store) List() ([]adh.Arc, error) {
	const op = "state.Store.List"
	entries, err := os.ReadDir(s.dir)
	switch {
	case os.IsNotExist(err):
		return []adh.Arc{}, nil
	case err != nil:
		return nil, &adh.Error{Op: op, Err: err}
	}
	arcs := make([]adh.Arc, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		id := entry.Name()[:len(entry.Name())-len(".json")]
		arc, getErr := s.Get(id)
		if getErr != nil {
			return nil, getErr
		}
		arcs = append(arcs, arc)
	}
	sort.Slice(arcs, func(i, j int) bool { return arcs[i].ID < arcs[j].ID })
	return arcs, nil
}

// NextID returns the next sequential arc ID ("arc-0001"), derived from the
// current arc count so no clock or randomness is needed.
func (s *Store) NextID() (string, error) {
	arcs, err := s.List()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("arc-%04d", len(arcs)+1), nil
}

func (s *Store) path(id string) string { return filepath.Join(s.dir, id+".json") }

func (s *Store) write(op string, arc *adh.Arc) error {
	data, err := json.MarshalIndent(arc, "", "  ")
	if err != nil {
		return &adh.Error{Op: op, Err: err}
	}
	// Write atomically: a temp file in the same directory, then rename over the
	// target. A crash or a concurrent writer mid-write leaves the temp file, never
	// a truncated arc. (renameio + flock would add fsync durability and
	// cross-process locking; see TODO.md.)
	tmp, err := os.CreateTemp(s.dir, "."+arc.ID+"-*.tmp")
	if err != nil {
		return &adh.Error{Op: op, Err: err}
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return &adh.Error{Op: op, Err: err}
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return &adh.Error{Op: op, Err: err}
	}
	if err := os.Rename(tmpName, s.path(arc.ID)); err != nil {
		_ = os.Remove(tmpName)
		return &adh.Error{Op: op, Err: err}
	}
	return nil
}
