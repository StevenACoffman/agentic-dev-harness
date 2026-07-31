package schedule

import (
	"fmt"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
)

// parser is the accepted cron syntax: standard 5-field crontab plus the
// @descriptor and @every shortcuts. Built per call — cheap, and a package-level
// var would be shared mutable state (gochecknoglobals).
func parser() cron.Parser {
	return cron.NewParser(
		cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor,
	)
}

// ParseCron validates a cron expression and returns a schedule that computes
// next-fire times. Parser errors wrap ErrInvalidCron; an @every duration must be
// positive (robfig rounds sub-second values up to a second, so `@every 0s` is
// pre-checked to stay a visible user error).
func ParseCron(expr string) (cron.Schedule, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return nil, fmt.Errorf("%w: empty expression", ErrInvalidCron)
	}
	if rest, ok := strings.CutPrefix(expr, "@every "); ok {
		d, err := time.ParseDuration(strings.TrimSpace(rest))
		if err != nil {
			return nil, fmt.Errorf("%w: @every duration: %w", ErrInvalidCron, err)
		}
		if d <= 0 {
			return nil, fmt.Errorf(
				"%w: @every duration must be positive: got %v",
				ErrInvalidCron,
				d,
			)
		}
	}
	sched, err := parser().Parse(expr)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidCron, err)
	}
	return sched, nil
}

// NextFire returns the next time the cron expression fires after now, or the zero
// time when the expression does not parse (a stored job with a bad expression
// simply never fires again rather than erroring the whole tick).
func NextFire(expr string, now time.Time) time.Time {
	sched, err := ParseCron(expr)
	if err != nil {
		return time.Time{}
	}
	return sched.Next(now)
}

// Validate enforces the JobSpec invariants required before insert.
func (s JobSpec) Validate() error {
	if s.Name == "" {
		return fmt.Errorf("%w: name required", ErrInvalidSpec)
	}
	if len(s.Command) == 0 || s.Command[0] == "" {
		return fmt.Errorf("%w: command required", ErrInvalidSpec)
	}
	if _, err := ParseCron(s.Cron); err != nil {
		return err
	}
	return nil
}
