package cmd_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/state"
)

// jsonLines splits command output into its JSON Lines records, asserting each is
// a well-formed JSON object. It returns the decoded objects.
func jsonLines(t *testing.T, out string) []map[string]any {
	t.Helper()
	trimmed := strings.TrimRight(out, "\n")
	if trimmed == "" {
		return nil
	}
	lines := strings.Split(trimmed, "\n")
	records := make([]map[string]any, 0, len(lines))
	for _, line := range lines {
		var rec map[string]any
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatalf("line is not a JSON object: %v\nline: %q", err, line)
		}
		records = append(records, rec)
	}
	return records
}

func TestStatusJSONL(t *testing.T) {
	t.Chdir(t.TempDir())
	seed := []adh.Arc{
		{ID: "arc-0001", Stage: adh.StageExecution, Status: adh.StatusOpen},
		{ID: "arc-0002", Stage: adh.StageOps, Status: adh.StatusBlocked},
	}
	for i := range seed {
		if err := state.Default().Save(&seed[i]); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	recs := jsonLines(t, mustRun(t, "status", "--jsonl"))
	if len(recs) != 1 {
		t.Fatalf("status --jsonl = %d lines, want 1 summary", len(recs))
	}
	r := recs[0]
	if r["arcs_total"] != 2.0 || r["pending_gates"] != 1.0 || r["autonomy"] != "L2" {
		t.Errorf("summary = %v, want total 2, pending 1, autonomy L2", r)
	}
}

func TestArcListJSONL(t *testing.T) {
	t.Chdir(t.TempDir())
	seed := []adh.Arc{
		{ID: "arc-0001", Stage: adh.StageStrategy, Status: adh.StatusOpen, Title: "first"},
		{ID: "arc-0002", Stage: adh.StageOps, Status: adh.StatusClosed, Title: "second"},
	}
	for i := range seed {
		if err := state.Default().Save(&seed[i]); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	// The global flag precedes the positional verb (ff stops at the first positional).
	recs := jsonLines(t, mustRun(t, "arc", "--jsonl", "list"))
	if len(recs) != 2 {
		t.Fatalf("arc list --jsonl = %d lines, want one per arc", len(recs))
	}
	if recs[0]["id"] != "arc-0001" || recs[0]["title"] != "first" {
		t.Errorf("first record = %v, want arc-0001/first", recs[0])
	}
}

func TestArcShowJSONL(t *testing.T) {
	t.Chdir(t.TempDir())
	arc := adh.Arc{
		ID:     "arc-0001",
		Title:  "widen it",
		Stage:  adh.StageCritic,
		Status: adh.StatusOpen,
		Labels: []string{"internal"},
	}
	if err := state.Default().Save(&arc); err != nil {
		t.Fatalf("seed: %v", err)
	}
	recs := jsonLines(t, mustRun(t, "arc", "--jsonl", "show", "arc-0001"))
	if len(recs) != 1 {
		t.Fatalf("arc show --jsonl = %d lines, want 1", len(recs))
	}
	if recs[0]["id"] != "arc-0001" || recs[0]["stage"] != "critic" {
		t.Errorf("show record = %v, want the full arc", recs[0])
	}
}

func TestGateListJSONL(t *testing.T) {
	t.Chdir(t.TempDir())
	seed := []adh.Arc{
		{ID: "arc-0001", Stage: adh.StageExecution, Status: adh.StatusOpen},
		{
			ID:      "arc-0002",
			Stage:   adh.StageOps,
			Status:  adh.StatusBlocked,
			History: []string{"blocked: awaiting ops approval"},
		},
	}
	for i := range seed {
		if err := state.Default().Save(&seed[i]); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	recs := jsonLines(t, mustRun(t, "gate", "--jsonl", "list"))
	if len(recs) != 1 {
		t.Fatalf("gate list --jsonl = %d lines, want only the blocked arc", len(recs))
	}
	if recs[0]["arc"] != "arc-0002" || recs[0]["reason"] != "awaiting ops approval" {
		t.Errorf("gate record = %v, want arc-0002 with its reason", recs[0])
	}
}

func TestGateListJSONLEmptyIsNoLines(t *testing.T) {
	t.Chdir(t.TempDir())
	arc := adh.Arc{ID: "arc-0001", Stage: adh.StageExecution, Status: adh.StatusOpen}
	if err := state.Default().Save(&arc); err != nil {
		t.Fatalf("seed: %v", err)
	}
	// No blocked arcs: JSONL mode is an empty stream, not a "no pending gates" line.
	if out := mustRun(t, "gate", "--jsonl", "list"); strings.TrimSpace(out) != "" {
		t.Errorf("gate list --jsonl with no gates = %q, want empty", out)
	}
}

func TestEvalJSONL(t *testing.T) {
	t.Chdir(t.TempDir())
	// An arc at evaluation with no findings: eval advances it to ops (exit 0) and
	// emits one disposition record.
	arc := adh.Arc{ID: "arc-0001", Stage: adh.StageEvaluation, Status: adh.StatusOpen}
	if err := state.Default().Save(&arc); err != nil {
		t.Fatalf("seed: %v", err)
	}
	recs := jsonLines(t, mustRun(t, "eval", "--jsonl", "arc-0001"))
	if len(recs) != 1 {
		t.Fatalf("eval --jsonl = %d lines, want 1 disposition", len(recs))
	}
	r := recs[0]
	if r["arc"] != "arc-0001" || r["returned_to_execution"] != false || r["stage"] != "ops" {
		t.Errorf("disposition = %v, want arc-0001 advanced to ops", r)
	}
}
