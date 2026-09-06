package root

import (
	"fmt"
	"strings"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
)

// The operand kinds a command can require, and the whole vocabulary of them.
//
// Each value is the noun phrase the refusal reads with, so the wording lives once beside
// the identifier rather than in each command. A kind names *what must exist* for the
// operand to be satisfiable — which is what a caller has to go and get, and what a test
// fixture has to mint — so two commands wanting an arc id share one kind while an
// artifact path and a transcript directory do not.
const (
	OperandArc      OperandKind = "an arc id"
	OperandUnit     OperandKind = "a context unit id"
	OperandLabel    OperandKind = "a label or path to route by"
	OperandPath     OperandKind = "an artifact path"
	OperandManifest OperandKind = "a proof manifest path"
	OperandDir      OperandKind = "a directory of transcripts"
	OperandTitle    OperandKind = "a title"
	OperandLevel    OperandKind = "a level (L0-L4)"
	OperandTool     OperandKind = "a tool id (see `tool list`)"
	OperandSpec     OperandKind = "an NFR spec id"
	OperandClass    OperandKind = "a lesson class"
	OperandStaging  OperandKind = "a staging id"
	OperandBranch   OperandKind = "a branch name"
)

// OperandKind is the closed vocabulary of positional operands a command can require.
//
// A named type rather than a bare string, for the reason SPEC-ADDITIONS §13.2 gives for
// reusing adh.FindingKind instead of inventing a second taxonomy: a closed set with one
// owner can be switched over exhaustively, so a member added later is named by the
// compiler and the linter rather than forgotten by whoever needed it.
type OperandKind string

// MissingOperandError is returned by a verb invoked without the positional operand it
// needs.
//
// **The refusal carries the requirement as data, and that is the whole point.** Twenty-two
// verbs each wrote this refusal as prose, so the only way to learn that `arc show` wants
// an arc id was to read the message — and the --jsonl contract test, which enumerates
// every verb, could not supply an operand it had no way to ask about. Every payload
// reachable only with one therefore went unasserted, and two commands were printing prose
// under --jsonl inside that blind spot.
//
// It is the same correction MissingVerbError made one level up, for the same reason: the
// test used to parse the verb list out of the usage line and silently lost coverage twice
// when the prose drifted. Prose is not a contract.
//
// A missing operand is a *malformed invocation*, distinct from a well-formed one refused
// for its state (ECONFLICT) and from a missing flag, which is not positional and has
// MissingFlagError instead. Only the positional case belongs here.
type MissingOperandError struct {
	Scope string // "arc show" — the command and verb as the caller typed them
	Kind  OperandKind
}

// MissingFlagError is returned by a verb invoked without a flag it requires.
//
// Separate from MissingOperandError because the two are answerable in different ways: an
// operand is positional and names a thing that has to exist, so a caller — or a test
// fixture — can go and get one from a kind alone. A required flag names a knob whose
// value only the caller's intent supplies, and there is no vocabulary of flag values that
// would mean anything.
//
// It exists so the ratchet over refusals needs no exemptions. Three verbs refused for a
// missing flag in prose, and every mechanical check of "does this refusal say what it
// wants" then had to carve them out by name — a list that would go stale the way two
// others in this repository already did. A type they can carry is cheaper than a list of
// commands allowed not to.
type MissingFlagError struct {
	Scope string   // "harness gate" — the command and verb as the caller typed them
	Flags []string // "--candidate", with the dashes, as the caller would type them
}

// MissingVerbError is returned by a command invoked with no verb at all.
//
// A struct rather than a constructor, matching DryRunUnsupportedError: a composite
// literal is not a cross-package call, so callers return it directly instead of
// wrapping an error they did not need to wrap. It also means the verb list travels as
// data, which is what lets one Error method own the wording for every command.
//
// Thirteen commands each wrote this message and its unknown-verb twin by hand, naming
// their own verb list in both -- and again in the usage line, a third copy the --jsonl
// contract test parses to decide what to enumerate. The three agree today; nothing made
// them, and a drift in the usage copy would quietly narrow the contract's coverage
// rather than fail.
type MissingVerbError struct {
	Scope string
	Verbs []string
}

// UnknownVerbError is returned for a verb the command does not dispatch.
type UnknownVerbError struct {
	Scope string
	Got   string
	Verbs []string
}

// Error renders the refusal in the standard "<cmd>: <reason>" form (SPEC §8).
func (e MissingOperandError) Error() string {
	return fmt.Sprintf("%s: requires %s", e.Scope, e.Kind)
}

// Unwrap gives the refusal a machine-readable code; see MissingVerbError.Unwrap.
func (e MissingOperandError) Unwrap() error {
	return &adh.Error{Code: adh.EINVALID, Message: e.Error()}
}

// Error renders the refusal in the standard "<cmd>: <reason>" form (SPEC §8).
func (e MissingFlagError) Error() string {
	return fmt.Sprintf("%s: requires %s", e.Scope, andList(e.Flags))
}

// Unwrap gives the refusal a machine-readable code; see MissingVerbError.Unwrap.
func (e MissingFlagError) Unwrap() error {
	return &adh.Error{Code: adh.EINVALID, Message: e.Error()}
}

// Error renders the refusal in the standard "<cmd>: <reason>" form (SPEC §8).
func (e MissingVerbError) Error() string {
	return fmt.Sprintf("%s: expected a verb: %s", e.Scope, orList(e.Verbs))
}

// Unwrap gives the refusal a machine-readable code.
//
// An invocation with no verb is malformed, not an adh fault. Without a coded cause in
// the chain, ReasonForError reports "internal" -- the token meaning adh itself broke --
// and a caller with a retry-on-internal policy retries what can never succeed. EINVALID
// also selects the usage exit code in CodeForError, so the code fixes both at once.
func (e MissingVerbError) Unwrap() error {
	return &adh.Error{Code: adh.EINVALID, Message: e.Error()}
}

// Error renders the refusal in the standard "<cmd>: <reason>" form (SPEC §8).
func (e UnknownVerbError) Error() string {
	return fmt.Sprintf("%s: unknown verb %q; want %s", e.Scope, e.Got, orList(e.Verbs))
}

// Unwrap gives the refusal a machine-readable code; see MissingVerbError.Unwrap.
func (e UnknownVerbError) Unwrap() error {
	return &adh.Error{Code: adh.EINVALID, Message: e.Error()}
}

// orList renders verbs as prose: "list", "list or promote", "list, doctor, or run".
func orList(verbs []string) string { return joinList(verbs, "or") }

// andList renders required flags as prose: "--checks", "--candidate and --current".
//
// "and" rather than "or", because every flag in the list is required and "or" would say
// the opposite of what the refusal means.
func andList(flags []string) string { return joinList(flags, "and") }

// joinList renders a list as prose with the given final conjunction.
func joinList(items []string, conj string) string {
	switch len(items) {
	case 0:
		return ""
	case 1:
		return items[0]
	case 2:
		return items[0] + " " + conj + " " + items[1]
	default:
		return strings.Join(items[:len(items)-1], ", ") + ", " + conj + " " + items[len(items)-1]
	}
}
