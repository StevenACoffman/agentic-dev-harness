// Package cmd is the dispatcher for the agentic-dev-harness CLI.
// It registers all commands and routes incoming arguments
// to the matching command implementation.
package cmd

// climax:name agentic-dev-harness
// climax:root-pkg root
// climax:features jsonl
// climax:env-prefix AGENTIC_DEV_HARNESS

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/peterbourgon/ff/v4"
	"github.com/peterbourgon/ff/v4/ffhelp"

	"github.com/StevenACoffman/agentic-dev-harness/cmd/approve"
	"github.com/StevenACoffman/agentic-dev-harness/cmd/arc"
	"github.com/StevenACoffman/agentic-dev-harness/cmd/autonomy"
	"github.com/StevenACoffman/agentic-dev-harness/cmd/closecmd"
	"github.com/StevenACoffman/agentic-dev-harness/cmd/contextcmd"
	"github.com/StevenACoffman/agentic-dev-harness/cmd/device"
	"github.com/StevenACoffman/agentic-dev-harness/cmd/docscmd"
	"github.com/StevenACoffman/agentic-dev-harness/cmd/doctorcmd"
	"github.com/StevenACoffman/agentic-dev-harness/cmd/evalcmd"
	"github.com/StevenACoffman/agentic-dev-harness/cmd/failurescmd"
	"github.com/StevenACoffman/agentic-dev-harness/cmd/gatecmd"
	"github.com/StevenACoffman/agentic-dev-harness/cmd/harnesscmd"
	"github.com/StevenACoffman/agentic-dev-harness/cmd/initcmd"
	"github.com/StevenACoffman/agentic-dev-harness/cmd/judgecmd"
	"github.com/StevenACoffman/agentic-dev-harness/cmd/kpicmd"
	"github.com/StevenACoffman/agentic-dev-harness/cmd/lessoncmd"
	"github.com/StevenACoffman/agentic-dev-harness/cmd/loopcmd"
	"github.com/StevenACoffman/agentic-dev-harness/cmd/metricscmd"
	"github.com/StevenACoffman/agentic-dev-harness/cmd/nfrcmd"
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
	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
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

	// An unmatched token leaves the selected command a group parent (Exec == nil)
	// with a leftover positional; without this guard it falls through to Run,
	// returns ff.ErrNoExec, and exits 0 — indistinguishable from a bare invocation.
	// A bare invocation leaves no leftover arg and is left to the ErrNoExec path.
	sel := r.Command.GetSelected()
	if sel.Exec == nil {
		if rest := sel.Flags.GetArgs(); len(rest) > 0 {
			return refuse(r, stderr, sel, &adh.Error{
				Code:    adh.EINVALID,
				Message: fmt.Sprintf("%s: unknown subcommand %q", sel.Name, rest[0]),
			})
		}
	}
	if flag, fixed, found := strayFlag(sel.Name, sel.Flags.GetArgs()); found {
		return refuse(r, stderr, sel, &adh.Error{
			Code: adh.EINVALID,
			Message: fmt.Sprintf(
				"%s: %s came after the verb, where it is not parsed; try `%s`",
				sel.Name, flag, fixed),
		})
	}

	if runErr := runSelected(ctx, r, stderr); runErr != nil {
		return runErr
	}
	return nil
}

// refuse reports a pre-dispatch usage error the way the caller asked to be told.
//
// These three refusals -- an unparseable flag, an unknown subcommand, a flag written
// after the verb -- happen before the command runs, so they never reached the envelope
// translation in runSelected. Under --jsonl a machine consumer got the usage banner on
// stderr and *nothing at all* on stdout, which is the same defect as prose on stdout
// wearing different clothes: the caller asked for one envelope per outcome and got no
// outcome. The banner remains the right answer for a human.
func refuse(r *root.Config, stderr io.Writer, sel *ff.Command, err error) error {
	if r.JSONL {
		code := root.CodeForError(err)
		_ = r.EmitError(code, root.ReasonForError(err), err.Error())
		return root.ExitError(code)
	}
	_, _ = fmt.Fprintf(stderr, "\n%s\n", ffhelp.Command(sel))
	return err
}

// strayFlag reports a leftover argument that looks like a flag.
//
// **ff stops parsing flags at the first positional**, so everything after a verb is a
// positional — and a flag written there is silently dropped rather than refused. Three
// measured examples of what that costs: `adh arc list --jsonl` printed human prose to a
// caller that asked for JSON; `adh oracle selftest --seed 7` ran the default seed and
// reported the planted-defect gate as passing; and `adh context verify --repo x` fed
// `--repo` to the verb as an arc id, failing with "no such arc: --repo".
//
// The rule is narrower than "no trailing arguments", and deliberately: several verbs
// take a positional legitimately — `arc show a1`, `nfr show latency`. What no verb
// takes is a **flag-shaped** one, so that is the whole test. A leftover flag is wrong
// whether it names a real flag in the wrong place or an unknown one that should have
// been refused; both are silence today.
//
// A bare "-" is excluded, because it is the conventional name for standard input and is
// a positional wherever a command accepts one.
//
// It lives here rather than in each command for the reason this repository keeps
// arriving at: twelve verbs would be twelve places to remember, and the thirteenth
// command written next month would have the defect on the day it was written. Here a
// new command inherits the guard by existing.
// The suggested form moves the flag **and everything after it** ahead of the verbs,
// rather than the flag alone. That is what makes it right for a flag carrying a value:
// `oracle selftest --seed 7` becomes `oracle --seed 7 selftest`, where moving only the
// flag would have produced `oracle --seed selftest 7`. It is correct whenever the
// trailing run begins with the stray flag, which is how the mistake is actually typed;
// where it is not, the suggestion is still closer than the original and the flag it
// names is the real answer.
func strayFlag(name string, rest []string) (flag, fixed string, found bool) {
	for i, arg := range rest {
		if len(arg) <= 1 || !strings.HasPrefix(arg, "-") {
			continue
		}
		reordered := make([]string, 0, len(rest)+1)
		reordered = append(reordered, name)
		reordered = append(reordered, rest[i:]...)
		reordered = append(reordered, rest[:i]...)
		return arg, strings.Join(reordered, " "), true
	}
	return "", "", false
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
	kpicmd.New(r)
	nfrcmd.New(r)
	doctorcmd.New(r)
	selfeval.New(r)
	sleep.New(r)
	loopcmd.New(r)
	workercmd.New(r)
	judgecmd.New(r)
	harnesscmd.New(r)
	docscmd.New(r)
}
