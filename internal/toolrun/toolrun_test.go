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

func TestAppendOutcome(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tool-runs.json")
	if err := toolrun.AppendOutcome(path, "chk", "2026-06", true, true, 42, ""); err != nil {
		t.Fatalf("AppendOutcome: %v", err)
	}
	records, err := toolrun.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("records = %v, want 1", records)
	}
	r := records[0]
	if r.Tool != "chk" || !r.Ran || !r.Failed || r.DurationMS != 42 || r.Stratum != "2026-06" {
		t.Errorf("record = %+v, want chk ran+failed 42ms in 2026-06", r)
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

// TestTheReasonReachesTheLog. Ran:false alone is a count, not a signal: a log that
// records how often the deterministic path missed and never why cannot say whether the
// misses are a broken environment, a critic naming artifacts that do not exist, or
// findings naming nothing — and those call for entirely different responses.
func TestTheReasonReachesTheLog(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), toolrun.RunFile)
	if err := toolrun.AppendOutcome(
		path, "chk", "2026-06", false, false, 3, "tool-failed",
	); err != nil {
		t.Fatalf("append: %v", err)
	}
	got, err := toolrun.Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d records, want 1", len(got))
	}
	if got[0].Unrunnable != "tool-failed" {
		t.Errorf("Unrunnable = %q, want the reason the check did not run", got[0].Unrunnable)
	}
}

// TestARunThatSucceededCarriesNoReason, so an empty field means "it ran" rather than
// "nobody recorded why it did not".
func TestARunThatSucceededCarriesNoReason(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), toolrun.RunFile)
	if err := toolrun.AppendOutcome(path, "chk", "2026-06", true, false, 3, ""); err != nil {
		t.Fatalf("append: %v", err)
	}
	got, _ := toolrun.Load(path)
	if len(got) == 1 && got[0].Unrunnable != "" {
		t.Errorf("a successful run carries a reason: %q", got[0].Unrunnable)
	}
}
