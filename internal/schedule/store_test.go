package schedule_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/StevenACoffman/agentic-dev-harness/internal/schedule"
)

// openStore opens a temp-dir store closed at test end.
func openStore(t *testing.T) *schedule.Store {
	t.Helper()
	st, err := schedule.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func TestStoreAddListRemove(t *testing.T) {
	t.Parallel()
	st := openStore(t)
	ctx := context.Background()
	now := time.Date(2026, 7, 29, 10, 0, 0, 0, time.UTC)

	job, err := st.Add(ctx, schedule.JobSpec{
		Name: "nightly", Cron: "@daily", Command: []string{"sleep", "run"},
	}, now)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if job.NextFire.IsZero() || !job.NextFire.After(now) {
		t.Errorf("new job NextFire = %s, want a future time", job.NextFire)
	}

	jobs, err := st.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(jobs) != 1 || jobs[0].Name != "nightly" || len(jobs[0].Command) != 2 {
		t.Fatalf("List = %+v, want the one job with its command", jobs)
	}

	// A duplicate name conflicts.
	if _, err := st.Add(ctx, schedule.JobSpec{
		Name: "nightly", Cron: "@daily", Command: []string{"sleep", "run"},
	}, now); !errors.Is(err, schedule.ErrConflict) {
		t.Errorf("duplicate Add = %v, want ErrConflict", err)
	}

	if err := st.Remove(ctx, "nightly"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if err := st.Remove(ctx, "nightly"); !errors.Is(err, schedule.ErrNotFound) {
		t.Errorf("Remove(absent) = %v, want ErrNotFound", err)
	}
}

func TestStoreDueAndAdvance(t *testing.T) {
	t.Parallel()
	st := openStore(t)
	ctx := context.Background()
	now := time.Date(2026, 7, 29, 10, 0, 0, 0, time.UTC)

	// Fires every 30 minutes → not due at creation.
	if _, err := st.Add(ctx, schedule.JobSpec{
		Name: "every30", Cron: "@every 30m", Command: []string{"loop", "run", "x"},
	}, now); err != nil {
		t.Fatalf("Add: %v", err)
	}
	due, err := st.Due(ctx, now)
	if err != nil {
		t.Fatalf("Due: %v", err)
	}
	if len(due) != 0 {
		t.Errorf("Due(now) = %d, want 0 (next fire is 30m out)", len(due))
	}
	// An hour later it is due.
	if due, err = st.Due(ctx, now.Add(time.Hour)); err != nil || len(due) != 1 {
		t.Fatalf("Due(now+1h) = (%v, %d), want (nil, 1)", err, len(due))
	}

	// SoonestDeadline is the next fire after now.
	soon, err := st.SoonestDeadline(ctx, now)
	if err != nil || !soon.Equal(now.Add(30*time.Minute)) {
		t.Errorf("SoonestDeadline = (%s, %v), want %s", soon, err, now.Add(30*time.Minute))
	}

	// Advancing past all fires clears due-ness; MarkRan records the outcome.
	if err := st.AdvanceNextFire(ctx, "every30", time.Time{}); err != nil {
		t.Fatalf("AdvanceNextFire: %v", err)
	}
	if err := st.MarkRan(ctx, "every30", schedule.RunOK, now); err != nil {
		t.Fatalf("MarkRan: %v", err)
	}
	if due, err = st.Due(ctx, now.Add(24*time.Hour)); err != nil || len(due) != 0 {
		t.Errorf("Due after zero-advance = (%v, %d), want (nil, 0)", err, len(due))
	}
	jobs, _ := st.List(ctx)
	if jobs[0].LastStatus != schedule.RunOK || jobs[0].LastRun.IsZero() {
		t.Errorf("last run = (%s, %s), want (ok, non-zero)", jobs[0].LastStatus, jobs[0].LastRun)
	}
}
