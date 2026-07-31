package schedule_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/StevenACoffman/agentic-dev-harness/internal/schedule"
)

// fakeRunner records the commands it was asked to run and fails those whose first
// token is in failCmds, so a tick's success/failure handling is exercised without
// spawning a process.
type fakeRunner struct {
	ran      [][]string
	failCmds map[string]bool
}

func (f *fakeRunner) Run(_ context.Context, command []string) error {
	f.ran = append(f.ran, command)
	if len(command) > 0 && f.failCmds[command[0]] {
		return errors.New("boom")
	}
	return nil
}

func TestTickFiresDueAndAdvances(t *testing.T) {
	t.Parallel()
	st := openStore(t)
	ctx := context.Background()
	now := time.Date(2026, 7, 29, 10, 0, 0, 0, time.UTC)

	// due: @every 30m becomes due an hour on; not-due: @daily fires at midnight.
	if _, err := st.Add(ctx, schedule.JobSpec{
		Name: "due", Cron: "@every 30m", Command: []string{"loop", "run", "x"},
	}, now); err != nil {
		t.Fatalf("Add due: %v", err)
	}
	if _, err := st.Add(ctx, schedule.JobSpec{
		Name: "later", Cron: "@daily", Command: []string{"sleep", "run"},
	}, now); err != nil {
		t.Fatalf("Add later: %v", err)
	}

	at := now.Add(time.Hour)
	runner := &fakeRunner{failCmds: map[string]bool{}}
	fired, err := schedule.Tick(ctx, st, at, runner)
	if err != nil {
		t.Fatalf("Tick: %v", err)
	}
	if len(fired) != 1 || fired[0].Name != "due" || fired[0].Status != schedule.RunOK {
		t.Fatalf("fired = %+v, want only 'due' with ok", fired)
	}
	if len(runner.ran) != 1 || runner.ran[0][0] != "loop" {
		t.Errorf("ran = %v, want the due job's command", runner.ran)
	}
	// The due job's next fire advanced past 'at'; it is no longer due at 'at'.
	stillDue, _ := st.Due(ctx, at)
	if len(stillDue) != 0 {
		t.Errorf("Due after tick = %d, want 0 (next fire advanced)", len(stillDue))
	}
}

func TestNextSleep(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 7, 29, 10, 0, 0, 0, time.UTC)
	poll := time.Minute
	tests := []struct {
		name string
		next time.Time
		want time.Duration
	}{
		{"nothing scheduled sleeps the poll cap", time.Time{}, poll},
		{"far deadline is capped at poll", now.Add(time.Hour), poll},
		{"near deadline wins over poll", now.Add(10 * time.Second), 10 * time.Second},
		{"past-due is floored, not negative", now.Add(-time.Hour), 100 * time.Millisecond},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := schedule.NextSleep(tt.next, now, poll); got != tt.want {
				t.Errorf("NextSleep = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestTickContinuesOnFailure(t *testing.T) {
	t.Parallel()
	st := openStore(t)
	ctx := context.Background()
	now := time.Date(2026, 7, 29, 10, 0, 0, 0, time.UTC)
	for _, n := range []string{"a", "b"} {
		if _, err := st.Add(ctx, schedule.JobSpec{
			Name: n, Cron: "@every 1m", Command: []string{n},
		}, now); err != nil {
			t.Fatalf("Add %s: %v", n, err)
		}
	}
	at := now.Add(2 * time.Minute)
	runner := &fakeRunner{failCmds: map[string]bool{"a": true}}
	fired, err := schedule.Tick(ctx, st, at, runner)
	if err != nil {
		t.Fatalf("Tick: %v", err)
	}
	// Both fired; a failed even though it errored, and b still ran.
	if len(fired) != 2 || len(runner.ran) != 2 {
		t.Fatalf("fired=%+v ran=%v, want both jobs run", fired, runner.ran)
	}
	got := map[string]schedule.RunStatus{}
	for _, f := range fired {
		got[f.Name] = f.Status
	}
	if got["a"] != schedule.RunFail || got["b"] != schedule.RunOK {
		t.Errorf("statuses = %v, want a:fail b:ok", got)
	}
}
