package loopcmd

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/cmd/root"
	"github.com/StevenACoffman/agentic-dev-harness/internal/state"
)

// fakeSensor reports a scripted finding without spawning a process.
type fakeSensor struct{ finding bool }

func (f fakeSensor) Sense(_ context.Context, _, _ string) bool { return f.finding }

// testConfig builds a loop Config wired to a fake sensor, writing stdout to out.
func testConfig(t *testing.T, out *bytes.Buffer, jsonl bool, s sensorRunner) *Config {
	t.Helper()
	r := root.New(func(string) string { return "" }, strings.NewReader(""), out, &bytes.Buffer{})
	r.JSONL = jsonl
	cfg := New(r)
	cfg.sensor = s
	return cfg
}

// writeLoop registers a single dep-scan loop whose finding opens an arc.
func writeLoop(t *testing.T) {
	t.Helper()
	if err := os.MkdirAll(".adh", 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	reg := `{"loops":[{"id":"dep-scan","goal":"no known-vulnerable dependency ships",` +
		`"sensor":"scan","on_finding":"open arc","retire_when":"policy moves into a check",` +
		`"owner":"security"}]}`
	if err := os.WriteFile(filepath.Join(".adh", "loops.json"), []byte(reg), 0o600); err != nil {
		t.Fatalf("write loops: %v", err)
	}
}

func TestRunOpensArcOnFinding(t *testing.T) {
	t.Chdir(t.TempDir())
	writeLoop(t)
	var out bytes.Buffer
	cfg := testConfig(t, &out, true, fakeSensor{finding: true})

	if err := cfg.exec(context.Background(), []string{"run", "dep-scan"}); err != nil {
		t.Fatalf("run: %v", err)
	}

	arcs, err := state.Default().List()
	if err != nil {
		t.Fatalf("list arcs: %v", err)
	}
	if len(arcs) != 1 {
		t.Fatalf("got %d arcs, want one opened by the loop", len(arcs))
	}
	got := arcs[0]
	if got.Title != "no known-vulnerable dependency ships" {
		t.Errorf("arc title = %q, want the loop goal", got.Title)
	}
	if len(got.Labels) != 1 || got.Labels[0] != "security" {
		t.Errorf("arc labels = %v, want [security] (the loop owner)", got.Labels)
	}
	// The outcome carries the opened arc id for the agent to drive.
	var rec struct {
		Status string            `json:"status"`
		Data   map[string]string `json:"data"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(out.Bytes()), &rec); err != nil {
		t.Fatalf("outcome not JSON: %v (%q)", err, out.String())
	}
	if rec.Status != root.StatusOK || rec.Data["arc"] != got.ID {
		t.Errorf("outcome = %+v, want ok carrying arc %s", rec, got.ID)
	}
}

func TestRunNoArcWhenInvariantHolds(t *testing.T) {
	t.Chdir(t.TempDir())
	writeLoop(t)
	var out bytes.Buffer
	cfg := testConfig(t, &out, false, fakeSensor{finding: false})

	if err := cfg.exec(context.Background(), []string{"run", "dep-scan"}); err != nil {
		t.Fatalf("run: %v", err)
	}
	arcs, err := state.Default().List()
	if err != nil {
		t.Fatalf("list arcs: %v", err)
	}
	if len(arcs) != 0 {
		t.Errorf("invariant holds but %d arcs were opened", len(arcs))
	}
	if !strings.Contains(out.String(), "invariant holds") {
		t.Errorf("run output = %q, want it to report the invariant holds", out.String())
	}
}
