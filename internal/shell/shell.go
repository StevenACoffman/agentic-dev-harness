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
	"os/exec"
)

// Runner runs a command as `sh -c <command>`. The zero value is ready to use; a
// caller holds it behind its own point-of-use interface so tests inject a fake.
type Runner struct{}

// Run executes command via the shell in dir (empty dir means the current
// directory). ran is false only when the process could not start — an absent
// shell, or a canceled context; a command that started and exited non-zero still
// ran, and its status is exitCode. exitCode is 0 on a clean exit and -1 when the
// command never ran. There is no error return: a non-zero exit is a result to
// branch on (a failed check, a sensed departure), not a failure of the caller.
func (Runner) Run(ctx context.Context, command, dir string) (exitCode int, ran bool) {
	// The command is repository-owned config (a tool-registry entry §13, a loop
	// sensor §15), authored by the maintainer — not agent or critic input — so
	// there is no model-driven injection path.
	cmd := exec.CommandContext(ctx, "sh", "-c", command) //nolint:gosec // repo-owned config
	cmd.Dir = dir
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
