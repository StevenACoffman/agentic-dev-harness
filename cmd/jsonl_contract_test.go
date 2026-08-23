package cmd_test

import (
	"encoding/json"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/cmd"
	"github.com/StevenACoffman/agentic-dev-harness/cmd/root"
)

// payloadKeys is the `data` key set each command emits when it succeeds against an
// empty tree.
//
// **A maintained list, and the bidirectional assertion below is what makes it
// survivable.** A command in this map must produce exactly these keys, and a command
// producing a data payload must be in this map — so a new machine-readable payload
// fails the suite until somebody records its shape, which is the review moment you want
// when a new contract ships.
//
// Adding an entry here is recording a contract, not silencing a failure. If a test
// fails because a key changed, the question is whether the rename was intended for the
// consumers reading it; if it fails because a command is missing, the question is what
// that command now promises.
//
// Only the commands reachable from nothing appear. The rest refuse for want of an arc
// or a verb, and a refusal is not a violation — the envelope assertion still covers
// them. Payloads that need a fixture (arc show, eval, proof verify) are not covered
// here and that gap is deliberate rather than overlooked.
var payloadKeys = map[string][]string{
	"version": {
		"buildDate", "builtBy", "compiler", "gitCommit", "gitTreeState",
		"gitVersion", "goVersion", "moduleChecksum", "platform",
	},
	"status":  {"arcs_open", "arcs_total", "autonomy", "pending_gates"},
	"init":    {"config", "context_units", "loops", "tools"},
	"metrics": {"accepted", "arcs", "attention_minutes", "attention_per_accept", "compute_tokens"},
	"kpi":     {"proposals"},
	"doctor":  {"problems"},
	"selfeval": {
		"accepted", "arcs", "attention_per_accept", "compute_tokens",
		"deterministic_ratio", "failure_taxonomy", "steps_deterministic", "steps_model",
	},
}

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

// wantEnvelope asserts one line is a §8 envelope and returns its data payload.
//
// It decodes with DisallowUnknownFields on purpose. The envelope is the contract every
// command shares, and a command needing another top-level field is a change to that
// contract — a deliberate edit to root.Outcome rather than a quiet addition in one
// command. Writing EmitJSONL with an ad-hoc struct is one line, which makes an invented
// shape likelier than the key rename this file was added to catch.
func wantEnvelope(t *testing.T, line string, args []string) map[string]json.RawMessage {
	t.Helper()
	var env struct {
		Status  string                     `json:"status"`
		Code    int                        `json:"code"`
		Reason  string                     `json:"reason,omitempty"`
		Message string                     `json:"message,omitempty"`
		Data    map[string]json.RawMessage `json:"data,omitempty"`
	}
	dec := json.NewDecoder(strings.NewReader(line))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&env); err != nil {
		t.Errorf("adh %s --jsonl did not emit a §8 envelope:\n\t%q\n\t%v",
			strings.Join(args, " "), line, err)
		return nil
	}
	switch env.Status {
	case root.StatusOK:
		if env.Code != 0 {
			t.Errorf("adh %s: status ok with exit code %d",
				strings.Join(args, " "), env.Code)
		}
	case root.StatusBlocked, root.StatusError:
		// A refusal has to say why in a token a caller can branch on; the human
		// message is not that.
		if env.Reason == "" {
			t.Errorf("adh %s: status %q carries no reason",
				strings.Join(args, " "), env.Status)
		}
	default:
		t.Errorf("adh %s: unknown status %q", strings.Join(args, " "), env.Status)
	}
	return env.Data
}

// wantPayloadKeys asserts a command's data payload has exactly the recorded keys, in
// both directions.
func wantPayloadKeys(t *testing.T, name string, data map[string]json.RawMessage) {
	t.Helper()
	want, recorded := payloadKeys[name]
	if len(data) == 0 {
		if recorded {
			t.Errorf("adh %s: recorded a payload and emitted none; "+
				"either the command stopped answering or the entry is stale", name)
		}
		return
	}
	if !recorded {
		t.Errorf("adh %s emits a data payload and payloadKeys does not record it; "+
			"a machine-readable contract nothing asserts is one a rename can break "+
			"silently", name)
		return
	}
	got := make([]string, 0, len(data))
	for k := range data {
		got = append(got, k)
	}
	sort.Strings(got)
	if !slices.Equal(got, want) {
		t.Errorf("adh %s payload keys = %v, recorded %v", name, got, want)
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
			for _, line := range strings.Split(out, "\n") {
				if strings.TrimSpace(line) == "" {
					continue
				}
				data := wantEnvelope(t, line, args)
				if len(args) == 1 {
					// Payload keys are recorded per top-level command. A verb's
					// payload usually needs a fixture and is out of scope here.
					wantPayloadKeys(t, args[0], data)
				}
			}
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
