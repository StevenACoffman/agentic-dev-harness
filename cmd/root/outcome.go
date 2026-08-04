// outcome.go is the machine-output envelope (SPEC §8): the generic JSONL result
// shape climax generates (`climax init --jsonl`), specialized here with adh's
// reason vocabulary and error-code mapping. climax owns the envelope's shape;
// adh owns its words.

package root

import (
	"encoding/json"
	"fmt"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
)

// Outcome status values (SPEC §8): the class of a command's result, the field an
// agent switches on under --jsonl.
const (
	StatusOK      = "ok"      // the command succeeded
	StatusBlocked = "blocked" // stopped at a human gate or an environment gap, not a failure
	StatusError   = "error"   // the command failed
)

// Outcome reason tokens: a stable machine string for a blocked/error outcome, so
// an agent branches on the token instead of matching prose. Free-form reasons
// (e.g. a confirmed finding's kind) may also appear; these are the shared ones.
const (
	ReasonAtOps      = "at_ops"     // arc reached the ops ship gate (step)
	ReasonUngrounded = "ungrounded" // critic routing gap, exit 12 (step)
	ReasonGate       = "gate"       // pending human approval, exit 4 (approve)
	ReasonProof      = "proof"      // proof verification failed, exit 8 (proof/close)
	ReasonRequalify  = "requalify"  // worker changed; requalify before running, exit 9 (§14)
	ReasonFailed     = "failed"     // arc failed evaluation past its rework budget (run, §4.1)
)

// Exit codes surfaced in an error outcome's Code for a generic returned error
// (SPEC §7): usage/validation vs. everything else. Domain gates set their own
// (4, 5–8, 12) at the call site.
const (
	codeGeneric = 1
	codeUsage   = 2
)

// Outcome is a generic machine-readable result envelope, written as one JSON
// object per line (JSONL) on stdout.
type Outcome struct {
	Status  string `json:"status"`            // ok | blocked | error
	Code    int    `json:"code"`              // process exit code (0 for ok)
	Reason  string `json:"reason,omitempty"`  // stable machine token (app-defined)
	Message string `json:"message,omitempty"` // human-readable detail
	Data    any    `json:"data,omitempty"`    // payload on success
}

// EmitJSONL writes v as a single JSON line to Stdout.
func (c *Config) EmitJSONL(v any) error {
	if err := json.NewEncoder(c.Stdout).Encode(v); err != nil {
		return fmt.Errorf("emit jsonl: %w", err)
	}
	return nil
}

// EmitOK writes a success Outcome carrying data.
func (c *Config) EmitOK(data any) error {
	return c.EmitJSONL(Outcome{Status: StatusOK, Data: data})
}

// EmitBlocked writes a non-fatal "blocked" Outcome.
func (c *Config) EmitBlocked(code int, reason, message string) error {
	return c.EmitJSONL(Outcome{Status: StatusBlocked, Code: code, Reason: reason, Message: message})
}

// EmitError writes an error Outcome.
func (c *Config) EmitError(code int, reason, message string) error {
	return c.EmitJSONL(Outcome{Status: StatusError, Code: code, Reason: reason, Message: message})
}

// CodeForError maps a returned error to the process exit code an error outcome
// reports when the call site set none: a validation error is a usage error (2),
// everything else is generic (1). Domain gates (4, 5–8, 12) set their own code.
// This specializes the stub climax generates, reading adh's error taxonomy.
func CodeForError(err error) int {
	if adh.ErrorCode(err) == adh.EINVALID {
		return codeUsage
	}
	return codeGeneric
}

// ReasonForError is the machine reason token for a returned error: its domain
// error code (e.g. "not_found", "invalid", "conflict"), or "internal" for an
// untyped error. It lets an agent branch on the failure class under --jsonl.
func ReasonForError(err error) string {
	return adh.ErrorCode(err)
}
