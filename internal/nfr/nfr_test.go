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

// TestRegressed. Meets asks whether a value clears the acceptance bar; this asks
// whether it got worse. A change can pass every bar and still cost something, which is
// the case that used to advance an arc with nothing recorded.
func TestRegressed(t *testing.T) {
	t.Parallel()
	lower := nfr.Spec{Direction: nfr.Lower, Baseline: 100, Fail: 200, Goal: 50}
	higher := nfr.Spec{Direction: nfr.Higher, Baseline: 0.9, Fail: 0.8, Goal: 0.95}
	unbaselined := nfr.Spec{Direction: nfr.Lower, Fail: 200, Goal: 50}

	cases := map[string]struct {
		spec  nfr.Spec
		value float64
		want  bool
	}{
		"lower-is-better, got slower":   {lower, 150, true},
		"lower-is-better, got faster":   {lower, 80, false},
		"lower-is-better, unchanged":    {lower, 100, false},
		"higher-is-better, lost recall": {higher, 0.85, true},
		"higher-is-better, gained":      {higher, 0.93, false},
		// The case that decides the zero value. A spec with no baseline has nothing to
		// have regressed from, and reporting every unbaselined spec would make the
		// signal noise on its first run.
		"no baseline": {unbaselined, 9999, false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := tc.spec.Regressed(tc.value); got != tc.want {
				t.Errorf("Regressed(%v) = %v, want %v", tc.value, got, tc.want)
			}
		})
	}
}

// TestARegressionInsideTheBarStillMeets. The two predicates must not collapse: a value
// worse than baseline and better than Fail is admissible and has cost something, and
// both are true at once.
func TestARegressionInsideTheBarStillMeets(t *testing.T) {
	t.Parallel()
	s := nfr.Spec{Direction: nfr.Lower, Baseline: 100, Fail: 200, Goal: 50}
	if !s.Meets(150) {
		t.Error("150 breaches a Fail bar of 200")
	}
	if !s.Regressed(150) {
		t.Error("150 is not a regression from a baseline of 100")
	}
}

// wantCategory asserts whether a tag's leading category is accepted.
func wantCategory(t *testing.T, tag string, ok bool) {
	t.Helper()
	spec := nfr.Spec{
		ID: "s", Tag: tag, Scale: "count", Meter: "check",
		Direction: nfr.Lower, Fail: 10, Goal: 1,
	}
	err := spec.Valid()
	if ok && err != nil {
		t.Errorf("tag %q rejected: %v", tag, err)
	}
	if !ok && err == nil {
		t.Errorf("tag %q accepted, want rejected", tag)
	}
}

// TestTaxonomyCoversTheComplianceAxis. The FURPS+/25010 union has no category for a
// regulatory or audit requirement, and reconciling against a labelled corpus of 11,876
// requirement sentences showed that is not a rare gap: 21% of the nonfunctional labels
// name audit, legal or privacy. Before this, Valid rejected the tag, so a compliance
// requirement could not be written down at all.
func TestTaxonomyCoversTheComplianceAxis(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		tag  string
		ok   bool
	}{
		{"a standards category still validates", "Performance.Latency", true},
		{"audit, which the standards omit", "Audit.AccessLog", true},
		{"legal, for a regulatory citation", "Legal.HIPAA164514", true},
		{"privacy, which is not security", "Privacy.Minimisation", true},
		// The taxonomy is still closed: adding three categories must not turn it into
		// free prose, which is the whole reason a Tag is validated.
		{"an invented category is still refused", "Vibes.Speed", false},
		{"the corpus's catch-all is not adopted", "OtherNonfunctional.Thing", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			wantCategory(t, tt.tag, tt.ok)
		})
	}
}
