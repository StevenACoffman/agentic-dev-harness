// Package adh is the foundational domain package for the agentic-dev-harness
// CLI: the shared language every other package speaks. It holds the core domain
// types, the effectful interfaces, and the Error type with its code vocabulary.
// It imports no sibling application package and no third-party library; every
// other package imports it.
package adh

import (
	"errors"
	"strings"
)

// Error codes are machine-readable classifications set on leaf errors. They are
// the vocabulary the CLI's `reason` token is drawn from (SPEC §8), so a caller
// branches on these rather than on message text.
const (
	ECONFLICT     = "conflict"     // action cannot be performed in the current state
	EINTERNAL     = "internal"     // an unexpected internal error
	EINVALID      = "invalid"      // input or state failed validation
	ENOTFOUND     = "not_found"    // requested entity does not exist
	EUNAUTHORIZED = "unauthorized" // caller lacks the required authority
)

// Error is adh's one error type. A leaf error carries Code and Message; a
// wrapping error carries Op and Err. The two forms are never mixed on one value:
// leaves classify, wrappers build a single-line logical stack trace.
//
// **It is defined here rather than borrowed.** It used to be `= skillet/errs.Error`,
// an alias kept in the shared library for adh's benefit alone — no other consumer of
// that library ever imported the package. A type with one consumer belongs to that
// consumer, and the layout this repository follows puts the Error type in the root
// domain package beside the types it classifies failures for.
//
// A plain struct, deliberately: 203 sites compose it as a literal, and the typed CLI
// refusals in cmd/root reach it through Unwrap, which needs a concrete type for
// errors.As to find.
type Error struct {
	Code    string // machine-readable; set only on leaf errors
	Message string // human-readable; set only on leaf errors
	Op      string // "package.Type.Method"; set only on wrapping errors
	Err     error  // nested cause; set only on wrapping errors
}

// Error renders the logical stack trace: each wrapper contributes its Op, and
// the leaf contributes its Message (or Code when Message is empty).
func (e *Error) Error() string {
	var b strings.Builder
	if e.Op != "" {
		b.WriteString(e.Op)
	}
	switch {
	case e.Err != nil:
		if e.Op != "" {
			b.WriteString(": ")
		}
		b.WriteString(e.Err.Error())
	case e.Message != "":
		if e.Op != "" {
			b.WriteString(": ")
		}
		b.WriteString(e.Message)
	case e.Code != "":
		if e.Op != "" {
			b.WriteString(": ")
		}
		b.WriteString(e.Code)
	}
	return b.String()
}

// Unwrap exposes the nested cause so errors.Is and errors.As traverse the chain.
func (e *Error) Unwrap() error { return e.Err }

// ErrorCode returns the machine-readable code of the first *Error in the chain
// that carries one, EINTERNAL for any other non-nil error, and "" for nil.
// Call this instead of type-asserting *Error at a call site.
//
// **It classifies adh's own errors and nothing else, which is narrower than it was.**
// The predecessor also understood errors coded by the toerr `errcode` package, because
// it lived in the shared library and had to read both representations. Keeping that
// here would put a third-party import in the domain package, which is the one thing
// this package does not do — so a library error classifies as EINTERNAL, and the
// translation happens where such errors enter adh. Today that is cmd/root, which owns
// the `reason` vocabulary; see reasonForCodedError there.
func ErrorCode(err error) string {
	if err == nil {
		return ""
	}
	var e *Error
	if errors.As(err, &e) {
		if e.Code != "" {
			return e.Code
		}
		if e.Err != nil {
			return ErrorCode(e.Err)
		}
	}
	return EINTERNAL
}

// ErrorMessage returns the human-readable message of the first *Error in the
// chain that carries one, a generic message for any other non-nil error, and ""
// for nil. It has the same narrowed scope as ErrorCode.
func ErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	var e *Error
	if errors.As(err, &e) {
		if e.Message != "" {
			return e.Message
		}
		if e.Err != nil {
			return ErrorMessage(e.Err)
		}
	}
	return "an internal error occurred"
}
