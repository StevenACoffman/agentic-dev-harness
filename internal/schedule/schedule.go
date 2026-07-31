// Package schedule persists cron jobs that fire an adh command on a cadence and
// fires the ones that are due (SPEC-ADDITIONS §15, §18). A job names a command
// (e.g. `sleep run`, `loop run dep-scan`) and a cron expression; `tick` runs every
// due job, records its outcome, and advances its next fire time. The store is
// SQLite so a schedule survives across the stateless-per-invocation CLI; time
// enters as a parameter so the disposition is deterministic and testable.
package schedule

import (
	"errors"
	"time"
)

// RunStatus values (see RunStatus).
const (
	RunOK   RunStatus = "ok"   // the command exited zero
	RunFail RunStatus = "fail" // the command failed to start or exited non-zero
)

// Sentinel errors are part of the package contract; callers branch with errors.Is
// and command shells map them to exit codes.
var (
	// ErrNotFound means a named job does not exist.
	ErrNotFound = errors.New("schedule: job not found")
	// ErrConflict means a job with that name already exists.
	ErrConflict = errors.New("schedule: job already exists")
	// ErrInvalidCron means a cron expression was not accepted.
	ErrInvalidCron = errors.New("schedule: invalid cron expression")
	// ErrInvalidSpec means a job spec violates a creation invariant.
	ErrInvalidSpec = errors.New("schedule: invalid job spec")
)

// RunStatus is the outcome of a job's last run.
type RunStatus string

// Job is a stored schedule entry: a named command fired on a cron cadence, with
// the denormalized outcome of its last run.
type Job struct {
	Name       string    `json:"name"`
	Cron       string    `json:"cron"`
	Command    []string  `json:"command"`
	NextFire   time.Time `json:"next_fire,omitzero"`
	LastRun    time.Time `json:"last_run,omitzero"`
	LastStatus RunStatus `json:"last_status,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// JobSpec is the user-supplied subset used to create a job; the store fills in the
// timestamps and the first NextFire.
type JobSpec struct {
	Name    string
	Cron    string
	Command []string
}
