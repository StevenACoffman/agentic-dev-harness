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
	"github.com/StevenACoffman/agentic-dev-harness/cmd/evalcmd"
	"github.com/StevenACoffman/agentic-dev-harness/cmd/gate"
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
	"github.com/StevenACoffman/agentic-dev-harness/cmd/sleep"
	"github.com/StevenACoffman/agentic-dev-harness/cmd/status"
	"github.com/StevenACoffman/agentic-dev-harness/cmd/step"
	"github.com/StevenACoffman/agentic-dev-harness/cmd/toolcmd"
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
	version.New(r)
	gate.New(r)
	// register new commands here

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
	evalcmd.New(r)
	run.New(r)
	lessoncmd.New(r)
	metricscmd.New(r)
	sleep.New(r)
	loopcmd.New(r)
	workercmd.New(r)
	judgecmd.New(r)
	harnesscmd.New(r)
	if err := r.Command.Parse(args, ff.WithEnvVarPrefix("AGENTIC_DEV_HARNESS")); err != nil {
		_, _ = fmt.Fprintf(stderr, "\n%s\n", ffhelp.Command(r.Command))
		return fmt.Errorf("parse: %w", err)
	}

	if err := r.Command.Run(ctx); err != nil {
		// Don't print usage help for ErrNoExec (no subcommand given) or
		// ExitError (command already reported its own outcome).
		var exitErr root.ExitError
		if !errors.Is(err, ff.ErrNoExec) && !errors.As(err, &exitErr) {
			_, _ = fmt.Fprintf(stderr, "\n%s\n", ffhelp.Command(r.Command.GetSelected()))
		}
		return err
	}

	return nil
}
