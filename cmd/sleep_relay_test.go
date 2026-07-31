package cmd_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/state"
)

// seedClosedArc saves a closed arc carrying a failure history line, so the
// consolidation cycle harvests a reflected failure class from it.
func seedClosedArc(t *testing.T, id, historyLine string) {
	t.Helper()
	arc := adh.Arc{
		ID:      id,
		Stage:   adh.StageOps,
		Status:  adh.StatusClosed,
		History: []string{historyLine},
	}
	if err := state.Default().Save(&arc); err != nil {
		t.Fatalf("seed closed arc: %v", err)
	}
}

// TestSleepRelayEmitsProposalPrompt: `sleep run --relay` (no reply) emits the
// agent-driven proposal prompt naming the reflected class and the LEARNED region.
func TestSleepRelayEmitsProposalPrompt(t *testing.T) {
	t.Chdir(t.TempDir())
	seedClosedArc(t, "arc-0001", "critic: missing test coverage")
	out := mustRun(t, "sleep", "--relay", "run")
	if !strings.Contains(out, "missing test coverage") {
		t.Errorf("relay proposal prompt should name the reflected class:\n%s", out)
	}
	if !strings.Contains(out, "LEARNED region") {
		t.Errorf("relay proposal prompt should show the LEARNED region:\n%s", out)
	}
}

// TestSleepRelayResumeFeedsGate: `sleep run --relay --response` feeds the agent's
// edit through adh's held-out gate; an empty edit is gated out, staging nothing.
func TestSleepRelayResumeFeedsGate(t *testing.T) {
	t.Chdir(t.TempDir())
	seedClosedArc(t, "arc-0001", "critic: missing test coverage")
	reply := filepath.Join(t.TempDir(), "reply.md")
	if err := os.WriteFile(reply, []byte(""), 0o600); err != nil {
		t.Fatalf("write reply: %v", err)
	}
	out := mustRun(t, "sleep", "--relay", "--response", reply, "run")
	if !strings.Contains(out, "no proposal staged") {
		t.Errorf("an empty relay edit should stage nothing (gated out):\n%s", out)
	}
}
