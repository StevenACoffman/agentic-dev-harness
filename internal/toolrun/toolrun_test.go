package toolrun_test

import (
	"path/filepath"
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/internal/toolrun"
)

func TestAppendThenLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runs", "tool-runs.json")
	if err := toolrun.Append(path,
		toolrun.Record{Tool: "skillsaw-eval", Stratum: "2026-06", Ran: true, Failed: true},
	); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if err := toolrun.Append(path,
		toolrun.Record{Tool: "skillsaw-eval", Stratum: "2026-07", Ran: true, Failed: false},
	); err != nil {
		t.Fatalf("Append: %v", err)
	}
	records, err := toolrun.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("records = %v, want 2 accumulated", records)
	}
	if records[0].Tool != "skillsaw-eval" || !records[0].Failed {
		t.Errorf("record[0] = %+v, want the failing skillsaw run", records[0])
	}
}

func TestLoadMissingIsEmpty(t *testing.T) {
	records, err := toolrun.Load(filepath.Join(t.TempDir(), "absent.json"))
	if err != nil {
		t.Fatalf("Load of a missing file: %v", err)
	}
	if len(records) != 0 {
		t.Errorf("records = %v, want empty", records)
	}
}

func TestAppendNothingIsNoop(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tool-runs.json")
	if err := toolrun.Append(path); err != nil {
		t.Fatalf("Append of nothing: %v", err)
	}
	if _, err := toolrun.Load(path); err != nil {
		t.Fatalf("Load: %v", err)
	}
}
