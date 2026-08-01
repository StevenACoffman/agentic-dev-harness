package failures_test

import (
	"path/filepath"
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/internal/failures"
)

func TestAppendThenLoadRecords(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rec", "failure-records.json")
	if err := failures.AppendRecords(path,
		failures.Record{Class: "oracle", Stratum: "2026-06", Labels: []string{"ui"}},
	); err != nil {
		t.Fatalf("AppendRecords: %v", err)
	}
	if err := failures.AppendRecords(path,
		failures.Record{Class: "oracle", Stratum: "2026-07", Paths: []string{"board.go"}},
	); err != nil {
		t.Fatalf("AppendRecords: %v", err)
	}
	recs, err := failures.LoadRecords(path)
	if err != nil {
		t.Fatalf("LoadRecords: %v", err)
	}
	if len(recs) != 2 {
		t.Fatalf("records = %v, want 2 accumulated", recs)
	}
}

func TestLoadRecordsMissingIsEmpty(t *testing.T) {
	recs, err := failures.LoadRecords(filepath.Join(t.TempDir(), "absent.json"))
	if err != nil {
		t.Fatalf("LoadRecords of a missing file: %v", err)
	}
	if len(recs) != 0 {
		t.Errorf("records = %v, want empty", recs)
	}
}

func TestAppendNoRecordsIsNoop(t *testing.T) {
	path := filepath.Join(t.TempDir(), "failure-records.json")
	if err := failures.AppendRecords(path); err != nil {
		t.Fatalf("AppendRecords of nothing: %v", err)
	}
	if _, err := failures.LoadRecords(path); err != nil {
		t.Fatalf("LoadRecords: %v", err)
	}
}

func TestStrataCount(t *testing.T) {
	recs := []failures.Record{
		{Class: "oracle", Stratum: "2026-06"},
		{Class: "oracle", Stratum: "2026-07"},
		{Class: "oracle", Stratum: "2026-07"}, // same stratum, not double-counted
		{Class: "device", Stratum: "2026-07"},
		{Class: "device"}, // no stratum contributes nothing
	}
	got := failures.StrataCount(recs)
	if got["oracle"] != 2 {
		t.Errorf("oracle strata = %d, want 2 distinct", got["oracle"])
	}
	if got["device"] != 1 {
		t.Errorf("device strata = %d, want 1", got["device"])
	}
}

func TestScopeFor(t *testing.T) {
	recs := []failures.Record{
		{Class: "oracle", Labels: []string{"ui", "board"}, Paths: []string{"a.go"}},
		{Class: "oracle", Labels: []string{"ui"}, Paths: []string{"b.go"}},
		{Class: "device", Labels: []string{"hw"}},
	}
	labels, paths := failures.ScopeFor(recs, "oracle")
	if len(labels) != 2 || labels[0] != "board" || labels[1] != "ui" {
		t.Errorf("labels = %v, want sorted distinct [board ui]", labels)
	}
	if len(paths) != 2 || paths[0] != "a.go" || paths[1] != "b.go" {
		t.Errorf("paths = %v, want sorted distinct [a.go b.go]", paths)
	}
}

func TestRootCauseCountsAndClassify(t *testing.T) {
	if got := failures.ClassifyRootCause(true); got != failures.RootGroundedMiss {
		t.Errorf("ClassifyRootCause(grounded) = %q, want %q", got, failures.RootGroundedMiss)
	}
	if got := failures.ClassifyRootCause(false); got != failures.RootUngrounded {
		t.Errorf("ClassifyRootCause(ungrounded) = %q, want %q", got, failures.RootUngrounded)
	}
	recs := []failures.Record{
		{Class: "oracle", RootCause: failures.RootUngrounded},
		{Class: "oracle", RootCause: failures.RootUngrounded},
		{Class: "oracle", RootCause: failures.RootGroundedMiss},
		{Class: "oracle"}, // no cause ignored
		{Class: "device", RootCause: failures.RootUngrounded},
	}
	got := failures.RootCauseCounts(recs, "oracle")
	if got[failures.RootUngrounded] != 2 || got[failures.RootGroundedMiss] != 1 {
		t.Errorf("root-cause counts = %v, want ungrounded 2 + grounded-miss 1", got)
	}
}
