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

func TestMcNemar(t *testing.T) {
	t.Parallel()
	if _, sig := verdict.McNemar(0, 0); sig {
		t.Error("no discordant pairs cannot be significant")
	}
	if _, sig := verdict.McNemar(2, 1); sig {
		t.Error("3 discordant pairs is below the χ² critical value")
	}
	if _, sig := verdict.McNemar(12, 0); !sig {
		t.Error("12 vs 0 discordant pairs should be significant")
	}
}

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
