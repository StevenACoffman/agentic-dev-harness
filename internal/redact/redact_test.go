package redact_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/internal/redact"
)

func newRedactor(t *testing.T) *redact.Redactor {
	t.Helper()
	r, err := redact.New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return r
}

func TestRedactReplacesSecret(t *testing.T) {
	t.Parallel()
	r := newRedactor(t)
	// A GitHub personal-access-token shape (ghp_ + 36 chars) the default ruleset
	// detects on its prefix — a realistic value, not an allowlisted docs example.
	const token = "ghp_A1b2C3d4E5f6G7h8I9j0K1l2M3n4O5p6Q7r8" //nolint:gosec // synthetic test fixture, not a real credential
	got := r.Redact("github_token = " + token)
	if strings.Contains(got, token) {
		t.Errorf("secret survived redaction: %q", got)
	}
	if !strings.Contains(got, redact.Placeholder) {
		t.Errorf("redacted text %q has no %s placeholder", got, redact.Placeholder)
	}
	// The surrounding context is preserved.
	if !strings.Contains(got, "github_token") {
		t.Errorf("redaction removed context, not just the secret: %q", got)
	}
}

func TestRedactLeavesBenignTextUnchanged(t *testing.T) {
	t.Parallel()
	r := newRedactor(t)
	for _, in := range []string{"", "just some ordinary prose with no credentials"} {
		if got := r.Redact(in); got != in {
			t.Errorf("Redact(%q) = %q, want it unchanged", in, got)
		}
	}
}
