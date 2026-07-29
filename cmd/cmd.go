// Package cmd is the dispatcher for the agentic-dev-harness CLI.
// It registers all commands and routes incoming arguments
// to the matching command implementation.
package cmd

// climax:name agentic-dev-harness
// climax:root-pkg root
// climax:env-prefix AGENTIC_DEV_HARNESS

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/peterbourgon/ff/v4"
	"github.com/peterbourgon/ff/v4/ffhelp"

	"github.com/StevenACoffman/agentic-dev-harness/cmd/approve"
	"github.com/StevenACoffman/agentic-dev-harness/cmd/arc"
	"github.com/StevenACoffman/agentic-dev-harness/cmd/autonomy"
	"github.com/StevenACoffman/agentic-dev-harness/cmd/closecmd"
	"github.com/StevenACoffman/agentic-dev-harness/cmd/contextcmd"
	"github.com/StevenACoffman/agentic-dev-harness/cmd/device"
	"github.com/StevenACoffman/agentic-dev-harness/cmd/docscmd"
	"github.com/StevenACoffman/agentic-dev-harness/cmd/evalcmd"
	"github.com/StevenACoffman/agentic-dev-harness/cmd/failurescmd"
	"github.com/StevenACoffman/agentic-dev-harness/cmd/gatecmd"
	"github.com/StevenACoffman/agentic-dev-harness/cmd/harnesscmd"
	"github.com/StevenACoffman/agentic-dev-harness/cmd/initcmd"
	"github.com/StevenACoffman/agentic-dev-harness/cmd/judgecmd"
	"github.com/StevenACoffman/agentic-dev-harness/cmd/lessoncmd"
	"github.com/StevenACoffman/agentic-dev-harness/cmd/loopcmd"
	"github.com/StevenACoffman/agentic-dev-harness/cmd/metricscmd"
	"github.com/StevenACoffman/agentic-dev-harness/cmd/oracle"
	"github.com/StevenACoffman/agentic-dev-harness/cmd/proof"
	"github.com/StevenACoffman/agentic-dev-harness/cmd/reject"
	"github.com/StevenACoffman/agentic-dev-harness/cmd/root"
	"github.com/StevenACoffman/agentic-dev-harness/cmd/run"
	"github.com/StevenACoffman/agentic-dev-harness/cmd/selfeval"
	"github.com/StevenACoffman/agentic-dev-harness/cmd/sleep"
	"github.com/StevenACoffman/agentic-dev-harness/cmd/stagecmd"
	"github.com/StevenACoffman/agentic-dev-harness/cmd/status"
	"github.com/StevenACoffman/agentic-dev-harness/cmd/step"
	"github.com/StevenACoffman/agentic-dev-harness/cmd/toolcmd"
	"github.com/StevenACoffman/agentic-dev-harness/cmd/vcscmd"
	"github.com/StevenACoffman/agentic-dev-harness/cmd/version"
	"github.com/StevenACoffman/agentic-dev-harness/cmd/workercmd"
)

// Run parses args and dispatches to the matching command.
// args must not include the executable name (pass os.Args[1:]).
//
// Every flag can be set via a AGENTIC_DEV_HARNESS_-prefixed environment variable.
// The mapping rule is: prepend AGENTIC_DEV_HARNESS_, uppercase, replace dashes with
// underscores.
//
// Flags supplied on the command line always take precedence over env vars.
func Run(
	ctx context.Context,
	args []string,
	getenv func(string) string,
	stdin io.Reader,
	stdout, stderr io.Writer,
) error {
	r := root.New(getenv, stdin, stdout, stderr)
	register(r)
	if err := r.Command.Parse(args, ff.WithEnvVarPrefix("AGENTIC_DEV_HARNESS")); err != nil {
		_, _ = fmt.Fprintf(stderr, "\n%s\n", ffhelp.Command(r.Command))
		return fmt.Errorf("parse: %w", err)
	}
	// --quiet suppresses non-error output for every command at once: they all write
	// through the embedded *root.Config, so redirecting stdout here is enough.
	// Errors still go to stderr.
	if r.Quiet {
		r.Stdout = io.Discard
	}
	// Build the diagnostic logger now that the flags are parsed: --verbose/--quiet
	// set the level, --jsonl selects the JSON handler. It writes to stderr, the
	// diagnostic stream kept separate from the stdout data plane (SPEC §8).
	r.Log = root.NewLogger(stderr, r.JSONL, root.LogLevel(r.Verbose, r.Quiet))

	if runErr := runSelected(ctx, r, stderr); runErr != nil {
		return runErr
	}
	return nil
}

// runSelected runs the parsed command and translates its error: an ExitError,
// ErrNoExec, or ErrHelp passes through; under --jsonl any other error becomes one
// structured error outcome (no usage banner) carried by an ExitError; otherwise
// the usage banner is printed and the error returned.
func runSelected(ctx context.Context, r *root.Config, stderr io.Writer) error {
	if err := r.Command.Run(ctx); err != nil {
		var exitErr root.ExitError
		switch {
		case errors.As(err, &exitErr), errors.Is(err, ff.ErrNoExec), errors.Is(err, ff.ErrHelp):
			// Already reported (ExitError) or not a failure (no subcommand / help).
			return err
		case r.JSONL:
			// Machine consumers get one structured error outcome instead of the
			// usage banner; the ExitError carries the code so main stays quiet.
			code := root.CodeForError(err)
			_ = r.EmitError(code, root.ReasonForError(err), err.Error())
			return root.ExitError(code)
		default:
			_, _ = fmt.Fprintf(stderr, "\n%s\n", ffhelp.Command(r.Command.GetSelected()))
			return err
		}
	}

	return nil
}

// register wires every subcommand onto the root config. main is the only place
// concrete command packages are composed (go-advice §1); this keeps Run focused on
// parse-and-dispatch.
func register(r *root.Config) {
	version.New(r)
	vcscmd.New(r)
	gatecmd.New(r)
	arc.New(r)
	status.New(r)
	initcmd.New(r)
	autonomy.New(r)
	approve.New(r)
	reject.New(r)
	closecmd.New(r)
	oracle.New(r)
	proof.New(r)
	device.New(r)
	contextcmd.New(r)
	toolcmd.New(r)
	step.New(r)
	stagecmd.New(r)
	evalcmd.New(r)
	run.New(r)
	lessoncmd.New(r)
	failurescmd.New(r)
	metricscmd.New(r)
	selfeval.New(r)
	sleep.New(r)
	loopcmd.New(r)
	workercmd.New(r)
	judgecmd.New(r)
	harnesscmd.New(r)
	docscmd.New(r)
}
