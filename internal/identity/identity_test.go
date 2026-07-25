package identity_test

import (
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/internal/identity"
)

func TestHashLength(t *testing.T) {
	got := identity.Hash("anything")
	if len(got) != 16 {
		t.Errorf("Hash length = %d, want 16", len(got))
	}
}

func TestHashDeterministicAndDistinct(t *testing.T) {
	a1 := identity.Hash("# skill\nrule one\n")
	a2 := identity.Hash("# skill\nrule one\n")
	b := identity.Hash("# skill\nrule two\n")
	if a1 != a2 {
		t.Errorf("Hash not deterministic: %q != %q", a1, a2)
	}
	if a1 == b {
		t.Errorf("distinct content produced the same hash %q", a1)
	}
}

func TestHashKnownVector(t *testing.T) {
	// sha256("") = e3b0c44298fc1c14..., first 16 hex chars below.
	if got := identity.Hash(""); got != "e3b0c44298fc1c14" {
		t.Errorf("Hash(\"\") = %q, want e3b0c44298fc1c14", got)
	}
}
