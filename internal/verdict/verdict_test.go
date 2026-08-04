package verdict_test

import (
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/verdict"
)

func TestDecide(t *testing.T) {
	t.Parallel()
	sig := verdict.Outcome{Delta: 3, Significant: true}
	tests := []struct {
		name           string
		primary        verdict.Outcome
		replication    verdict.Outcome
		hasReplication bool
		want           verdict.Verdict
	}{
		{"regression kills", verdict.Outcome{Delta: -0.1}, sig, true, verdict.Kill},
		{"no replication", verdict.Outcome{Delta: 0.2}, sig, false, verdict.ReplicationMissing},
		{"meaningful + replicated", verdict.Outcome{Delta: 0.2}, sig, true, verdict.Elevate},
		{
			"gain too small to elevate",
			verdict.Outcome{Delta: 0.01},
			sig, true, verdict.Directional,
		},
		{
			"replication not significant",
			verdict.Outcome{Delta: 0.2},
			verdict.Outcome{Delta: 1, Significant: false},
			true,
			verdict.Directional,
		},
		{
			"replication regressed",
			verdict.Outcome{Delta: 0.2},
			verdict.Outcome{Delta: -1, Significant: true},
			true,
			verdict.Directional,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := verdict.Decide(
				tt.primary,
				tt.replication,
				verdict.DefaultMinEffect,
				tt.hasReplication,
			)
			if got != tt.want {
				t.Errorf("Decide = %q, want %q", got, tt.want)
			}
		})
	}
}

// McNemar moved to skillet/stats; see stats.TestMcNemarSignificance.

func TestValidateSplits(t *testing.T) {
	t.Parallel()
	clean := []verdict.SplitAssignment{{ID: "a", Split: "selection"}, {ID: "b", Split: "test"}}
	if err := verdict.ValidateSplits(clean); err != nil {
		t.Errorf("clean splits should validate: %v", err)
	}
	leaky := []verdict.SplitAssignment{{ID: "a", Split: "selection"}, {ID: "a", Split: "test"}}
	if err := verdict.ValidateSplits(leaky); adh.ErrorCode(err) != adh.EINVALID {
		t.Errorf("a leaked id should be EINVALID, got %v", err)
	}
}

func TestReplicate(t *testing.T) {
	t.Parallel()
	pass := verdict.Outcome{Delta: 0.2, Significant: true}
	tests := []struct {
		name string
		runs []verdict.Outcome
		want verdict.Verdict
	}{
		{"two independent passes elevate", []verdict.Outcome{pass, pass}, verdict.Elevate},
		{"one run cannot replicate", []verdict.Outcome{pass}, verdict.ReplicationMissing},
		{"none is replication-missing", nil, verdict.ReplicationMissing},
		{
			"a regression kills",
			[]verdict.Outcome{pass, {Delta: -0.1, Significant: true}},
			verdict.Kill,
		},
		{
			"one directional run does not elevate",
			[]verdict.Outcome{pass, {Delta: 0.2, Significant: false}},
			verdict.Directional,
		},
		{
			"a sub-threshold run does not elevate",
			[]verdict.Outcome{pass, {Delta: 0.01, Significant: true}},
			verdict.Directional,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := verdict.Replicate(tt.runs, verdict.DefaultMinEffect); got != tt.want {
				t.Errorf("Replicate = %q, want %q", got, tt.want)
			}
		})
	}
}
