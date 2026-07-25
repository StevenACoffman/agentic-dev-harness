// Package evidence is the append-only audit log for the self-optimization loop
// (SPEC-ADDITIONS §18.6). It copies skillsaw/store's idiom — append-only,
// validate-on-write, a malformed line is a hard error (corruption is surfaced,
// never swallowed) — but records JSON lines with adh columns binding a gate
// decision to its arc, stage, and scores. Read and Append are pure over io; the
// file open/append lives in the command shell.
package evidence

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
)

// bufCap bounds a single JSONL line (a record with long notes stays well under).
const bufCap = 1024 * 1024

// Status values are the per-cycle outcomes recorded in the log.
const (
	StatusBaseline Status = "baseline"
	StatusKeep     Status = "keep"
	StatusRevert   Status = "revert"
	StatusError    Status = "error"
)

// Status is a record outcome.
type Status string

// Record is one evidence line: a gate decision bound to its arc, stage, and the
// scores that produced it.
type Record struct {
	Timestamp  string  `json:"timestamp"`
	Arc        string  `json:"arc,omitempty"`
	Stage      string  `json:"stage,omitempty"`
	GateAction string  `json:"gate_action,omitempty"`
	OldScore   float64 `json:"old_score"`
	NewScore   float64 `json:"new_score"`
	Status     Status  `json:"status"`
	Note       string  `json:"note,omitempty"`
}

// Read parses records from a JSONL reader. Blank lines are skipped; a malformed
// line is a hard error (EINVALID) rather than a silent skip, so a corrupt log is
// surfaced, never swallowed.
func Read(rd io.Reader) ([]Record, error) {
	scanner := bufio.NewScanner(rd)
	scanner.Buffer(make([]byte, 0, 64*1024), bufCap)
	var records []Record
	for line := 1; scanner.Scan(); line++ {
		text := strings.TrimSpace(scanner.Text())
		if text == "" {
			continue
		}
		var rec Record
		if err := json.Unmarshal([]byte(text), &rec); err != nil {
			return nil, &adh.Error{
				Code:    adh.EINVALID,
				Message: fmt.Sprintf("evidence: line %d is not valid JSON", line),
			}
		}
		records = append(records, rec)
	}
	if err := scanner.Err(); err != nil {
		return nil, &adh.Error{Op: "evidence.Read", Err: err}
	}
	return records, nil
}

// Append writes records as JSON lines to w. A record with an unknown status is
// rejected before anything is written for it (validate on write).
func Append(w io.Writer, records ...Record) error {
	for i := range records {
		rec := &records[i]
		if !rec.Status.valid() {
			return &adh.Error{
				Code:    adh.EINVALID,
				Message: "evidence: invalid status " + string(rec.Status),
			}
		}
		data, err := json.Marshal(rec)
		if err != nil {
			return &adh.Error{Op: "evidence.Append", Err: err}
		}
		if _, err := fmt.Fprintln(w, string(data)); err != nil {
			return &adh.Error{Op: "evidence.Append", Err: err}
		}
	}
	return nil
}

func (s Status) valid() bool {
	switch s {
	case StatusBaseline, StatusKeep, StatusRevert, StatusError:
		return true
	default:
		return false
	}
}
