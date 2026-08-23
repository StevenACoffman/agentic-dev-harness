package cmd_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/cmd"
	"github.com/StevenACoffman/agentic-dev-harness/cmd/root"
)

// notData names the commands whose stdout is not a data stream, so --jsonl does not
// apply to them.
//
// One entry, and it should stay that way. `docs` generates a man page: roff *is* its
// product, and there is no JSON rendering of it that a caller would want. Three other
// commands failed this test on its first run — init, metrics, and selfeval printed
// their human report under --jsonl — and every one of those was a defect rather than
// an exception, which is the ratio that makes a single named exception credible.
//
// Adding a name here is a decision to exempt a command from a contract every other
// command keeps. It wants the reason written next to it.
var notData = map[string]bool{"docs": true}

// wantJSONLStdout asserts the --jsonl output contract for one invocation: whatever
// reaches stdout is empty or parses as JSON, one object per line.
//
// The assertion is narrow on purpose. --jsonl does not promise that a command
// *succeeds*, and a usage error legitimately goes to stderr — so an error is not a
// violation. What it promises is that a caller parsing stdout is never handed prose,
// which holds in every state including failure, and is what makes this a table over
// the whole registry rather than a fixture per command.
func wantJSONLStdout(t *testing.T, out string, args []string) {
	t.Helper()
	for _, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var obj map[string]json.RawMessage
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			t.Errorf("adh %s --jsonl wrote non-JSON to stdout:\n\t%q\n\t%v",
				strings.Join(args, " "), line, err)
			return
		}
	}
}

// TestJSONLStdoutIsAlwaysJSON is the output half of the flag defect fixed alongside it.
//
// `adh arc list --jsonl` printed "no arcs" to stdout — human prose to a caller that
// asked for JSON, plausible enough to be parsed as an empty result. The stray-flag
// guard closes that from the input side; **this closes it from the other, and would
// have caught the original bug without anyone reasoning about ff's parse order.**
//
// The command list is derived from the registry rather than written out, so a command
// added later is covered on the day it is registered rather than on the day somebody
// remembers this file.
// It runs serially: each case chdirs into a fresh temp directory so a command reads no
// state from the developer's own repository, and t.Chdir is process-global and refuses
// to run under t.Parallel.
func TestJSONLStdoutIsAlwaysJSON(t *testing.T) {
	for _, args := range registeredInvocations(t) {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			t.Chdir(t.TempDir())
			out, _ := run(t, append([]string{"--jsonl"}, args...)...)
			wantJSONLStdout(t, out, args)
		})
	}
}

// registeredInvocations lists every registered command and, for a command that groups
// subcommands, each of those — so the table covers what the CLI actually exposes.
func registeredInvocations(t *testing.T) [][]string {
	t.Helper()
	r := root.New(func(string) string { return "" }, strings.NewReader(""), nil, nil)
	cmd.RegisterForTest(r)

	var out [][]string
	for _, cmd := range r.Command.Subcommands {
		if notData[cmd.Name] {
			continue
		}
		if len(cmd.Subcommands) == 0 {
			out = append(out, []string{cmd.Name})
			continue
		}
		for _, sub := range cmd.Subcommands {
			out = append(out, []string{cmd.Name, sub.Name})
		}
	}
	return out
}
