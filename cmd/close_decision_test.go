package cmd_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/cmd/root"
)

const filledADR = `# Decision: adopt pgx

**Status:** Accepted

## Context

We compared database drivers for the service.

## Decision

Adopt pgx with sqlc.

## Consequences

### Easier

Typed queries and fewer round trips.

### Harder

PostgreSQL only.
`

// TestCloseDecisionWithADR: a decision-resolution arc closes when its proof is a
// complete ADR (§12) — the ADR is the proof, not a hash manifest.
func TestCloseDecisionWithADR(t *testing.T) {
	t.Chdir(t.TempDir())
	id := parkedAtOps(t)
	adrPath := filepath.Join(t.TempDir(), "adr.md")
	if err := os.WriteFile(adrPath, []byte(filledADR), 0o600); err != nil {
		t.Fatalf("write adr: %v", err)
	}
	mustRun(t, "close", "--as", "decision", "--proof", adrPath, id)
}

// TestCloseDecisionRejectsSkeleton: an unfilled ADR skeleton is not proof — the
// close fails NO-PROOF-NO-CLOSE (exit 8), so an undocumented decision cannot ship.
func TestCloseDecisionRejectsSkeleton(t *testing.T) {
	t.Chdir(t.TempDir())
	id := parkedAtOps(t)
	adrPath := filepath.Join(t.TempDir(), "adr.md")
	skeleton := "# Decision: x\n\n**Status:** Accepted\n\n## Context\n\nc\n\n## Decision\n\n<state the decision now settled>\n\n## Consequences\n\n### Easier\n\ne\n\n### Harder\n\nh\n"
	if err := os.WriteFile(adrPath, []byte(skeleton), 0o600); err != nil {
		t.Fatalf("write adr: %v", err)
	}
	_, err := run(t, "close", "--as", "decision", "--proof", adrPath, id)
	var exit root.ExitError
	if !errors.As(err, &exit) || int(exit) != 8 {
		t.Fatalf("close decision with a skeleton ADR = %v, want ExitError(8)", err)
	}
}
