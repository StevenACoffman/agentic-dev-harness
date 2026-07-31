package cmd_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/cmd/root"
	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/contextstore"
	"github.com/StevenACoffman/agentic-dev-harness/internal/state"
)

// stepResult mirrors the payload the step command carries in an --jsonl outcome's
// data field.
type stepResult struct {
	Arc    string `json:"arc"`
	Stage  string `json:"stage"`
	Status string `json:"status"`
	Prompt string `json:"prompt"`
}

// seedOpenArc writes an open arc at the strategy stage into the current (temp)
// workspace and returns its id.
func seedOpenArc(t *testing.T) string {
	t.Helper()
	arc := adh.Arc{ID: "arc-0001", Stage: adh.StageStrategy, Status: adh.StatusOpen}
	if err := state.Default().Save(&arc); err != nil {
		t.Fatalf("seed arc: %v", err)
	}
	return arc.ID
}

func decodeStep(t *testing.T, out string) stepResult {
	t.Helper()
	// step emits the outcome envelope {status, code, data:{...}}; the step payload
	// is the data.
	var env struct {
		Status string     `json:"status"`
		Data   stepResult `json:"data"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("decode step JSON %q: %v", out, err)
	}
	if env.Status != root.StatusOK {
		t.Fatalf("step outcome status = %q, want ok\n%s", env.Status, out)
	}
	return env.Data
}

func TestStepRelayEmitParksPending(t *testing.T) {
	t.Chdir(t.TempDir())
	id := seedOpenArc(t)

	out := mustRun(t, "step", "--relay", "--jsonl", id)
	got := decodeStep(t, out)
	if got.Status != "awaiting" {
		t.Errorf("status = %q, want awaiting", got.Status)
	}
	if got.Stage != string(adh.StageStrategy) {
		t.Errorf("stage = %q, want strategy (emit does not advance)", got.Stage)
	}
	if got.Prompt == "" {
		t.Error("awaiting result carried no prompt")
	}

	arc, err := state.Default().Get(id)
	if err != nil {
		t.Fatalf("reload arc: %v", err)
	}
	if arc.Pending == nil || arc.Pending.Stage != adh.StageStrategy {
		t.Fatalf("pending turn not parked at strategy: %+v", arc.Pending)
	}
	if arc.Stage != adh.StageStrategy {
		t.Errorf("arc advanced on emit: stage = %s", arc.Stage)
	}
}

func TestStepRelayEmitIsIdempotent(t *testing.T) {
	t.Chdir(t.TempDir())
	id := seedOpenArc(t)

	first := decodeStep(t, mustRun(t, "step", "--relay", "--jsonl", id))
	second := decodeStep(t, mustRun(t, "step", "--relay", "--jsonl", id))
	if first.Prompt != second.Prompt {
		t.Errorf("re-emit changed the prompt:\n%q\nvs\n%q", first.Prompt, second.Prompt)
	}
	arc, err := state.Default().Get(id)
	if err != nil {
		t.Fatalf("reload arc: %v", err)
	}
	if arc.Stage != adh.StageStrategy {
		t.Errorf("re-emit advanced the arc: stage = %s", arc.Stage)
	}
}

func TestStepRelayStrategyReplyChoosesResolution(t *testing.T) {
	t.Chdir(t.TempDir())
	id := seedOpenArc(t)
	mustRun(t, "step", "--relay", "--jsonl", id) // emit the strategy prompt, park

	respPath := filepath.Join(t.TempDir(), "reply.txt")
	reply := "resolution: investigation\ninspect the crash logs, no code change"
	if err := os.WriteFile(respPath, []byte(reply), 0o600); err != nil {
		t.Fatalf("write reply: %v", err)
	}
	mustRun(t, "step", "--relay", "--response", respPath, "--jsonl", id)

	arc, err := state.Default().Get(id)
	if err != nil {
		t.Fatalf("reload arc: %v", err)
	}
	// Strategy chose the resolution from the reply (§12) instead of defaulting.
	if arc.Resolution != adh.ResolutionInvestigation {
		t.Errorf("resolution = %q, want investigation", arc.Resolution)
	}
	if arc.Stage != adh.StageExecution {
		t.Errorf("stage = %q, want execution after the strategy turn", arc.Stage)
	}
}

func TestStepRelayResumeAdvances(t *testing.T) {
	t.Chdir(t.TempDir())
	id := seedOpenArc(t)
	mustRun(t, "step", "--relay", "--jsonl", id) // open the turn

	respPath := filepath.Join(t.TempDir(), "reply.txt")
	if err := os.WriteFile(respPath, []byte("chose a code change; steps: ..."), 0o600); err != nil {
		t.Fatalf("write reply: %v", err)
	}

	out := mustRun(t, "step", "--relay", "--response", respPath, "--jsonl", id)
	got := decodeStep(t, out)
	if got.Status != "advanced" {
		t.Errorf("status = %q, want advanced", got.Status)
	}
	if got.Stage != string(adh.StageExecution) {
		t.Errorf("stage = %q, want execution", got.Stage)
	}

	arc, err := state.Default().Get(id)
	if err != nil {
		t.Fatalf("reload arc: %v", err)
	}
	if arc.Pending != nil {
		t.Errorf("pending turn not cleared after resume: %+v", arc.Pending)
	}
	if len(arc.History) != 1 {
		t.Errorf("history len = %d, want 1 (the relayed reply)", len(arc.History))
	}
}

func TestStepRelayResumeWithoutPendingFails(t *testing.T) {
	t.Chdir(t.TempDir())
	id := seedOpenArc(t)

	respPath := filepath.Join(t.TempDir(), "reply.txt")
	if err := os.WriteFile(respPath, []byte("a reply with no open turn"), 0o600); err != nil {
		t.Fatalf("write reply: %v", err)
	}
	if _, err := run(t, "step", "--relay", "--response", respPath, id); err == nil {
		t.Error("resume with no pending turn succeeded, want an error")
	}
}

// seedCriticArc writes an open arc parked at the critic stage with the given
// routing footprint.
func seedCriticArc(t *testing.T, labels, paths []string) string {
	t.Helper()
	arc := adh.Arc{
		ID:         "arc-0001",
		Stage:      adh.StageCritic,
		Status:     adh.StatusOpen,
		Resolution: adh.ResolutionChange,
		Labels:     labels,
		Paths:      paths,
	}
	if err := state.Default().Save(&arc); err != nil {
		t.Fatalf("seed critic arc: %v", err)
	}
	return arc.ID
}

// writeContextUnit drops a routable context unit into the store.
func writeContextUnit(t *testing.T, id string, labels []string) {
	t.Helper()
	if err := os.MkdirAll(contextstore.DefaultStoreDir, 0o750); err != nil {
		t.Fatalf("mkdir context: %v", err)
	}
	unit := contextUnitJSON(t, id, labels)
	if err := os.WriteFile(
		filepath.Join(contextstore.DefaultStoreDir, id+".json"),
		unit,
		0o600,
	); err != nil {
		t.Fatalf("write unit: %v", err)
	}
}

func contextUnitJSON(t *testing.T, id string, labels []string) []byte {
	t.Helper()
	data, err := json.Marshal(map[string]any{"id": id, "kind": "runbook", "labels": labels})
	if err != nil {
		t.Fatalf("marshal unit: %v", err)
	}
	return data
}

func TestStepRelayCriticEmitCarriesGrounding(t *testing.T) {
	t.Chdir(t.TempDir())
	writeContextUnit(t, "u-auth", []string{"auth"})
	id := seedCriticArc(t, []string{"auth"}, []string{"internal/authz/policy.go"})

	got := decodeStep(t, mustRun(t, "step", "--relay", "--jsonl", id))
	if got.Status != "awaiting" {
		t.Fatalf("status = %q, want awaiting", got.Status)
	}
	for _, want := range []string{"u-auth", "internal/authz/policy.go", "tests pass"} {
		if !strings.Contains(got.Prompt, want) {
			t.Errorf("critic prompt missing grounding %q:\n%s", want, got.Prompt)
		}
	}
}

func TestStepRelayCriticRoutingGapExits12(t *testing.T) {
	t.Chdir(t.TempDir())
	// The store exists (a unit) but nothing routes for the arc's labels — the
	// environment is set up yet did not teach this arc: a routing gap (exit 12).
	writeContextUnit(t, "u-billing", []string{"billing"})
	id := seedCriticArc(t, []string{"auth"}, nil)

	_, err := run(t, "step", "--relay", id)
	var exit root.ExitError
	if !errors.As(err, &exit) || int(exit) != 12 {
		t.Fatalf("populated non-matching store = %v, want ExitError(12)", err)
	}
	arc, getErr := state.Default().Get(id)
	if getErr != nil {
		t.Fatalf("reload arc: %v", getErr)
	}
	if arc.Pending != nil {
		t.Error("a routing gap must not park a pending turn")
	}
}

func TestStepRelayCriticNoStoreEmitsUngrounded(t *testing.T) {
	t.Chdir(t.TempDir()) // no .adh/context store: grounding not configured, not a gap
	id := seedCriticArc(t, []string{"auth"}, nil)

	got := decodeStep(t, mustRun(t, "step", "--relay", "--jsonl", id))
	if got.Status != "awaiting" {
		t.Fatalf("status = %q, want awaiting (no store is not a gap)", got.Status)
	}
	if !strings.Contains(got.Prompt, "No repository grounding") {
		t.Errorf("ungrounded critic prompt should say so:\n%s", got.Prompt)
	}
}

func TestStepRelayCriticUndeclaredEmitsUngrounded(t *testing.T) {
	t.Chdir(t.TempDir())
	id := seedCriticArc(t, nil, nil) // declares no footprint → not a gap

	got := decodeStep(t, mustRun(t, "step", "--relay", "--jsonl", id))
	if got.Status != "awaiting" {
		t.Fatalf("status = %q, want awaiting (undeclared is not a gap)", got.Status)
	}
	if !strings.Contains(got.Prompt, "No repository grounding") {
		t.Errorf("ungrounded critic prompt should say so:\n%s", got.Prompt)
	}
}

func TestStepRelayCriticResumeStoresFindings(t *testing.T) {
	t.Chdir(t.TempDir())
	id := seedCriticArc(t, nil, nil) // undeclared → no routing gap
	mustRun(t, "step", "--relay", "--jsonl", id)

	respPath := filepath.Join(t.TempDir(), "findings.json")
	reply := `{"findings":[{"summary":"clears differ","kind":"oracle","ref":"corpus"}]}`
	if err := os.WriteFile(respPath, []byte(reply), 0o600); err != nil {
		t.Fatalf("write findings: %v", err)
	}

	got := decodeStep(t, mustRun(t, "step", "--relay", "--response", respPath, "--jsonl", id))
	if got.Status != "advanced" || got.Stage != string(adh.StageEvaluation) {
		t.Fatalf("resume = %+v, want advanced to evaluation", got)
	}
	arc, err := state.Default().Get(id)
	if err != nil {
		t.Fatalf("reload arc: %v", err)
	}
	if len(arc.Findings) != 1 || arc.Findings[0].Kind != adh.FindingOracle {
		t.Errorf("stored findings = %+v, want one oracle finding", arc.Findings)
	}
}

func TestStepRelayCriticResumeRejectsNonFindings(t *testing.T) {
	t.Chdir(t.TempDir())
	id := seedCriticArc(t, nil, nil)
	mustRun(t, "step", "--relay", "--jsonl", id)

	respPath := filepath.Join(t.TempDir(), "reply.txt")
	if err := os.WriteFile(respPath, []byte("looks fine to me"), 0o600); err != nil {
		t.Fatalf("write reply: %v", err)
	}
	if _, err := run(t, "step", "--relay", "--response", respPath, id); err == nil {
		t.Error("a free-text critic reply should be rejected, want an error")
	}
	arc, err := state.Default().Get(id)
	if err != nil {
		t.Fatalf("reload arc: %v", err)
	}
	if arc.Stage != adh.StageCritic {
		t.Errorf("a rejected reply advanced the arc: stage = %s", arc.Stage)
	}
}

func TestStepRelayRefusesEvaluation(t *testing.T) {
	t.Chdir(t.TempDir())
	arc := adh.Arc{ID: "arc-0001", Stage: adh.StageEvaluation, Status: adh.StatusOpen}
	if err := state.Default().Save(&arc); err != nil {
		t.Fatalf("seed arc: %v", err)
	}
	_, err := run(t, "step", "--relay", arc.ID)
	if err == nil || !strings.Contains(err.Error(), "adh eval") {
		t.Errorf("step --relay at evaluation = %v, want a pointer to `adh eval`", err)
	}
}

func TestStepRelayResumeRejectsEmptyReply(t *testing.T) {
	t.Chdir(t.TempDir())
	id := seedOpenArc(t)
	mustRun(t, "step", "--relay", "--jsonl", id)

	respPath := filepath.Join(t.TempDir(), "reply.txt")
	if err := os.WriteFile(respPath, []byte("   \n"), 0o600); err != nil {
		t.Fatalf("write reply: %v", err)
	}
	if _, err := run(t, "step", "--relay", "--response", respPath, id); err == nil {
		t.Error("resume with an empty reply succeeded, want an error")
	}
}
