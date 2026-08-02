// Package adh is the foundational domain package for the agentic-dev-harness
// CLI: the shared language every other package speaks. It holds the core domain
// types and the effectful interfaces, and re-exports the shared Error type and
// its codes from skillet/errs so adh code keeps speaking adh.Error while the one
// definition lives in the shared library. It imports no sibling application
// package; every other package imports it.
package adh

import "github.com/StevenACoffman/skillet/errs"

// Error codes are machine-readable classifications set on leaf errors,
// re-exported from skillet/errs.
const (
	ECONFLICT     = errs.ECONFLICT     // action cannot be performed in the current state
	EINTERNAL     = errs.EINTERNAL     // an unexpected internal error
	EINVALID      = errs.EINVALID      // input or state failed validation
	ENOTFOUND     = errs.ENOTFOUND     // requested entity does not exist
	EUNAUTHORIZED = errs.EUNAUTHORIZED // caller lacks the required authority
)

// Error is the shared error type (skillet/errs.Error), re-exported so adh code
// and its callers keep using adh.Error while the single definition lives in
// skillet/errs. A leaf error carries Code and Message; a wrapping error carries
// Op and Err.
type Error = errs.Error

// ErrorCode returns the machine-readable code of the first *Error in the chain
// that carries one, EINTERNAL for any other non-nil error, and "" for nil.
func ErrorCode(err error) string { return errs.ErrorCode(err) }

// ErrorMessage returns the human-readable message of the first *Error in the
// chain that carries one, a generic message for any other non-nil error, and ""
// for nil.
func ErrorMessage(err error) string { return errs.ErrorMessage(err) }
