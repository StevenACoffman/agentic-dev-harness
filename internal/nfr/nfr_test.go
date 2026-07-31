package nfr_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/nfr"
)

func TestSpecValid(t *testing.T) {
	base := nfr.Spec{
		ID: "latency", Tag: "Performance.Latency", Scale: "ms p95",
		Meter: "bench-latency", Direction: nfr.Lower, Fail: 300, Goal: 200, Stretch: 100,
	}
	tests := []struct {
		name    string
		mutate  func(s *nfr.Spec)
		wantErr bool
	}{
		{"valid lower", func(*nfr.Spec) {}, false},
		{"valid higher", func(s *nfr.Spec) {
			s.Tag, s.Direction, s.Fail, s.Goal, s.Stretch = "Reliability.Availability", nfr.Higher, 99, 99.9, 99.99
		}, false},
		{"unknown category", func(s *nfr.Spec) { s.Tag = "Vibes.Speed" }, true},
		{"no scale", func(s *nfr.Spec) { s.Scale = "" }, true},
		{"no meter", func(s *nfr.Spec) { s.Meter = "" }, true},
		{"bad direction", func(s *nfr.Spec) { s.Direction = "sideways" }, true},
		{"misordered lower", func(s *nfr.Spec) { s.Goal = 400 }, true}, // goal worse than fail
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := base
			tt.mutate(&s)
			err := s.Valid()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Valid() = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && adh.ErrorCode(err) != adh.EINVALID {
				t.Errorf("Valid() code = %q, want %q", adh.ErrorCode(err), adh.EINVALID)
			}
		})
	}
}

func TestSpecMeets(t *testing.T) {
	lower := nfr.Spec{Direction: nfr.Lower, Fail: 300}
	if !lower.Meets(250) || lower.Meets(350) {
		t.Errorf("lower Meets: 250 should pass, 350 should fail")
	}
	higher := nfr.Spec{Direction: nfr.Higher, Fail: 99}
	if !higher.Meets(99.9) || higher.Meets(98) {
		t.Errorf("higher Meets: 99.9 should pass, 98 should fail")
	}
}

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	spec := `{"id":"latency","tag":"Performance.Latency","scale":"ms","meter":"bench","direction":"lower","fail":300,"goal":200}`
	if err := os.WriteFile(filepath.Join(dir, "latency.json"), []byte(spec), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	specs, err := nfr.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(specs) != 1 || specs[0].ID != "latency" {
		t.Fatalf("Load = %+v, want one latency spec", specs)
	}
	// An absent dir is no specs, not an error.
	none, err := nfr.Load(filepath.Join(dir, "missing"))
	if err != nil || len(none) != 0 {
		t.Errorf("Load(absent) = (%v, %v), want ([], nil)", none, err)
	}
}
