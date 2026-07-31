package metrics_test

import (
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/internal/metrics"
)

func TestSummarize(t *testing.T) {
	records := []metrics.Record{
		{Arc: "arc-1", AttentionMinutes: 10, ComputeTokens: 100, Accepted: true},
		{Arc: "arc-2", AttentionMinutes: 30, ComputeTokens: 200, Accepted: false},
		{Arc: "arc-3", AttentionMinutes: 10, ComputeTokens: 150, Accepted: true},
	}
	got := metrics.Summarize(records)
	if got.Arcs != 3 || got.Accepted != 2 || got.AttentionMinutes != 50 {
		t.Fatalf("Summarize = %+v", got)
	}
	if got.AttentionPerAccept != 25.0 {
		t.Errorf("AttentionPerAccept = %v, want 25", got.AttentionPerAccept)
	}
}

func TestSummarizeNoAccepts(t *testing.T) {
	got := metrics.Summarize([]metrics.Record{{AttentionMinutes: 5}})
	if got.AttentionPerAccept != 5.0 {
		t.Errorf(
			"AttentionPerAccept with zero accepts = %v, want 5 (no divide by zero)",
			got.AttentionPerAccept,
		)
	}
}

func TestAttentionDelta(t *testing.T) {
	cur := metrics.Summary{AttentionPerAccept: 30}
	prev := metrics.Summary{AttentionPerAccept: 20}
	if d := cur.AttentionDelta(prev); d != 10 {
		t.Errorf("AttentionDelta = %v, want 10 (a regression)", d)
	}
}
