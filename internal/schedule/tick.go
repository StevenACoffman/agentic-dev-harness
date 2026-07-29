package schedule

import (
	"context"
	"fmt"
	"time"
)

// minSleep floors a daemon's sleep so a past-due or near deadline cannot spin the
// loop. It is small enough not to delay a genuinely-due job.
const minSleep = 100 * time.Millisecond

// Runner runs one job's command and reports whether it succeeded. A nil error is
// success (the command exited zero); any error — a non-zero exit or a failure to
// start — marks the run failed. It is the point-of-use seam: the sleep command
// supplies a runner that execs the adh binary; a test injects a fake.
type Runner interface {
	Run(ctx context.Context, command []string) error
}

// Fired records one job the tick ran and its outcome.
type Fired struct {
	Name   string    `json:"name"`
	Status RunStatus `json:"status"`
}

// Tick runs every job due at now: it executes the command, records the outcome,
// and advances the job's next fire to the next cron instant after now. One job's
// failure marks it failed and does not stop the others — the tick reports every
// job it fired. A store write failure (recording the outcome) is returned, since
// it means the schedule state is no longer trustworthy.
func Tick(ctx context.Context, store *Store, now time.Time, runner Runner) ([]Fired, error) {
	due, err := store.Due(ctx, now)
	if err != nil {
		return nil, err
	}
	fired := make([]Fired, 0, len(due))
	for i := range due {
		job := due[i]
		status := RunOK
		if runErr := runner.Run(ctx, job.Command); runErr != nil {
			status = RunFail
		}
		if err := store.MarkRan(ctx, job.Name, status, now); err != nil {
			return fired, err
		}
		if err := store.AdvanceNextFire(ctx, job.Name, NextFire(job.Cron, now)); err != nil {
			return fired, fmt.Errorf("schedule: advance %q: %w", job.Name, err)
		}
		fired = append(fired, Fired{Name: job.Name, Status: status})
	}
	return fired, nil
}

// NextSleep is how long a daemon should sleep after a tick: until the earlier of
// the next deadline and the poll cap, floored above zero. The poll cap bounds how
// long an out-of-band add or remove goes unnoticed (there is no wake channel); a
// zero next (nothing scheduled) sleeps the full cap. It is pure so the loop's
// timing is tested without a clock.
func NextSleep(next, now time.Time, poll time.Duration) time.Duration {
	sleep := poll
	if !next.IsZero() {
		if d := next.Sub(now); d < sleep {
			sleep = d
		}
	}
	if sleep < minSleep {
		return minSleep
	}
	return sleep
}
