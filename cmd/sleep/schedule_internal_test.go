package sleep

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/StevenACoffman/agentic-dev-harness/cmd/root"
	"github.com/StevenACoffman/agentic-dev-harness/internal/schedule"
)

// fakeRunner records commands without spawning a process.
type fakeRunner struct{ ran [][]string }

func (f *fakeRunner) Run(_ context.Context, command []string) error {
	f.ran = append(f.ran, command)
	return nil
}

// testCfg builds a sleep Config wired to a fake runner, writing stdout to out.
func testCfg(t *testing.T, out *bytes.Buffer, jsonl bool, runner schedule.Runner) *Config {
	t.Helper()
	r := root.New(func(string) string { return "" }, strings.NewReader(""), out, &bytes.Buffer{})
	r.JSONL = jsonl
	cfg := New(r)
	cfg.runner = runner
	return cfg
}

func TestScheduleAddListRemove(t *testing.T) {
	t.Chdir(t.TempDir())
	var out bytes.Buffer
	cfg := testCfg(t, &out, false, &fakeRunner{})
	ctx := context.Background()

	if err := cfg.schedule(ctx, []string{"add", "nightly", "@daily", "sleep", "run"}); err != nil {
		t.Fatalf("add: %v", err)
	}
	out.Reset()
	if err := cfg.schedule(ctx, []string{"list"}); err != nil {
		t.Fatalf("list: %v", err)
	}
	if !strings.Contains(out.String(), "nightly") || !strings.Contains(out.String(), "@daily") {
		t.Errorf("list output = %q, want the job", out.String())
	}
	if err := cfg.schedule(ctx, []string{"remove", "nightly"}); err != nil {
		t.Fatalf("remove: %v", err)
	}
	out.Reset()
	if err := cfg.schedule(ctx, []string{"list"}); err != nil {
		t.Fatalf("list after remove: %v", err)
	}
	if strings.TrimSpace(out.String()) != "" {
		t.Errorf("list after remove = %q, want empty", out.String())
	}
}

func TestScheduleTickFiresDueJob(t *testing.T) {
	t.Chdir(t.TempDir())
	var out bytes.Buffer
	runner := &fakeRunner{}
	cfg := testCfg(t, &out, true, runner)
	ctx := context.Background()

	if err := cfg.schedule(
		ctx,
		[]string{"add", "dep", "@every 1h", "loop", "run", "dep"},
	); err != nil {
		t.Fatalf("add: %v", err)
	}
	// Force the job due by backdating its next fire directly in the store.
	forceDue(ctx, t, "dep")

	out.Reset()
	if err := cfg.schedule(ctx, []string{"tick"}); err != nil {
		t.Fatalf("tick: %v", err)
	}
	if len(runner.ran) != 1 || strings.Join(runner.ran[0], " ") != "loop run dep" {
		t.Fatalf("runner ran %v, want the job's command", runner.ran)
	}
	var rec struct {
		Status string `json:"status"`
		Data   struct {
			Fired int `json:"fired"`
		} `json:"data"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(out.Bytes()), &rec); err != nil {
		t.Fatalf("tick outcome not JSON: %v (%q)", err, out.String())
	}
	if rec.Status != root.StatusOK || rec.Data.Fired != 1 {
		t.Errorf("tick outcome = %+v, want ok with fired=1", rec)
	}
}

// forceDue backdates a job's next fire so the next tick runs it.
func forceDue(ctx context.Context, t *testing.T, name string) {
	t.Helper()
	store, err := schedule.Open(ctx, scheduleStoreDir)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer func() { _ = store.Close() }()
	if err := store.AdvanceNextFire(ctx, name, time.Now().UTC().Add(-time.Minute)); err != nil {
		t.Fatalf("force due: %v", err)
	}
}
