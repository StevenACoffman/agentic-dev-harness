package loop_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/loop"
)

func TestValidate(t *testing.T) {
	valid := loop.Loop{
		ID: "dep-scan", Goal: "no vulnerable dep ships", Sensor: "adh tool run dep-scan",
		RetireWhen: "policy moves into a repo check",
	}
	tests := []struct {
		name     string
		reg      loop.Registry
		wantCode string
	}{
		{name: "valid", reg: loop.Registry{Loops: []loop.Loop{valid}}, wantCode: ""},
		{
			name:     "no retirement condition",
			reg:      loop.Registry{Loops: []loop.Loop{{ID: "x", Goal: "g", Sensor: "s"}}},
			wantCode: adh.EINVALID,
		},
		{
			name:     "no sensor",
			reg:      loop.Registry{Loops: []loop.Loop{{ID: "x", Goal: "g", RetireWhen: "w"}}},
			wantCode: adh.EINVALID,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := adh.ErrorCode(tt.reg.Validate()); got != tt.wantCode {
				t.Errorf("Validate code = %q, want %q", got, tt.wantCode)
			}
		})
	}
}

func TestFind(t *testing.T) {
	reg := loop.Registry{
		Loops: []loop.Loop{{ID: "dep-scan", Goal: "g", Sensor: "s", RetireWhen: "w"}},
	}
	if got, ok := reg.Find("dep-scan"); !ok || got.ID != "dep-scan" {
		t.Errorf("Find = (%v, %v), want dep-scan", got.ID, ok)
	}
	if _, ok := reg.Find("missing"); ok {
		t.Errorf("Find found a nonexistent loop")
	}
}

// TestStarterRegistryValid: the seeded standing loops form a valid registry, so
// `adh init` writes a file `loop list`/`run` accept and each has a retire condition.
func TestStarterRegistryValid(t *testing.T) {
	reg := loop.StarterRegistry()
	if err := reg.Validate(); err != nil {
		t.Fatalf("StarterRegistry().Validate() = %v, want nil", err)
	}
	for _, id := range []string{"context-drift", "harness-integrity", "lesson-backlog"} {
		l, ok := reg.Find(id)
		if !ok {
			t.Errorf("StarterRegistry() missing loop %q", id)
			continue
		}
		if l.OnFinding != "open arc" {
			t.Errorf("loop %q on_finding = %q, want \"open arc\"", id, l.OnFinding)
		}
	}
}

// TestMarshalRoundTrips: Marshal's output decodes back to the same registry.
func TestMarshalRoundTrips(t *testing.T) {
	want := loop.StarterRegistry()
	data, err := loop.Marshal(want)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	path := filepath.Join(t.TempDir(), "loops.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := loop.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got.Loops) != len(want.Loops) {
		t.Fatalf("round-trip loop count = %d, want %d", len(got.Loops), len(want.Loops))
	}
}
