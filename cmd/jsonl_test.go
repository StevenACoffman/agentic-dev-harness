package cmd_test

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/cmd/root"
	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/state"
)

// assertBlocked checks a command exited with wantCode and emitted a blocked
// outcome carrying wantReason — the structured signal the relay branches on
// instead of matching stderr prose.
func assertBlocked(t *testing.T, out string, err error, wantCode int, wantReason string) {
	t.Helper()
	var exit root.ExitError
	if !errors.As(err, &exit) || int(exit) != wantCode {
		t.Fatalf("exit = %v, want ExitError(%d)", err, wantCode)
	}
	recs := jsonLines(t, out)
	if len(recs) != 1 {
		t.Fatalf("blocked output = %d lines, want 1 outcome:\n%s", len(recs), out)
	}
	rec := recs[0]
	if rec["status"] != root.StatusBlocked || rec["reason"] != wantReason ||
		rec["code"] != float64(wantCode) {
		t.Errorf("blocked outcome = %v, want status blocked / reason %q / code %d",
			rec, wantReason, wantCode)
	}
}

// okData asserts rec is a success outcome envelope and returns its data payload.
func okData(t *testing.T, rec map[string]any) map[string]any {
	t.Helper()
	if rec["status"] != root.StatusOK {
		t.Fatalf("outcome status = %v, want ok: %v", rec["status"], rec)
	}
	data, ok := rec["data"].(map[string]any)
	if !ok {
		t.Fatalf("outcome carried no data payload: %v", rec)
	}
	return data
}

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
	d := okData(t, recs[0])
	if d["arcs_total"] != 2.0 || d["pending_gates"] != 1.0 || d["autonomy"] != "L2" {
		t.Errorf("summary = %v, want total 2, pending 1, autonomy L2", d)
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
	d := okData(t, recs[0])
	if d["id"] != "arc-0001" || d["title"] != "first" {
		t.Errorf("first record = %v, want arc-0001/first", d)
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
	d := okData(t, recs[0])
	if d["id"] != "arc-0001" || d["stage"] != "critic" {
		t.Errorf("show record = %v, want the full arc", d)
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
	d := okData(t, recs[0])
	if d["arc"] != "arc-0002" || d["reason"] != "awaiting ops approval" {
		t.Errorf("gate record = %v, want arc-0002 with its reason", d)
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

// assertError checks a command exited with wantCode and emitted an error outcome
// carrying wantReason.
func assertError(t *testing.T, out string, err error, wantCode int, wantReason string) {
	t.Helper()
	var exit root.ExitError
	if !errors.As(err, &exit) || int(exit) != wantCode {
		t.Fatalf("exit = %v, want ExitError(%d)", err, wantCode)
	}
	recs := jsonLines(t, out)
	if len(recs) != 1 {
		t.Fatalf("error output = %d lines, want 1 outcome:\n%s", len(recs), out)
	}
	rec := recs[0]
	if rec["status"] != root.StatusError || rec["reason"] != wantReason ||
		rec["code"] != float64(wantCode) {
		t.Errorf("error outcome = %v, want status error / reason %q / code %d",
			rec, wantReason, wantCode)
	}
}

func TestDispatcherErrorEnvelopeJSONL(t *testing.T) {
	t.Chdir(t.TempDir())
	// A plain returned error (no such arc) becomes a structured error outcome from
	// the dispatcher, not a usage banner — the agent parses it like any outcome.
	out, err := run(t, "arc", "--jsonl", "show", "arc-9999")
	var exit root.ExitError
	if !errors.As(err, &exit) {
		t.Fatalf("want an ExitError, got %v", err)
	}
	recs := jsonLines(t, out)
	if len(recs) != 1 {
		t.Fatalf("dispatcher error = %d lines, want 1 outcome:\n%s", len(recs), out)
	}
	rec := recs[0]
	if rec["status"] != root.StatusError || rec["message"] == "" || rec["code"] == 0.0 {
		t.Errorf("error outcome = %v, want status error with a message and non-zero code", rec)
	}
}

func TestApproveGateBlockedJSONL(t *testing.T) {
	t.Chdir(t.TempDir())
	arc := adh.Arc{ID: "arc-0001", Stage: adh.StageOps, Status: adh.StatusBlocked}
	if err := state.Default().Save(&arc); err != nil {
		t.Fatalf("seed: %v", err)
	}
	// No --phrase: the gate is not satisfied → a blocked outcome, exit 4.
	out, err := run(t, "approve", "--jsonl", "arc-0001")
	assertBlocked(t, out, err, 4, root.ReasonGate)
}

func TestApproveOKJSONL(t *testing.T) {
	t.Chdir(t.TempDir())
	arc := adh.Arc{ID: "arc-0001", Stage: adh.StageOps, Status: adh.StatusBlocked}
	if err := state.Default().Save(&arc); err != nil {
		t.Fatalf("seed: %v", err)
	}
	// The phrase is the arc id; global --jsonl and local --phrase precede the id.
	recs := jsonLines(t, mustRun(t, "approve", "--jsonl", "--phrase", "arc-0001", "arc-0001"))
	d := okData(t, recs[0])
	if d["status"] != "approved" {
		t.Errorf("approve data = %v, want status approved", d)
	}
}

func TestRejectOKJSONL(t *testing.T) {
	t.Chdir(t.TempDir())
	arc := adh.Arc{ID: "arc-0001", Stage: adh.StageOps, Status: adh.StatusBlocked}
	if err := state.Default().Save(&arc); err != nil {
		t.Fatalf("seed: %v", err)
	}
	recs := jsonLines(t, mustRun(t, "reject", "--jsonl", "arc-0001"))
	d := okData(t, recs[0])
	if d["status"] != "rejected" || d["stage"] != "execution" {
		t.Errorf("reject data = %v, want rejected / execution", d)
	}
}

func TestCloseNoProofErrorJSONL(t *testing.T) {
	t.Chdir(t.TempDir())
	// A change arc at ops with no proof: NO-PROOF-NO-CLOSE fails it (exit 8).
	arc := adh.Arc{
		ID:         "arc-0001",
		Stage:      adh.StageOps,
		Status:     adh.StatusOpen,
		Resolution: adh.ResolutionChange,
	}
	if err := state.Default().Save(&arc); err != nil {
		t.Fatalf("seed: %v", err)
	}
	out, err := run(t, "close", "--jsonl", "arc-0001")
	assertError(t, out, err, 8, root.ReasonProof)
}

func TestProofVerifyFailErrorJSONL(t *testing.T) {
	t.Chdir(t.TempDir())
	// A manifest naming a missing artifact fails verification (exit 8).
	manifest := "packet.json"
	body := `{"arc":"arc-0001","artifacts":[{"path":"gone.txt","sha256":"deadbeefdeadbeef"}]}`
	if err := os.WriteFile(manifest, []byte(body), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	out, err := run(t, "proof", "verify", "--jsonl", manifest)
	assertError(t, out, err, 8, root.ReasonProof)
}

func TestStepAtOpsBlockedJSONL(t *testing.T) {
	t.Chdir(t.TempDir())
	arc := adh.Arc{ID: "arc-0001", Stage: adh.StageOps, Status: adh.StatusOpen}
	if err := state.Default().Save(&arc); err != nil {
		t.Fatalf("seed: %v", err)
	}
	out, err := run(t, "step", "--relay", "--jsonl", "arc-0001")
	assertBlocked(t, out, err, 4, root.ReasonAtOps)
}

func TestStepRoutingGapBlockedJSONL(t *testing.T) {
	t.Chdir(t.TempDir())
	// A populated store that routes nothing for the arc's labels is a routing gap
	// (helpers from steprelay_test.go, same package).
	writeContextUnit(t, "u-billing", []string{"billing"})
	id := seedCriticArc(t, []string{"auth"}, nil)
	out, err := run(t, "step", "--relay", "--jsonl", id)
	assertBlocked(t, out, err, 12, root.ReasonUngrounded)
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
	d := okData(t, recs[0])
	if d["arc"] != "arc-0001" || d["stage"] != "ops" {
		t.Errorf("disposition = %v, want arc-0001 advanced to ops", d)
	}
}
