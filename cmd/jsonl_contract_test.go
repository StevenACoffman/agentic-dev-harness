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

// jsonlDebt names invocations that do not yet honour --jsonl.
//
// **These are violations, not exceptions.** notData below is for a command where JSON
// does not apply; every entry here is a command that should emit an envelope and
// prints prose instead. They surfaced together when the table started deriving verbs
// from the usage line, which is how eleven of them were hiding at once.
//
// It is a ratchet and the assertion runs both ways: an invocation listed here must
// still violate, and one not listed must not. So fixing a command *fails this test*
// until its entry is removed, and a new violation fails immediately. The list can only
// shrink.
//
// Each is a multi-part human report -- staged proposals, calibration, common patterns
// -- whose JSON shape is a contract worth designing rather than transcribing. Minting
// eight machine contracts in an afternoon is how you get eight you regret, and the
// envelope assertion already covers the commands themselves.
var jsonlDebt = map[string]bool{
	"autonomy show":     true,
	"oracle diff":       true,
	"oracle invariants": true,
	"oracle selftest":   true,
	"context lint":      true,
	"context index":     true,
	"sleep run":         true,
	"sleep status":      true,
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

// wantStillViolating asserts a recorded debt has not been paid without being removed
// from the list.
//
// The ratchet only works in both directions. Without this, fixing a command would
// leave a stale entry that silently exempts it again the next time it regresses.
func wantStillViolating(t *testing.T, out string, args []string) {
	t.Helper()
	for _, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var obj map[string]json.RawMessage
		if json.Unmarshal([]byte(line), &obj) != nil {
			return // still prose, as recorded
		}
	}
	t.Errorf("adh %s --jsonl now emits JSON; remove it from jsonlDebt so the "+
		"contract is enforced rather than exempted", strings.Join(args, " "))
}

// checkInvocation runs one invocation with --jsonl and applies the contract to it.
//
// Extracted from the table so the loop reads as what it is — every registered
// invocation, checked — and so the three assertions it composes stay separable.
func checkInvocation(t *testing.T, args []string) {
	t.Helper()
	out, _ := run(t, append([]string{"--jsonl"}, args...)...)
	if jsonlDebt[strings.Join(args, " ")] {
		wantStillViolating(t, out, args)
		return
	}
	wantJSONLStdout(t, out, args)
	for _, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		data := wantEnvelope(t, line, args)
		if len(args) == 1 {
			// Payload keys are recorded per top-level command. A verb's payload
			// usually needs a fixture and is out of scope here.
			wantPayloadKeys(t, args[0], data)
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
			checkInvocation(t, args)
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
		out = append(out, []string{cmd.Name})
		for _, sub := range cmd.Subcommands {
			out = append(out, []string{cmd.Name, sub.Name})
		}
		for _, verb := range verbsFromUsage(cmd.Usage) {
			out = append(out, []string{cmd.Name, verb})
		}
	}
	return out
}

// verbsFromUsage extracts the verbs a command dispatches internally, from the
// `<a|b|c>` group in its usage line.
//
// **Twelve commands dispatch on args[0] rather than registering ff subcommands, so the
// registry walk above cannot see their verbs** — and that blind spot is why `tool list`
// and `arc new` were both violating the --jsonl contract while this test passed. The
// usage string is where those verbs are already written down, and it is user-facing, so
// it is kept accurate by the same pressure that keeps `--help` accurate.
//
// A verb needing an argument will refuse, and a refusal is not a violation: the envelope
// assertion still applies to it. What this buys is that the verbs which *do* answer are
// covered.
func verbsFromUsage(usage string) []string {
	open := strings.Index(usage, "<")
	if open < 0 {
		return nil
	}
	shut := strings.Index(usage[open:], ">")
	if shut < 0 {
		return nil
	}
	group := usage[open+1 : open+shut]
	if !strings.Contains(group, "|") {
		return nil // a placeholder like <arc-id>, not a verb list
	}
	var verbs []string
	for _, alt := range strings.Split(group, "|") {
		// "set L0-L4" is a verb plus its argument; the verb is the first word.
		if verb, _, _ := strings.Cut(strings.TrimSpace(alt), " "); verb != "" {
			verbs = append(verbs, verb)
		}
	}
	return verbs
}
