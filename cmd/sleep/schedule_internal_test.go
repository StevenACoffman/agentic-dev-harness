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

// fakeRunner records commands without spawning a process. When fired is set it
// also signals each run there, so a daemon test can wait for a job to fire.
type fakeRunner struct {
	ran   [][]string
	fired chan []string
}

func (f *fakeRunner) Run(_ context.Context, command []string) error {
	f.ran = append(f.ran, command)
	if f.fired != nil {
		f.fired <- command
	}
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
	forceDue(ctx, t, cfg.scheduleDir(), "dep")

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

// TestScheduleRunFiresThenStops runs the daemon against a due job, waits for it to
// fire through the fake runner, then cancels the context and asserts the daemon
// shuts down cleanly (graceful stop from SIGINT/SIGTERM's ctx).
func TestScheduleRunFiresThenStops(t *testing.T) {
	t.Chdir(t.TempDir())
	var out bytes.Buffer
	runner := &fakeRunner{fired: make(chan []string, 1)}
	cfg := testCfg(t, &out, false, runner)
	ctx := context.Background()

	if err := cfg.schedule(
		ctx,
		[]string{"add", "dep", "@every 1h", "loop", "run", "dep"},
	); err != nil {
		t.Fatalf("add: %v", err)
	}
	store, err := schedule.Open(ctx, cfg.scheduleDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer func() { _ = store.Close() }()
	if err := store.AdvanceNextFire(ctx, "dep", time.Now().UTC().Add(-time.Minute)); err != nil {
		t.Fatalf("force due: %v", err)
	}

	daemonCtx, cancel := context.WithCancel(ctx)
	errc := make(chan error, 1)
	go func() { errc <- cfg.scheduleRun(daemonCtx, store) }()

	select {
	case cmd := <-runner.fired:
		if strings.Join(cmd, " ") != "loop run dep" {
			t.Errorf("daemon fired %v, want the due job's command", cmd)
		}
	case <-time.After(2 * time.Second):
		cancel()
		t.Fatal("daemon did not fire the due job in time")
	}
	cancel()
	select {
	case err := <-errc:
		if err != nil {
			t.Errorf("daemon returned %v, want nil on shutdown", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("daemon did not stop after context cancel")
	}
}

// forceDue backdates a job's next fire so the next tick runs it.
func forceDue(ctx context.Context, t *testing.T, dir, name string) {
	t.Helper()
	store, err := schedule.Open(ctx, dir)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer func() { _ = store.Close() }()
	if err := store.AdvanceNextFire(ctx, name, time.Now().UTC().Add(-time.Minute)); err != nil {
		t.Fatalf("force due: %v", err)
	}
}
