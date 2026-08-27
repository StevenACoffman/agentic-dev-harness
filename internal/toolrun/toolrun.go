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
	"github.com/StevenACoffman/skillet/atomicfile"
)

// RunFile is the tool-run outcome log path (§16/§18): a JSON array of records `adh
// tool run` appends to, read by `adh kpi` to observe per-tool KPIs.
const RunFile = ".adh/tool-runs.json"

// Record is one tool invocation's outcome: the tool id, the year-month Stratum it ran
// in (the §18.2 replication axis), whether it Ran at all (a startable command), whether
// it Failed (ran and exited non-zero), and how long it took (DurationMS, 0 when it did
// not run). A run that could not start is Ran=false.
type Record struct {
	Tool       string `json:"tool"`
	Stratum    string `json:"stratum,omitempty"`
	Ran        bool   `json:"ran"`
	Failed     bool   `json:"failed"`
	DurationMS int    `json:"duration_ms,omitempty"`

	// Unrunnable is why the check did not run, meaningful only when Ran is false.
	//
	// **Ran:false alone is a count, not a signal.** A log that records how often the
	// deterministic path missed and never why cannot say whether the misses are a
	// broken environment, a critic naming artifacts that do not exist, or findings
	// naming nothing at all — and those three call for entirely different responses.
	// The reason is computed at adjudication (adh.Unrunnable) and used to be discarded
	// once the verdict was reported.
	//
	// The load-bearing analogue is a surveyed router's `KeywordsNotFound`: what makes
	// its miss log actionable rather than a tally is that it records *which*
	// deterministic layer missed and on what.
	Unrunnable string `json:"unrunnable,omitempty"`
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

// AppendOutcome records one tool run to the log at path: it builds the stratum-stamped
// record from the run's outcome and appends it. It is the single owner of the Record
// field mapping, so every tool-run site — `adh tool run`, `context verify`, adjudication
// — turns its result into telemetry the same way. Best-effort at the call site: the
// returned error is for surfacing, never for failing the run.
func AppendOutcome(
	path, tool, stratum string, ran, failed bool, durationMS int, unrunnable string,
) error {
	return Append(path, Record{
		Tool:       tool,
		Stratum:    stratum,
		Ran:        ran,
		Failed:     failed,
		DurationMS: durationMS,
		Unrunnable: unrunnable,
	})
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
