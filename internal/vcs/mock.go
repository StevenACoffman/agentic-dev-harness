package vcs

import "time"

// Mock is an in-memory Repo for tests and consumers that need no real
// repository. Branch and Changed are scripted; Commit clears Changed, marks the
// tree clean, and returns a deterministic hash derived from a monotonic counter.
type Mock struct {
	Branch  string
	Changed []string
	Patch   string // scripted Diff output
	commits int
}

// Diff returns the scripted patch. It ignores paths — the mock has no tree to
// diff — so a test sets Patch to whatever the consumer should see.
func (m *Mock) Diff(_ []string) (string, error) {
	return m.Patch, nil
}

// Revert drops the given paths from the scripted changed set, mirroring a revert
// of their working-tree changes. Paths not in the set are ignored.
func (m *Mock) Revert(paths []string) error {
	drop := make(map[string]bool, len(paths))
	for _, path := range paths {
		drop[path] = true
	}
	kept := m.Changed[:0]
	for _, path := range m.Changed {
		if !drop[path] {
			kept = append(kept, path)
		}
	}
	m.Changed = kept
	return nil
}

// CurrentBranch returns the scripted branch, defaulting to "main".
func (m *Mock) CurrentBranch() (string, error) {
	if m.Branch == "" {
		return "main", nil
	}
	return m.Branch, nil
}

// Status reports the scripted branch and changed set; clean when Changed empty.
func (m *Mock) Status() (Status, error) {
	branch, _ := m.CurrentBranch()
	return Status{Branch: branch, Clean: len(m.Changed) == 0, Changed: m.Changed}, nil
}

// CreateBranch switches the scripted branch.
func (m *Mock) CreateBranch(name string) error {
	m.Branch = name
	return nil
}

// Commit clears the changed set and returns a deterministic fake hash. It
// ignores who and when so callers stay clock-free and tests stay stable.
func (m *Mock) Commit(_ string, _ Signature, _ time.Time) (string, error) {
	m.commits++
	m.Changed = nil
	return "mockhash" + string(rune('0'+m.commits%10)), nil
}
