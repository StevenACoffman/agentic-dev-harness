package critic

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
)

// appendJSONL marshals entry and appends it as one line to the append-only log at
// path, creating the parent directory. It is the shared write path of the critic's
// coverage and precision logs; the caller decides when an entry is empty enough to
// skip. op names the caller for the error's logical trace.
func appendJSONL(op, path string, entry any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return &adh.Error{Op: op, Err: err}
	}
	line, err := json.Marshal(entry)
	if err != nil {
		return &adh.Error{Op: op, Err: err}
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return &adh.Error{Op: op, Err: err}
	}
	defer func() { _ = file.Close() }()
	if _, err := file.Write(append(line, '\n')); err != nil {
		return &adh.Error{Op: op, Err: err}
	}
	return nil
}

// loadJSONL reads the JSONL log at path into a slice of T. An absent file is no
// entries, not an error; a corrupt line is a hard error — the log's integrity is its
// value. The shared read path of the coverage and precision logs.
func loadJSONL[T any](op, path string) ([]T, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return []T{}, nil
	}
	if err != nil {
		return nil, &adh.Error{Op: op, Err: err}
	}
	entries := make([]T, 0)
	for _, line := range splitJSONLines(data) {
		var entry T
		if err := json.Unmarshal(line, &entry); err != nil {
			return nil, &adh.Error{Op: op, Err: err}
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

// splitJSONLines splits a JSONL byte slice into its non-empty lines.
func splitJSONLines(data []byte) [][]byte {
	lines := make([][]byte, 0)
	start := 0
	for i, b := range data {
		if b == '\n' {
			if i > start {
				lines = append(lines, data[start:i])
			}
			start = i + 1
		}
	}
	if start < len(data) {
		lines = append(lines, data[start:])
	}
	return lines
}
