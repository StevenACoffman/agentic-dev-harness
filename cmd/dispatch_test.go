package cmd_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/peterbourgon/ff/v4"
)

// TestUnknownSubcommandFails is the base-scaffold guard: a mistyped subcommand on
// a group parent must fail (non-zero exit) rather than masquerade as a bare
// invocation. Without the guard ff returns ff.ErrNoExec, which the CLI maps to
// exit 0 — so a typo would silently succeed.
func TestUnknownSubcommandFails(t *testing.T) {
	t.Parallel()
	_, err := run(t, "bogus")
	if err == nil {
		t.Fatal("adh bogus returned nil; want an unknown-subcommand error")
	}
	if errors.Is(err, ff.ErrNoExec) {
		t.Fatal("adh bogus reported ff.ErrNoExec; a typo must not look like a bare invocation")
	}
	if !strings.Contains(err.Error(), "unknown subcommand") {
		t.Errorf("adh bogus error = %q, want it to mention \"unknown subcommand\"", err)
	}
}

// TestUnknownSubcommandOnNestedGroup guards a mistyped verb under a real group
// parent — proof has ff subcommands (create/verify), so `proof bogus` is the same
// unmatched-token case one level down.
func TestUnknownSubcommandOnNestedGroup(t *testing.T) {
	t.Parallel()
	_, err := run(t, "proof", "bogus")
	if err == nil || !strings.Contains(err.Error(), "unknown subcommand") {
		t.Fatalf("adh proof bogus = %v, want an unknown-subcommand error", err)
	}
}

// TestBareInvocationIsNoOp keeps the contract the guard must not break: a bare
// invocation (no subcommand) is ff.ErrNoExec, which the CLI maps to exit 0 — the
// guard fires only on a leftover token, never here.
func TestBareInvocationIsNoOp(t *testing.T) {
	t.Parallel()
	_, err := run(t)
	if !errors.Is(err, ff.ErrNoExec) {
		t.Fatalf("bare adh = %v, want ff.ErrNoExec (the exit-0 no-op path)", err)
	}
}
