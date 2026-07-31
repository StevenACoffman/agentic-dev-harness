// Package adh is the foundational domain package for the agentic-dev-harness
// CLI: the shared language every other package speaks. It holds the Error type
// and its codes, the core domain types, and the effectful interfaces. It imports
// no sibling application package; every other package imports it.
package adh

import (
	"errors"
	"strings"
)

// Error codes are machine-readable classifications set on leaf errors. Start
// with these five and add more only as a real need appears.
const (
	ECONFLICT     = "conflict"     // action cannot be performed in the current state
	EINTERNAL     = "internal"     // an unexpected internal error
	EINVALID      = "invalid"      // input or state failed validation
	ENOTFOUND     = "not_found"    // requested entity does not exist
	EUNAUTHORIZED = "unauthorized" // caller lacks the required authority
)

// Error is the one error type for the application. A leaf error carries Code and
// Message; a wrapping error carries Op and Err. The two forms are never mixed on
// one value: leaves classify, wrappers build a single-line logical stack trace.
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
// for nil.
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
