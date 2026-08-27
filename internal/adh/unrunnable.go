package adh

// Why a named artifact did not run (§19.2).
//
// UnrunnableUnset is the zero value and names no reason, so an Adjudicated nobody
// populated does not claim one. It is also what a finding that *did* run carries,
// which is correct: there is no reason, because there was no failure to explain.
const (
	UnrunnableUnset Unrunnable = ""

	// UnrunnableNoRef: the finding named no artifact at all. The critic asserted
	// something and pointed at nothing, so there was never anything to run. Noise
	// rather than a refusal.
	UnrunnableNoRef Unrunnable = "no-ref"

	// UnrunnableUnknownRef: the finding named an artifact the repository does not
	// declare. Also noise — and distinct from NoRef, because it is a critic naming a
	// tool that never existed rather than one omitting the field.
	UnrunnableUnknownRef Unrunnable = "unknown-ref"

	// UnrunnableToolFailed: the finding named a **registered** tool and that tool
	// could not start — missing binary, unbuilt, wrong commit.
	//
	// This is the honest refusal, and the only one of the three that is evidence
	// about the repository rather than about the critic. §19.2's blocking decision
	// was deferred because the three were indistinguishable; they are not.
	UnrunnableToolFailed Unrunnable = "tool-failed"
)

// Unrunnable is why an artifact named by a finding did not run.
type Unrunnable string

// Trustworthy reports whether this reason is a refusal by the repository rather
// than a defect in the finding that named it.
//
// Requires: nothing.
// Ensures: true only for UnrunnableToolFailed. The zero value is false, so a reason
// nobody set never reads as a trustworthy refusal.
//
// It exists as a method rather than a comparison at each call site because it is the
// predicate §19.2's blocking decision would turn on if it is ever revisited, and one
// place should own what "trustworthy" means when that happens.
func (u Unrunnable) Trustworthy() bool { return u == UnrunnableToolFailed }
