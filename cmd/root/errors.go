package root

import (
	"fmt"
	"strings"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
)

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
func orList(verbs []string) string {
	switch len(verbs) {
	case 0:
		return ""
	case 1:
		return verbs[0]
	case 2:
		return verbs[0] + " or " + verbs[1]
	default:
		return strings.Join(verbs[:len(verbs)-1], ", ") + ", or " + verbs[len(verbs)-1]
	}
}
