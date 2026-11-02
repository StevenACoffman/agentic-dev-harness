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
	"strings"
)

// POSIX exit codes the shell reports when the command could not be run at all:
// 126 = found but not executable, 127 = not found. A caller distinguishes these
// (an uninstalled tool) from a command that ran and exited non-zero (a real result).
const (
	notExecutable = 126
	notFound      = 127
)

// Command is a repository-owned command in one of its two forms. Exactly one is set.
//
// Script is the original form, run as `sh -c <Script>`: it can use pipes and
// substitutions, which is also why `tool doctor` has to read it for hazards. Argv is
// executed directly — Argv[0] with Argv[1:] as its arguments, no shell — so there is
// nothing to substitute or pipe, and Argv[0] can be resolved with exec.LookPath before
// an arc starts.
type Command struct {
	Script string
	Argv   []string
}

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
func (r Runner) RunIO(
	ctx context.Context,
	command, dir string,
	stdout, stderr io.Writer,
) (exitCode int, ran bool) {
	return r.Exec(ctx, Command{Script: command}, dir, stdout, stderr)
}

// Exec runs c in dir with its streams wired to stdout and stderr (nil discards), under
// the same exitCode/ran contract as RunIO. An Argv command never passes through a shell.
func (Runner) Exec(
	ctx context.Context,
	c Command,
	dir string,
	stdout, stderr io.Writer,
) (exitCode int, ran bool) {
	// The command is repository-owned config (a tool-registry entry §13, a loop sensor
	// §15), not agent or critic input. That is not by itself a trust boundary: an arc's
	// worker can edit the registry files in the working tree, so the callers refuse a
	// registry that differs from HEAD (execconfig) before they reach here. What runs is
	// therefore what a human committed.
	var cmd *exec.Cmd
	if len(c.Argv) > 0 {
		name, args := c.Argv[0], c.Argv[1:]
		cmd = exec.CommandContext(ctx, name, args...) //nolint:gosec // committed repo config
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", c.Script) //nolint:gosec // committed repo config
	}
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

// String renders the command for logs and reports: the script as written, or the argv
// joined with spaces. It is a description, not something to execute.
func (c Command) String() string {
	if len(c.Argv) > 0 {
		return strings.Join(c.Argv, " ")
	}
	return c.Script
}
