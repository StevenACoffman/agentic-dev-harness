package doctorcmd

import (
	"os/exec"
	"sort"
	"strings"

	"github.com/StevenACoffman/agentic-dev-harness/internal/contextstore"
	"github.com/StevenACoffman/agentic-dev-harness/internal/harnesscheck"
	"github.com/StevenACoffman/agentic-dev-harness/internal/toolreg"
	"github.com/StevenACoffman/skillet/claims"
	"github.com/StevenACoffman/skillet/markdown"
)

// commandProblems reports a command a context unit's prose claims the repository
// supports, that does not resolve.
//
// **"Every instruction the agent reads must be backed by a working command."** A unit
// naming `make verify` claims that target exists; `doctor` checks harness integrity and
// `context verify` checks context drift, and neither checked whether an instruction's
// commands resolve. It is the §13 tool registry's discipline pointed at prose.
//
// The split is skillet's and it is already half-built. `claims.Commands` finds
// command-shaped claims and imports nothing outside the standard library, deliberately:
// **resolving one to an executable is environment-dependent and belongs in the
// consumer.** adh is the consumer, and this is where the environment is allowed.
//
// A command resolves when it names a declared §13 tool or an executable on `$PATH`.
// Both are environmental, so an unresolved one is advisory — a missing toolchain is not
// a defect in the prose — and `doctor` treats every problem that way.
//
// Reported **once per distinct command**, not once per mention. `claims.Commands` errs
// toward finding fewer and its documented false positive is lowercase prose inside a
// code span; combined with a `$PATH` lookup on a fresh machine the honest failure mode
// is chattiness, and deduplicating is what keeps one uninstalled tool from producing
// twenty findings.
// storeDir is the *store* directory rather than the repository root, because
// contextstore.Content resolves a unit's ContentPath under the store and keeps it
// within it — passing the repo root reads nothing and reports nothing, which is how
// this check silently did nothing on its first run.
func commandProblems(
	storeDir string, units []contextstore.Unit, tools toolreg.Registry,
) []harnesscheck.Problem {
	declared := declaredTools(tools)
	seen := make(map[string]bool)
	problems := make([]harnesscheck.Problem, 0)

	for i := range units {
		unit := &units[i]
		body, err := contextstore.Content(storeDir, unit)
		if err != nil || body == "" {
			// An unreadable content path is already reported as a dangling source, so
			// saying it twice under a second name would only bury the first.
			continue
		}
		for _, cmd := range claims.Commands(markdown.Parse(body).CodeSpans) {
			name, _, _ := strings.Cut(cmd, " ")
			if seen[cmd] || declared[name] || onPath(name) {
				continue
			}
			seen[cmd] = true
			problems = append(problems, harnesscheck.Problem{
				Kind: harnesscheck.KindDanglingCommand, Ref: unit.ID,
				Detail: "prose names a command that does not resolve: " + cmd,
			})
		}
	}
	sort.Slice(problems, func(i, j int) bool { return problems[i].Detail < problems[j].Detail })
	return problems
}

// declaredTools indexes the §13 registry by the first token of each tool's command, so
// a unit naming `modelith lint` matches a tool declared as `modelith lint --format json`.
func declaredTools(tools toolreg.Registry) map[string]bool {
	out := make(map[string]bool, len(tools.Tools))
	for i := range tools.Tools {
		name, _, _ := strings.Cut(tools.Tools[i].Run, " ")
		out[name] = true
	}
	return out
}

// onPath reports whether name is an executable the environment provides.
//
// A relative path (`./scripts/build.sh`) is looked up as written, which exec.LookPath
// handles: it resolves a name containing a separator against the working directory
// rather than against $PATH.
func onPath(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
