package sleep

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/StevenACoffman/agentic-dev-harness/internal/schedule"
)

const (
	// adhDir is the per-repo state directory the schedule database lives in.
	adhDir = ".adh"
	// pollInterval caps how long the run daemon sleeps between ticks, so a job
	// added or removed out of band is noticed within it (there is no wake channel).
	pollInterval = time.Minute
)

// execRunner runs a scheduled job by re-invoking the adh binary with the job's
// command (e.g. `adh loop run dep-scan`). The command is a repository-owned
// schedule entry an operator authored with `sleep schedule add`, never model
// input, so there is no injection path from the model.
type execRunner struct{}

// repoDir is the repository root the schedule store lives under — the --repo
// global, or the current directory — so a daemon launched from elsewhere (e.g.
// launchd) still finds the repo's jobs.
func (cfg *Config) repoDir() string {
	if cfg.Repo != "" {
		return cfg.Repo
	}
	return "."
}

// scheduleDir is the directory holding the schedule database for this repo.
func (cfg *Config) scheduleDir() string {
	return filepath.Join(cfg.repoDir(), adhDir)
}

// Run execs the adh binary with the job's arguments. Child output is discarded in
// this first cut; captured run history is a documented follow-up.
func (execRunner) Run(ctx context.Context, command []string) error {
	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate adh binary: %w", err)
	}
	cmd := exec.CommandContext(ctx, self, command...) //nolint:gosec // repo-owned schedule command
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("run scheduled command: %w", err)
	}
	return nil
}

// schedule dispatches the `sleep schedule` verbs over the SQLite job store.
func (cfg *Config) schedule(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("sleep: schedule expects a verb: add, list, remove, tick, or run")
	}
	store, err := schedule.Open(ctx, cfg.scheduleDir())
	if err != nil {
		return fmt.Errorf("sleep: %w", err)
	}
	defer func() { _ = store.Close() }()

	switch args[0] {
	case "add":
		return cfg.scheduleAdd(ctx, store, args[1:])
	case "list":
		return cfg.scheduleList(ctx, store)
	case "remove":
		return cfg.scheduleRemove(ctx, store, args[1:])
	case "tick":
		return cfg.scheduleTick(ctx, store)
	case "run":
		return cfg.scheduleRun(ctx, store)
	default:
		return fmt.Errorf(
			"sleep: unknown schedule verb %q; want add, list, remove, tick, or run",
			args[0],
		)
	}
}

// scheduleRun is the blocking daemon: it ticks the due jobs, then sleeps until the
// earlier of the next deadline and the poll cap, and repeats until the context is
// canceled (SIGINT/SIGTERM, wired in main). It reports through the diagnostic
// stream so stdout stays clean for a long-runner. Run at most one daemon per store
// — there is no cross-process lock yet (the arc store shares that gap).
func (cfg *Config) scheduleRun(ctx context.Context, store *schedule.Store) error {
	cfg.Log.InfoContext(
		ctx,
		"schedule daemon started",
		"op",
		"sleep",
		"poll",
		pollInterval.String(),
	)
	for {
		now := time.Now().UTC()
		fired, err := schedule.Tick(ctx, store, now, cfg.runner)
		if err != nil {
			return cfg.daemonStop(ctx, err)
		}
		if len(fired) > 0 {
			cfg.Log.InfoContext(ctx, "schedule tick fired jobs", "op", "sleep", "count", len(fired))
		}
		next, err := store.SoonestDeadline(ctx, now)
		if err != nil {
			return cfg.daemonStop(ctx, err)
		}
		timer := time.NewTimer(schedule.NextSleep(next, now, pollInterval))
		select {
		case <-ctx.Done():
			timer.Stop()
			cfg.Log.InfoContext(ctx, "schedule daemon stopped", "op", "sleep")
			return nil
		case <-timer.C:
		}
	}
}

// daemonStop maps a store error to the daemon's exit. An error while the context
// is still live is a real failure that stops the daemon. A cancellation
// interrupting an in-flight query mid-tick is instead a graceful shutdown — a job
// may then re-run on the next start, at-least-once being the shutdown trade.
func (cfg *Config) daemonStop(ctx context.Context, err error) error {
	if ctx.Err() == nil {
		return fmt.Errorf("sleep: %w", err)
	}
	cfg.Log.InfoContext(ctx, "schedule daemon stopped", "op", "sleep")
	return nil
}

// scheduleAdd registers a cron job: `schedule add <name> <cron> <command...>`. The
// cron expression must be a single argument, so a multi-field crontab is quoted
// (`"0 3 * * *"`).
func (cfg *Config) scheduleAdd(ctx context.Context, store *schedule.Store, args []string) error {
	if len(args) < 3 {
		return errors.New(
			"sleep: schedule add <name> <cron> <command...> (quote a multi-field cron)",
		)
	}
	spec := schedule.JobSpec{Name: args[0], Cron: args[1], Command: args[2:]}
	job, err := store.Add(ctx, spec, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("sleep: %w", err)
	}
	if cfg.JSONL {
		return cfg.emitOK(map[string]any{
			"name": job.Name, "cron": job.Cron, "next_fire": job.NextFire.Format(time.RFC3339),
		})
	}
	_, _ = fmt.Fprintf(cfg.Stdout, "scheduled %q (%s), next fire %s\n",
		job.Name, job.Cron, job.NextFire.Format(time.RFC3339))
	return nil
}

// scheduleList prints every job with its cadence, next fire, and last outcome.
func (cfg *Config) scheduleList(ctx context.Context, store *schedule.Store) error {
	jobs, err := store.List(ctx)
	if err != nil {
		return fmt.Errorf("sleep: %w", err)
	}
	if cfg.JSONL {
		return cfg.emitOK(map[string]any{"jobs": jobs})
	}
	for i := range jobs {
		j := &jobs[i]
		_, _ = fmt.Fprintf(cfg.Stdout, "%s\t%s\tnext %s\tlast %s\n",
			j.Name, j.Cron, formatTime(j.NextFire), lastRun(j.LastRun, j.LastStatus))
	}
	return nil
}

// scheduleRemove deletes a named job.
func (cfg *Config) scheduleRemove(ctx context.Context, store *schedule.Store, args []string) error {
	if len(args) == 0 {
		return errors.New("sleep: schedule remove <name>")
	}
	if err := store.Remove(ctx, args[0]); err != nil {
		return fmt.Errorf("sleep: %w", err)
	}
	if cfg.JSONL {
		return cfg.emitOK(map[string]any{"removed": args[0]})
	}
	_, _ = fmt.Fprintf(cfg.Stdout, "removed %q\n", args[0])
	return nil
}

// scheduleTick fires every job due now through the runner, reporting each.
func (cfg *Config) scheduleTick(ctx context.Context, store *schedule.Store) error {
	fired, err := schedule.Tick(ctx, store, time.Now().UTC(), cfg.runner)
	if err != nil {
		return fmt.Errorf("sleep: %w", err)
	}
	if cfg.JSONL {
		return cfg.emitOK(map[string]any{"fired": len(fired), "jobs": fired})
	}
	_, _ = fmt.Fprintf(cfg.Stdout, "fired %d job(s)\n", len(fired))
	for i := range fired {
		_, _ = fmt.Fprintf(cfg.Stdout, "  %s\t%s\n", fired[i].Name, fired[i].Status)
	}
	return nil
}

// emitOK wraps the root success envelope with the sleep error prefix.
func (cfg *Config) emitOK(data any) error {
	if err := cfg.EmitOK(data); err != nil {
		return fmt.Errorf("sleep: %w", err)
	}
	return nil
}

// formatTime renders a schedule time, or a dash when it never fires again.
func formatTime(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Format(time.RFC3339)
}

// lastRun renders a job's last-run summary, or a dash when it has never run.
func lastRun(at time.Time, status schedule.RunStatus) string {
	if at.IsZero() {
		return "-"
	}
	return string(status) + "@" + at.Format(time.RFC3339)
}
