package schedule_test

import (
	"errors"
	"testing"
	"time"

	"github.com/StevenACoffman/agentic-dev-harness/internal/schedule"
)

func TestParseCron(t *testing.T) {
	t.Parallel()
	valid := []string{"@daily", "@hourly", "@every 30m", "0 3 * * *", "*/15 9-17 * * 1-5"}
	for _, expr := range valid {
		if _, err := schedule.ParseCron(expr); err != nil {
			t.Errorf("ParseCron(%q) = %v, want ok", expr, err)
		}
	}
	invalid := []string{"", "not-a-cron", "@every", "@every 0s", "@every -5m", "60 * * * *"}
	for _, expr := range invalid {
		_, err := schedule.ParseCron(expr)
		if !errors.Is(err, schedule.ErrInvalidCron) {
			t.Errorf("ParseCron(%q) = %v, want ErrInvalidCron", expr, err)
		}
	}
}

func TestNextFire(t *testing.T) {
	t.Parallel()
	// A fixed anchor keeps the expectation deterministic.
	now := time.Date(2026, 7, 29, 10, 0, 0, 0, time.UTC)
	// @daily fires at the next midnight UTC.
	got := schedule.NextFire("@daily", now)
	want := time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("NextFire(@daily) = %s, want %s", got, want)
	}
	// @every 30m fires 30 minutes on.
	if got := schedule.NextFire("@every 30m", now); !got.Equal(now.Add(30 * time.Minute)) {
		t.Errorf("NextFire(@every 30m) = %s, want %s", got, now.Add(30*time.Minute))
	}
	// A bad expression never fires (zero time), not a panic.
	if got := schedule.NextFire("bogus", now); !got.IsZero() {
		t.Errorf("NextFire(bogus) = %s, want zero", got)
	}
}

func TestJobSpecValidate(t *testing.T) {
	t.Parallel()
	ok := schedule.JobSpec{Name: "nightly", Cron: "@daily", Command: []string{"sleep", "run"}}
	if err := ok.Validate(); err != nil {
		t.Errorf("Validate(valid) = %v, want ok", err)
	}
	bad := []schedule.JobSpec{
		{Cron: "@daily", Command: []string{"sleep"}},          // no name
		{Name: "x", Cron: "@daily"},                           // no command
		{Name: "x", Cron: "@daily", Command: []string{""}},    // empty command token
		{Name: "x", Cron: "nope", Command: []string{"sleep"}}, // bad cron
	}
	for i, s := range bad {
		if err := s.Validate(); err == nil {
			t.Errorf("Validate(bad[%d]) = nil, want error", i)
		}
	}
}
