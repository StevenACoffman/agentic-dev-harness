package failures_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/internal/failures"
)

func TestAppendThenLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "reg", "failures.json")
	if err := failures.Append(path, "critic: oracle divergence", "critic: bad proof"); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if err := failures.Append(path, "eval: device unhealthy"); err != nil {
		t.Fatalf("Append: %v", err)
	}
	notes, err := failures.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(notes) != 3 {
		t.Fatalf("notes = %v, want 3 accumulated", notes)
	}
	if notes[2] != "eval: device unhealthy" {
		t.Errorf("last note = %q, want the appended one", notes[2])
	}
}

func TestLoadMissingIsEmpty(t *testing.T) {
	notes, err := failures.Load(filepath.Join(t.TempDir(), "absent.json"))
	if err != nil {
		t.Fatalf("Load missing: %v", err)
	}
	if len(notes) != 0 {
		t.Errorf("missing registry = %v, want empty", notes)
	}
}

func TestAppendNothingIsNoop(t *testing.T) {
	path := filepath.Join(t.TempDir(), "failures.json")
	if err := failures.Append(path); err != nil {
		t.Fatalf("Append(): %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("empty append created a file (err=%v); want none", err)
	}
}
