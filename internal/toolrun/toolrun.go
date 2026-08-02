// Package toolrun owns the append-only tool-run outcome log (SPEC-ADDITIONS §16/§18):
// one record per `adh tool run`, stamped so a §13 tool's declared KPIs can be measured
// against how it actually behaves over time. Load and Append are the thin I/O shell
// around a JSON array; a missing file reads as empty. Writes are atomic.
package toolrun

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/atomicfile"
)

// RunFile is the tool-run outcome log path (§16/§18): a JSON array of records `adh
// tool run` appends to, read by `adh kpi` to observe per-tool KPIs.
const RunFile = ".adh/tool-runs.json"

// Record is one tool invocation's outcome: the tool id, the year-month Stratum it ran
// in (the §18.2 replication axis), whether it Ran at all (a startable command), and
// whether it Failed (ran and exited non-zero). A run that could not start is Ran=false.
type Record struct {
	Tool    string `json:"tool"`
	Stratum string `json:"stratum,omitempty"`
	Ran     bool   `json:"ran"`
	Failed  bool   `json:"failed"`
}

// Load reads the tool-run log at path. A missing file is empty, not an error; a
// malformed one is EINVALID.
func Load(path string) ([]Record, error) {
	const op = "toolrun.Load"
	data, err := os.ReadFile(path)
	switch {
	case os.IsNotExist(err):
		return nil, nil
	case err != nil:
		return nil, &adh.Error{Op: op, Err: err}
	}
	var records []Record
	if err := json.Unmarshal(data, &records); err != nil {
		return nil, &adh.Error{Op: op, Err: err}
	}
	return records, nil
}

// Append adds records to the log at path, creating it and its directory if absent, and
// rewrites the file atomically. Appending nothing is a no-op, so it never creates an
// empty log.
func Append(path string, records ...Record) error {
	const op = "toolrun.Append"
	if len(records) == 0 {
		return nil
	}
	existing, err := Load(path)
	if err != nil {
		return err
	}
	existing = append(existing, records...)
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
