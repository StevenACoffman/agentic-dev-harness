package cmd_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/StevenACoffman/agentic-dev-harness/cmd"
	"github.com/StevenACoffman/agentic-dev-harness/cmd/root"
	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/vcs"
)

// payloadKeys is the `data` key set each command emits when it succeeds.
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
// Three trees' worth of reach are covered: the bare one for the commands that answer
// from nothing, the seeded one for those needing an arc, a registry, or a git
// repository, and — since the refusals started carrying their requirement as data —
// the operands themselves, supplied from the fixture that minted them.
//
// `arc show`'s payload is an adh.Arc, so its key set is whatever the fixture's arc
// populates: adding a field to Arc without `omitempty` fails this entry. That is the
// review moment, not a defect — a new field on the arc record is a change to what every
// caller of `arc show` receives.
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
	"autonomy show":     {"level"},
	"context check":     {"units"},
	"context eval":      {"cases", "passed", "precision", "recall"},
	"context index":     {"units"},
	"context lint":      {"problems", "units"},
	"context misses":    {"misses", "proposals"},
	"context verify":    {"drift", "results"},
	"device validate":   {"detail", "ok"},
	"loop tick":         {"arcs_opened", "loops", "results"},
	"nfr lint":          {"specs", "valid"},
	"oracle diff":       {"boards", "mode"},
	"oracle invariants": {"boards", "hold"},
	"oracle selftest":   {"passed"},
	"sleep run":         {"reason", "staged"},
	"sleep status":      {"common_patterns", "staged"},
	"tool doctor":       {"tools", "valid"},
	"worker show":       {"epoch", "models"},

	// Reachable only with state, and unasserted until the seeded run existed. Five of
	// these were printing prose under --jsonl at the same time.
	"arc list":         {"id", "stage", "status", "title"},
	"failures list":    {"candidates", "confirmed"},
	"context list":     {"freshness", "id", "kind", "labels", "verified"},
	"loop list":        {"goal", "id"},
	"tool list":        {"id", "verifies"},
	"vcs status":       {"branch", "changed", "clean"},
	"worker requalify": {"epoch", "roles"},

	// Reachable only with an *operand*, and unasserted until the refusals started
	// naming what they wanted and the fixture started minting it. Seven of these were
	// printing prose under --jsonl at the same time: arc new, context route, nfr list,
	// autonomy set, ops, harness hash, and strategy.
	//
	// `device validate` was the eighth and is in the block above, because it needs no
	// operand: it was unreachable only because `device` refused in prose, so the walk
	// could not see it had a verb at all.
	"arc new":       {"id"},
	"arc show":      {"id", "labels", "proof", "stage", "status", "title"},
	"autonomy set":  {"level"},
	"context route": {"units"},
	"context show": {
		"content", "id", "kind", "owner", "provenance", "sources", "superseded_by",
		"verified",
	},
	"harness eval":    {"diagnosis", "rubric"},
	"harness hash":    {"artifact", "hash"},
	"nfr list":        {"guard", "id", "scale", "tag"},
	"nfr show":        {"direction", "fail", "goal", "id", "meter", "scale", "tag"},
	"ops":             {"arc", "at_gate", "stage"},
	"proof create":    {"arc", "artifacts", "git_sha", "manifest"},
	"proof verify":    {"arc", "artifacts", "verified"},
	"sessions report": {"by_marker", "retries", "sessions", "turns"},
	"step":            {"arc", "stage", "status"},
	"strategy":        {"arc", "stage", "status"},
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
// **It is empty, and an empty ratchet is still load-bearing.** The eleven entries are
// paid; what remains is the forward direction, which fails the moment a command prints
// prose under --jsonl. Deleting the map because it holds nothing would remove the guard
// that keeps it holding nothing.
var jsonlDebt = map[string]bool{}

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

// fixture is the seeded state, and every value in it that a command can be asked to name.
//
// **It exists so the operand invocations are derived rather than maintained.** The entry
// asking for this coverage recorded the constraint: a second table — of arguments this
// time — goes stale the way the verb table did twice in three passes. So this type owns
// every value, and no operand is written down anywhere else.
//
// Owning them is not the same as reading them back, and only ArcID is read back — the
// harness mints that one. The rest are values this fixture chooses and then writes, and
// Manifest is neither: it restates `proof create`'s default output path, which is a second
// copy that could drift. **What makes that safe is not care, it is that drift fails
// loudly**: a wrong value makes the command refuse, and `wantPayloadReached` reports a
// recorded payload that no longer appears.
type fixture struct {
	ArcID       string // read back from `arc new`, the string a caller reads off stdout
	UnitID      string // the context unit this fixture writes
	Label       string // the label shared by the arc and the unit, so routing matches
	Artifact    string // a committed file, hashable as a proof artifact
	Manifest    string // `proof create`'s default path for ArcID; drift makes verify refuse
	Transcripts string // a directory holding one Claude-format session
	SpecID      string // the NFR spec this fixture writes
}

// operands are the argv words to append to an invocation, or the reason the fixture mints
// nothing of the kind asked for. Exactly one of the two is set.
//
// A named pair rather than two return values: "" for the second string means something
// entirely different from "" for a slice, and at the call site neither position says
// which.
type operands struct {
	Args  []string
	Unmet string
}

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

// wantRecordedKeys asserts that a payload a command actually emitted is recorded, with
// exactly these keys. Runs against every tree, because a payload is a contract wherever
// it appears.
func wantRecordedKeys(t *testing.T, name string, data map[string]json.RawMessage) {
	t.Helper()
	if len(data) == 0 {
		return
	}
	want, recorded := payloadKeys[name]
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

// wantPayloadReached asserts the converse — a recorded payload is actually produced —
// and runs only against the seeded tree.
//
// The split is not tidiness. Against a bare tree most commands refuse for want of an
// arc, so "recorded but emitted nothing" is the normal case there and asserting it
// would forbid recording any payload that needs state. The seeded tree is the only
// place this direction can be true, so it is the only place it is checked.
func wantPayloadReached(t *testing.T, name string, data map[string]json.RawMessage) {
	t.Helper()
	if _, recorded := payloadKeys[name]; recorded && len(data) == 0 {
		t.Errorf("adh %s: recorded a payload and emitted none even with state; "+
			"either the command stopped answering or the entry is stale", name)
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

// checkInvocation asserts the --jsonl contract for one invocation: `form` is the command
// and verb, `extra` the operands the fixture supplied for it.
//
// The two are separate because `payloadKeys` is keyed by the *form*. Keying it by the
// whole argv would put a fixture-minted id in the key, so the recorded contract would
// change name every time the fixture did — and a payload is a promise about `arc show`,
// not about `arc show arc-0001`.
func checkInvocation(t *testing.T, form, extra []string, seeded bool) {
	t.Helper()
	args := append(append([]string{}, form...), extra...)
	out, _ := run(t, append([]string{"--jsonl"}, args...)...)
	name := strings.Join(form, " ")
	if jsonlDebt[name] {
		wantStillViolating(t, out, args)
		return
	}
	wantJSONLStdout(t, out, args)
	for _, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		data := wantEnvelope(t, line, args)
		// Keyed by the whole form, verb included. It was once keyed by the top-level
		// command on the reasoning that a verb's payload needs a fixture -- true when
		// written, and false as soon as eight verbs learned to answer from an empty tree.
		wantRecordedKeys(t, name, data)
		if seeded {
			wantPayloadReached(t, name, data)
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
			checkInvocation(t, args, nil, false)
		})
	}
}

// registeredInvocations lists every registered command and, for a command that groups
// subcommands, each of those — so the table covers what the CLI actually exposes.
func registeredInvocations(t *testing.T) [][]string {
	t.Helper()
	// Probing runs each command bare, so keep it out of the repository. Subtests
	// chdir to their own directories afterwards.
	t.Chdir(t.TempDir())
	r := root.New(func(string) string { return "" }, strings.NewReader(""), nil, nil)
	cmd.RegisterForTest(r)

	var out [][]string
	for _, c := range r.Command.Subcommands {
		if notData[c.Name] {
			continue
		}
		out = append(out, []string{c.Name})
		for _, sub := range c.Subcommands {
			out = append(out, []string{c.Name, sub.Name})
		}
		for _, verb := range dispatchedVerbs(t, c.Name) {
			out = append(out, []string{c.Name, verb})
		}
	}
	return out
}

// dispatchedVerbs asks a command which verbs it dispatches, by invoking it bare and
// reading the list off its own refusal.
//
// Twelve commands dispatch on args[0] rather than registering ff subcommands, so the
// registry walk cannot see their verbs. This used to parse them out of the `<a|b|c>`
// group in the usage line, which failed two ways: a command declaring a flag first put
// a placeholder there (`arc [--label <l>]... <new|list|show>` reads `<l>`), and a
// single-verb command has no alternation to find (`gate <list>`) — so `arc list`, `arc
// show`, `gate list` and `failures list` were never enumerated and never checked.
//
// Asking the command is not a workaround for that, it is the right source. The refusal
// carries the verb list as data, so there is no prose to parse and no third copy to
// drift.
func dispatchedVerbs(t *testing.T, name string) []string {
	t.Helper()
	_, err := run(t, name)
	var missing root.MissingVerbError
	if errors.As(err, &missing) {
		return missing.Verbs
	}
	return nil
}

// verbGroup finds the alternation group in a usage line — the first <a|b|c>.
//
// It scans every angle-bracket group rather than only the first, because a command
// that declares a flag ahead of its verb puts a placeholder there first: `arc
// [--label <l>]... <new|list|show>` reads <l>, which has no alternation. Taking only
// the first group silently skipped every arc verb, so the walk never reached the
// payloads `arc list` and `arc show` produce — the exact payloads the item asking for
// this coverage named.
func verbGroup(usage string) string {
	for rest := usage; ; {
		open := strings.Index(rest, "<")
		if open < 0 {
			return ""
		}
		rest = rest[open+1:]
		shut := strings.Index(rest, ">")
		if shut < 0 {
			return ""
		}
		if group := rest[:shut]; strings.Contains(group, "|") {
			return group
		}
		rest = rest[shut+1:]
	}
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
	group := verbGroup(usage)
	if group == "" {
		return nil
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

// seedFixture builds the minimum state that makes the state-dependent commands answer,
// and returns what it minted.
//
// One fixture, not one per command. The point is not to exercise each command's logic —
// the packages have their own tests for that — but to get past the "no arc" refusal so
// the --jsonl contract is checked on the surface an agent actually consumes.
//
// It commits, which earlier versions did not, because `proof create` records provenance
// only when a HEAD exists (SPEC §5.4). Without the commit the recorded payload would pin
// the shape with no `git_sha` — the one shape a real proof packet never has.
func seedFixture(t *testing.T) *fixture {
	t.Helper()
	repo, err := vcs.Init(".")
	if err != nil {
		t.Fatalf("git init: %v", err)
	}
	mustRun(t, "init")

	fx := &fixture{UnitID: "u1", Label: "sec", Artifact: "artifact.txt", SpecID: "s1"}
	dir := filepath.Join(".adh", "context")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	unit := `{"id":"` + fx.UnitID + `","kind":"note","labels":["` + fx.Label +
		`"],"verified":"machine-confirmed"}`
	if err := os.WriteFile(filepath.Join(dir, fx.UnitID+".json"), []byte(unit), 0o600); err != nil {
		t.Fatalf("write unit: %v", err)
	}
	specDir := filepath.Join(".adh", "nfr")
	if err := os.MkdirAll(specDir, 0o750); err != nil {
		t.Fatalf("mkdir %s: %v", specDir, err)
	}
	spec := `{"id":"` + fx.SpecID + `","tag":"latency","scale":"p95 ms","meter":"bench",` +
		`"direction":"lower_is_better","fail":100,"goal":50}`
	if err := os.WriteFile(
		filepath.Join(specDir, fx.SpecID+".json"),
		[]byte(spec),
		0o600,
	); err != nil {
		t.Fatalf("write spec: %v", err)
	}
	if err := os.WriteFile(fx.Artifact, []byte("fixture artifact\n"), 0o600); err != nil {
		t.Fatalf("write artifact: %v", err)
	}
	if _, err := repo.Commit("fixture", vcs.Signature{
		Name: "adh test", Email: "test@example.com",
	}, time.Unix(0, 0).UTC()); err != nil {
		t.Fatalf("commit: %v", err)
	}

	fx.ArcID = strings.TrimSpace(mustRun(t, "arc", "--label", fx.Label, "new", "fixture arc"))
	fx.Manifest = filepath.Join(".adh", "proof", fx.ArcID+".json")
	mustRun(t, "proof", "create", fx.ArcID, fx.Artifact)

	fx.Transcripts = "transcripts"
	if err := os.MkdirAll(fx.Transcripts, 0o750); err != nil {
		t.Fatalf("mkdir %s: %v", fx.Transcripts, err)
	}
	turn := `{"type":"user","sessionId":"s1","message":{"role":"user","content":"hello"}}` + "\n"
	session := filepath.Join(fx.Transcripts, "one.jsonl")
	if err := os.WriteFile(session, []byte(turn), 0o600); err != nil {
		t.Fatalf("write transcript: %v", err)
	}
	return fx
}

// operandFor answers what the fixture can hand a command that asked for one kind of
// operand.
//
// Requires: kind is a member of the root.OperandKind vocabulary.
// Ensures:  exactly one of Args and Unmet is set on the result.
//
// Pure: a function of the fixture's recorded values and the kind, with no I/O. The switch
// carries no default so `exhaustive` names a kind added later, which is the whole reason
// OperandKind is a closed named type instead of a string.
//
// **An unmet kind is a decision, and each one says why.** This is the `notData` rule
// applied to operands: exempting a command from a contract every other command keeps
// wants the reason written next to it, because a reader has to be able to tell a gap
// somebody chose from one nobody noticed.
func (fx *fixture) operandFor(kind root.OperandKind) operands {
	switch kind {
	case root.OperandArc:
		return operands{Args: []string{fx.ArcID}}
	case root.OperandUnit:
		return operands{Args: []string{fx.UnitID}}
	case root.OperandLabel:
		return operands{Args: []string{fx.Label}}
	case root.OperandPath:
		return operands{Args: []string{fx.Artifact}}
	case root.OperandManifest:
		return operands{Args: []string{fx.Manifest}}
	case root.OperandDir:
		return operands{Args: []string{fx.Transcripts}}
	case root.OperandSpec:
		return operands{Args: []string{fx.SpecID}}
	case root.OperandTitle:
		return operands{Args: []string{"an arc the contract test made"}}
	case root.OperandLevel:
		return operands{Args: []string{"L1"}}
	case root.OperandBranch:
		return operands{Args: []string{"contract-test-branch"}}
	case root.OperandTool:
		return operands{Unmet: "a tool id would make `tool run` execute a third-party " +
			"binary; the contract must not depend on what is installed here"}
	case root.OperandClass:
		return operands{Unmet: "a lesson class is evidence of a harvest over closed " +
			"arcs, and minting one means faking the corpus it is evidence of"}
	case root.OperandStaging:
		return operands{Unmet: "a staging id needs a `sleep` run that staged something, " +
			"which needs closed arcs for the same reason as a lesson class"}
	}
	return operands{Unmet: "no fixture rule for operand kind " + string(kind)}
}

// operandsFor maps the kinds an invocation requires onto the values this fixture minted.
//
// Requires: every kind is a member of the root.OperandKind vocabulary.
// Ensures:  Args holds one value per kind in order, or Unmet names the first kind the
//
//	fixture does not mint.
//
// Pure, and separate from discovery on purpose: what a command needs is found by asking
// it (I/O, and in a tree nobody asserts against), while what this fixture can answer with
// is a function of its own fields.
func (fx *fixture) operandsFor(kinds []root.OperandKind) operands {
	var args []string
	for _, kind := range kinds {
		next := fx.operandFor(kind)
		if next.Unmet != "" {
			return operands{Unmet: next.Unmet}
		}
		args = append(args, next.Args...)
	}
	return operands{Args: args}
}

// discoverOperands asks every form which operand kinds it requires, by invoking it with a
// placeholder for each kind it names until it stops naming one.
//
// **It must not run in a tree anything is asserted against.** An earlier version probed
// in the fixture's own directory on the reasoning that a command refusing for a missing
// operand had not done anything yet — true of every round but the last, which is by
// definition the round that no longer refuses and therefore *runs the command*. `adh
// strategy <arc>` advanced the fixture's arc to execution during discovery, so the
// invocation under assertion then refused for its stage and printed nothing, and a
// seventh command printing prose under --jsonl stayed hidden behind a passing test.
//
// The placeholder need not be a real id. Every command checks its operand count before it
// looks anything up, so the kinds it names do not depend on the tree it is asked in — and
// a lookup that fails on a placeholder ends the loop just as cleanly as a success. That
// invariant is what lets one throwaway tree answer for the whole walk.
//
// **It probes without --jsonl, which is load-bearing.** Under --jsonl, runSelected
// translates the command's error into an envelope and returns a bare ExitError carrying
// only the exit code, so errors.As finds no typed error to match: the refusal's kind
// survives in the human message alone. dispatchedVerbs probes bare for the same reason.
//
// A command still naming an operand after maxRounds fails the test rather than looping.
// Nothing needs more than two — `proof create` wants an arc and a path — so a third round
// means a command is asking for something it was already handed.
func discoverOperands(t *testing.T, forms [][]string) map[string][]root.OperandKind {
	t.Helper()
	const maxRounds = 3
	needs := make(map[string][]root.OperandKind, len(forms))
	for _, form := range forms {
		var kinds []root.OperandKind
		for range maxRounds {
			argv := slices.Clone(form)
			for range kinds {
				argv = append(argv, "operand-placeholder")
			}
			_, err := run(t, argv...)
			var missing root.MissingOperandError
			if !errors.As(err, &missing) {
				break
			}
			kinds = append(kinds, missing.Kind)
		}
		if len(kinds) >= maxRounds {
			t.Errorf("adh %s still wants an operand after %d were supplied: %v",
				strings.Join(form, " "), maxRounds, kinds)
			continue
		}
		if len(kinds) > 0 {
			needs[strings.Join(form, " ")] = kinds
		}
	}
	return needs
}

// TestJSONLStdoutIsAlwaysJSONWithState is the same contract over a seeded tree, and it
// is the half that was missing.
//
// Against a bare temp directory every state-dependent command refuses, prints nothing,
// and trivially satisfies "empty or JSON" — so the contract went unenforced across the
// whole surface an agent actually consumes, and three commands were printing prose
// there unobserved. A test that cannot fail for the interesting inputs is not covering
// them.
func TestJSONLStdoutIsAlwaysJSONWithState(t *testing.T) {
	forms := registeredInvocations(t)
	needs := discoverOperands(t, forms)
	for _, args := range forms {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			t.Chdir(t.TempDir())
			fx := seedFixture(t)
			// An unmet operand narrows this case to the bare refusal rather than
			// skipping it: the form is still under contract, only the payload behind
			// the operand goes unreached, and the reason is on the record.
			ops := fx.operandsFor(needs[strings.Join(args, " ")])
			if ops.Unmet != "" {
				t.Logf("operand not minted, asserting the bare refusal only: %s", ops.Unmet)
			}
			checkInvocation(t, args, ops.Args, true)
		})
	}
}

// TestUsageLineNamesTheVerbsItDispatches keeps the third copy of each verb list honest.
//
// Every verb-dispatching command states its verbs twice in code -- now once, via
// MissingVerbError -- and once more as prose in its usage line. The prose copy is what a
// person reads to learn what the command does, and nothing made it agree with the code.
// It agreed when this was written; that is not the same as being kept in agreement, and
// the enumeration above used to depend on it, so a drift narrowed the contract's coverage
// silently rather than failing.
//
// A command whose usage line has no alternation group is skipped, not failed: `gate
// <list>` and `failures <list>` cannot spell a one-item alternation, and demanding they
// invent one would be the test dictating prose.
func TestUsageLineNamesTheVerbsItDispatches(t *testing.T) {
	t.Chdir(t.TempDir())
	r := root.New(func(string) string { return "" }, strings.NewReader(""), nil, nil)
	cmd.RegisterForTest(r)
	for _, c := range r.Command.Subcommands {
		t.Run(c.Name, func(t *testing.T) {
			dispatched := dispatchedVerbs(t, c.Name)
			if len(dispatched) == 0 {
				t.Skip("does not dispatch verbs")
			}
			stated := verbsFromUsage(c.Usage)
			if len(stated) == 0 {
				t.Skipf("usage line states no alternation: %q", c.Usage)
			}
			if !slices.Equal(stated, dispatched) {
				t.Errorf("usage line says %v, the command dispatches %v\n\tusage: %q",
					stated, dispatched, c.Usage)
			}
		})
	}
}

// wantNotInternal asserts no line of out reports reason "internal".
//
// Extracted before the caller rather than after, which is the discipline this file keeps
// having to relearn: an inline loop-plus-decode-plus-check pushed the test past the
// complexity limit and read as machinery rather than as the claim it makes.
func wantNotInternal(t *testing.T, out, scope string) {
	t.Helper()
	for _, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var env struct {
			Reason string `json:"reason"`
			Code   int    `json:"code"`
		}
		if json.Unmarshal([]byte(line), &env) != nil {
			continue
		}
		if env.Reason == "internal" {
			t.Errorf("adh %s <bad verb> reported reason %q; a mistyped verb is not an "+
				"adh fault, and internal is what a caller retries on", scope, env.Reason)
		}
	}
}

// TestUsageErrorsAreNotReportedAsInternal is the ratchet on the reason vocabulary.
//
// `reason` is the token an agent branches on, and `internal` is the one value that means
// *adh itself broke* — retry, or escalate to a human. Every verb-dispatching command
// reported it for a mistyped verb, because an untyped error has no code and
// adh.ErrorCode defaults to EINTERNAL. So a caller with a sensible retry-on-internal
// policy retried an invocation that could never succeed, and the exit code said 1 where
// a usage error means 2.
//
// Derived from the same walk as the contract above, so a command added later inherits
// the assertion rather than needing to be remembered.
func TestUsageErrorsAreNotReportedAsInternal(t *testing.T) {
	for _, args := range registeredInvocations(t) {
		if len(args) != 1 {
			continue
		}
		t.Run(args[0], func(t *testing.T) {
			t.Chdir(t.TempDir())
			if len(dispatchedVerbs(t, args[0])) == 0 {
				t.Skip("does not dispatch verbs")
			}
			out, _ := run(t, "--jsonl", args[0], "definitely-not-a-verb")
			wantNotInternal(t, out, args[0])
		})
	}
}

// TestMissingOperandRefusalsCarryTheirKind is the ratchet on operand adoption.
//
// **Without it, a command that refuses in prose is invisible exactly the way the seven
// this pass found were.** The --jsonl walk supplies an operand only when the refusal
// names one as data, so a verb refusing with a bare adh.Error gets no operand, reaches no
// payload, and passes. Twenty-two sites adopted MissingOperandError in one change;
// nothing but this stops the twenty-third from being written the old way, and the cost of
// that is silent — a contract nobody asserts rather than a test that fails.
//
// It asserts the claim it can prove from outside: a bare invocation refused as EINVALID
// has to carry a typed refusal. EINVALID is the code for a malformed invocation, and with
// nothing supplied there are three ways to be malformed — no verb, no operand, no
// required flag — each of which has a type. A well-formed invocation refused for its
// state reports ECONFLICT or ENOTFOUND and is not this test's business.
//
// **It has no exemption list, and that was worth the third type.** Three verbs refused
// for a missing flag in prose; carving them out by name would have been an eighth of the
// work and would have added the fourth maintained list in a file whose comments record
// two of the first three going stale.
//
// Derived from the same registry walk as the contract, so a command added later inherits
// the assertion rather than needing to be remembered — and it runs against a bare tree
// because every command checks its operand count before it looks anything up, so no
// fixture changes the answer.
func TestMissingOperandRefusalsCarryTheirKind(t *testing.T) {
	for _, form := range registeredInvocations(t) {
		t.Run(strings.Join(form, " "), func(t *testing.T) {
			t.Chdir(t.TempDir())
			_, err := run(t, form...)
			if err == nil || adh.ErrorCode(err) != adh.EINVALID {
				return
			}
			var (
				operand root.MissingOperandError
				verb    root.MissingVerbError
				missing root.MissingFlagError
			)
			if errors.As(err, &operand) || errors.As(err, &verb) ||
				errors.As(err, &missing) {
				return
			}
			t.Errorf("adh %s refuses as invalid without saying what it wants:\n\t%v\n"+
				"a refusal for a missing operand has to carry its kind, or the --jsonl "+
				"walk cannot supply one and whatever payload sits behind it goes "+
				"unasserted", strings.Join(form, " "), err)
		})
	}
}
