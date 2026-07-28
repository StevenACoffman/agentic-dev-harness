package metrics

import (
	"encoding/json"
	"os"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
)

// LedgerFile is the effectiveness ledger path (§16): a JSON array of Records a
// closed arc appends its cost to.
const LedgerFile = ".adh/metrics.json"

// Load reads the effectiveness ledger at path. A missing file is empty (not an
// error); a malformed one is EINVALID. It is the thin I/O shell around the pure
// Summarize core.
func Load(path string) ([]Record, error) {
	const op = "metrics.Load"
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
