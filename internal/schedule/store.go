package schedule

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"time"

	// Registers the cgo-free "sqlite3" database/sql driver (pure Go via wazero).
	_ "github.com/ncruces/go-sqlite3/driver"
)

// DefaultDBFile is the schedule database, kept beside the rest of the .adh state.
const DefaultDBFile = "schedule.db"

// schema is the jobs table. v1 is cron-only and always-enabled; a job carries the
// denormalized outcome of its last run. Run history, one-shot jobs, and enable/
// disable are deferred (see the package doc), so there is no runs table yet.
const schema = `
CREATE TABLE IF NOT EXISTS jobs (
    name         TEXT    PRIMARY KEY,
    cron_expr    TEXT    NOT NULL,
    command_json TEXT    NOT NULL,
    next_fire    INTEGER NOT NULL DEFAULT 0,
    last_run     INTEGER NOT NULL DEFAULT 0,
    last_status  TEXT    NOT NULL DEFAULT '',
    created_at   INTEGER NOT NULL,
    updated_at   INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS jobs_next_fire ON jobs(next_fire);`

// jobCols is the canonical column order every SELECT scanned by scanJob must use.
const jobCols = `name, cron_expr, command_json, next_fire, last_run, last_status, created_at, updated_at`

// Store persists schedule jobs in SQLite. It is the imperative shell; the cron
// disposition (ParseCron/NextFire) is the pure core the caller supplies results
// from. Concurrent callers against one database are serialized by SQLite's WAL and
// busy timeout.
type Store struct {
	db *sql.DB
}

// scanner is the shared shape of *sql.Row and *sql.Rows that scanJob reads from.
type scanner interface {
	Scan(dest ...any) error
}

// Open returns a Store backed by dir/schedule.db, creating dir and applying the
// schema. ctx scopes the schema apply, not the Store's lifetime; close with Close.
func Open(ctx context.Context, dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("schedule: mkdir %s: %w", dir, err)
	}
	path := filepath.Join(dir, DefaultDBFile)
	// busy_timeout first (pragma order matters), then WAL for concurrent readers.
	dsn := "file:" + url.PathEscape(path) +
		"?_pragma=busy_timeout(5000)&_pragma=journal_mode(wal)&_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("schedule: open %s: %w", path, err)
	}
	if _, err := db.ExecContext(ctx, schema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("schedule: apply schema: %w", err)
	}
	return &Store{db: db}, nil
}

// Close releases the database handle.
func (s *Store) Close() error {
	if err := s.db.Close(); err != nil {
		return fmt.Errorf("schedule: close: %w", err)
	}
	return nil
}

// Add inserts a new job from spec, computing its first fire time from now. A name
// already in use is ErrConflict; an invalid spec is ErrInvalidSpec/ErrInvalidCron.
func (s *Store) Add(ctx context.Context, spec JobSpec, now time.Time) (Job, error) {
	if err := spec.Validate(); err != nil {
		return Job{}, err
	}
	cmdJSON, err := json.Marshal(spec.Command)
	if err != nil {
		return Job{}, fmt.Errorf("schedule: marshal command: %w", err)
	}
	var nextNano int64
	if t := NextFire(spec.Cron, now); !t.IsZero() {
		nextNano = t.UnixNano()
	}
	const q = `INSERT INTO jobs (name, cron_expr, command_json, next_fire, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?) ON CONFLICT(name) DO NOTHING`
	res, err := s.db.ExecContext(ctx, q,
		spec.Name, spec.Cron, string(cmdJSON), nextNano, now.UnixNano(), now.UnixNano())
	if err != nil {
		return Job{}, fmt.Errorf("schedule: insert job: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return Job{}, fmt.Errorf("%w: %q", ErrConflict, spec.Name)
	}
	return s.job(ctx, spec.Name)
}

// List returns every job ordered by name.
func (s *Store) List(ctx context.Context) ([]Job, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+jobCols+` FROM jobs ORDER BY name ASC`)
	if err != nil {
		return nil, fmt.Errorf("schedule: list: %w", err)
	}
	defer func() { _ = rows.Close() }()
	jobs := make([]Job, 0)
	for rows.Next() {
		job, scanErr := scanJob(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		jobs = append(jobs, job)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("schedule: list rows: %w", err)
	}
	return jobs, nil
}

// Remove deletes the named job; a name that does not exist is ErrNotFound.
func (s *Store) Remove(ctx context.Context, name string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM jobs WHERE name = ?`, name)
	if err != nil {
		return fmt.Errorf("schedule: remove: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("%w: %q", ErrNotFound, name)
	}
	return nil
}

// Due returns the jobs whose next fire is at or before now, earliest first.
func (s *Store) Due(ctx context.Context, now time.Time) ([]Job, error) {
	const q = `SELECT ` + jobCols + ` FROM jobs
		WHERE next_fire > 0 AND next_fire <= ? ORDER BY next_fire ASC`
	rows, err := s.db.QueryContext(ctx, q, now.UnixNano())
	if err != nil {
		return nil, fmt.Errorf("schedule: due: %w", err)
	}
	defer func() { _ = rows.Close() }()
	jobs := make([]Job, 0)
	for rows.Next() {
		job, scanErr := scanJob(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		jobs = append(jobs, job)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("schedule: due rows: %w", err)
	}
	return jobs, nil
}

// SoonestDeadline returns the earliest future fire time after now, or the zero
// time when nothing is scheduled. It is the seam a future blocking daemon sleeps
// on; the tick command does not use it.
func (s *Store) SoonestDeadline(ctx context.Context, now time.Time) (time.Time, error) {
	const q = `SELECT next_fire FROM jobs WHERE next_fire > ? ORDER BY next_fire ASC LIMIT 1`
	var nano int64
	err := s.db.QueryRowContext(ctx, q, now.UnixNano()).Scan(&nano)
	if errors.Is(err, sql.ErrNoRows) {
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("schedule: soonest deadline: %w", err)
	}
	return time.Unix(0, nano).UTC(), nil
}

// AdvanceNextFire sets the job's next fire time; a zero next means it never fires
// again. A name that does not exist is ErrNotFound.
func (s *Store) AdvanceNextFire(ctx context.Context, name string, next time.Time) error {
	var nano int64
	if !next.IsZero() {
		nano = next.UnixNano()
	}
	res, err := s.db.ExecContext(ctx, `UPDATE jobs SET next_fire = ? WHERE name = ?`, nano, name)
	if err != nil {
		return fmt.Errorf("schedule: advance next_fire: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("%w: %q", ErrNotFound, name)
	}
	return nil
}

// MarkRan records the outcome of a job's last run.
func (s *Store) MarkRan(ctx context.Context, name string, status RunStatus, at time.Time) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE jobs SET last_run = ?, last_status = ?, updated_at = ? WHERE name = ?`,
		at.UnixNano(), string(status), at.UnixNano(), name)
	if err != nil {
		return fmt.Errorf("schedule: mark ran: %w", err)
	}
	return nil
}

// job reads one job by name.
func (s *Store) job(ctx context.Context, name string) (Job, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+jobCols+` FROM jobs WHERE name = ?`, name)
	job, err := scanJob(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Job{}, fmt.Errorf("%w: %q", ErrNotFound, name)
	}
	if err != nil {
		return Job{}, err
	}
	return job, nil
}

// scanJob reads a job row in jobCols order.
func scanJob(sc scanner) (Job, error) {
	var (
		job                      Job
		cmdJSON, lastStatus      string
		nextNano, lastNano       int64
		createdNano, updatedNano int64
	)
	if err := sc.Scan(&job.Name, &job.Cron, &cmdJSON, &nextNano,
		&lastNano, &lastStatus, &createdNano, &updatedNano); err != nil {
		return Job{}, fmt.Errorf("schedule: scan job: %w", err)
	}
	if err := json.Unmarshal([]byte(cmdJSON), &job.Command); err != nil {
		return Job{}, fmt.Errorf("schedule: decode command: %w", err)
	}
	job.LastStatus = RunStatus(lastStatus)
	if nextNano > 0 {
		job.NextFire = time.Unix(0, nextNano).UTC()
	}
	if lastNano > 0 {
		job.LastRun = time.Unix(0, lastNano).UTC()
	}
	job.CreatedAt = time.Unix(0, createdNano).UTC()
	job.UpdatedAt = time.Unix(0, updatedNano).UTC()
	return job, nil
}
