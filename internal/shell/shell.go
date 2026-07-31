// Package shell runs a repository-owned command through the system shell. It is
// the one effectful edge shared by the checks the Evaluation stage adjudicates
// (§10, §19.2) and the sensors maintenance loops watch (§15): both run a
// maintainer-authored `sh -c <command>` and branch on whether it ran and how it
// exited. Centralizing it keeps the single gosec justification in one place — the
// command is repository config, never model or critic input.
package shell

import (
	"context"
	"errors"
	"io"
	"os/exec"
)

// POSIX exit codes the shell reports when the command could not be run at all:
// 126 = found but not executable, 127 = not found. A caller distinguishes these
// (an uninstalled tool) from a command that ran and exited non-zero (a real result).
const (
	notExecutable = 126
	notFound      = 127
)

// Runner runs a command as `sh -c <command>`. The zero value is ready to use; a
// caller holds it behind its own point-of-use interface so tests inject a fake.
type Runner struct{}

// NotRun reports whether a (exitCode, ran) result means the command could not be
// run at all: the process never started, or the shell reported not-found (127) or
// not-executable (126). Callers treat this as "tool unavailable" — pointing the
// worker at a repair hint — distinct from a command that ran and exited non-zero.
func NotRun(exitCode int, ran bool) bool {
	return !ran || exitCode == notExecutable || exitCode == notFound
}

// Run executes command via the shell in dir (empty dir means the current
// directory), discarding its output. ran is false only when the process could not
// start — an absent shell, or a canceled context; a command that started and
// exited non-zero still ran, and its status is exitCode. exitCode is 0 on a clean
// exit and -1 when the command never ran. There is no error return: a non-zero
// exit is a result to branch on (a failed check, a sensed departure), not a
// failure of the caller.
func (r Runner) Run(ctx context.Context, command, dir string) (exitCode int, ran bool) {
	return r.RunIO(ctx, command, dir, nil, nil)
}

// RunIO is Run with the child's stdout and stderr wired to the given writers, so a
// caller that invokes a declared tool (§13, `adh tool run`) can surface or capture
// its output. A nil writer discards that stream (this is how Run works). The
// exitCode/ran contract is identical to Run.
func (Runner) RunIO(
	ctx context.Context,
	command, dir string,
	stdout, stderr io.Writer,
) (exitCode int, ran bool) {
	// The command is repository-owned config (a tool-registry entry §13, a loop
	// sensor §15), authored by the maintainer — not agent or critic input — so
	// there is no model-driven injection path.
	cmd := exec.CommandContext(ctx, "sh", "-c", command) //nolint:gosec // repo-owned config
	cmd.Dir = dir
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	err := cmd.Run()
	if err == nil {
		return 0, true
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode(), true // ran and reported a non-zero status
	}
	return -1, false // could not start the command
}
