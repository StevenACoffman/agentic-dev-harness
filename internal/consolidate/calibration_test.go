package consolidate_test

import (
	"reflect"
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/internal/consolidate"
	"github.com/StevenACoffman/agentic-dev-harness/internal/evidence"
	"github.com/StevenACoffman/skillet/calibration"
)

// TestCalibrationExtractsProposedCandidates checks the sample extraction: only
// cycles that proposed a candidate become samples (the projected NewScore is the
// prediction, "kept" the outcome), and the metrics match skillet/calibration over
// those samples. Baseline (no candidate) and error records are skipped.
func TestCalibrationExtractsProposedCandidates(t *testing.T) {
	t.Parallel()
	records := []evidence.Record{
		{OldScore: 0.5, NewScore: 0.9, Status: evidence.StatusKeep},     // kept candidate
		{OldScore: 0.5, NewScore: 0.6, Status: evidence.StatusBaseline}, // rejected candidate
		{OldScore: 0.5, NewScore: 0.5, Status: evidence.StatusBaseline}, // no candidate — skipped
		{OldScore: 0, NewScore: 0, Status: evidence.StatusError},        // no candidate — skipped
	}
	got := consolidate.Calibration(records)
	want := calibration.Compute([]calibration.Sample{
		{Confidence: 0.9, Correct: true},  // kept
		{Confidence: 0.6, Correct: false}, // rejected
	})
	if got.Samples != 2 {
		t.Fatalf("Samples = %d, want 2 (baseline and error skipped)", got.Samples)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Calibration report = %+v, want %+v", got, want)
	}
}

// TestCalibrationEmptyWhenNoCandidates: a run with only no-candidate baselines
// yields the zero report — nothing was predicted, so there is nothing to score.
func TestCalibrationEmptyWhenNoCandidates(t *testing.T) {
	t.Parallel()
	records := []evidence.Record{
		{OldScore: 0.7, NewScore: 0.7, Status: evidence.StatusBaseline},
	}
	if rep := consolidate.Calibration(records); rep.Samples != 0 {
		t.Errorf("Samples = %d, want 0 (only a no-candidate baseline)", rep.Samples)
	}
}
