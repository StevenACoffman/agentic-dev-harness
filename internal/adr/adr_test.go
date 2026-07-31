package adr_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/adr"
)

// TestValid: a filled ADR is valid; the raw skeleton (placeholders remain) and one
// missing a section are not — so an undelivered decision cannot ship as proof.
func TestValid(t *testing.T) {
	skeleton := adr.Render("use pgx", "we compared database drivers")
	if err := adr.Valid(skeleton); err == nil {
		t.Error("raw skeleton should be invalid (unfilled placeholders)")
	}
	filled := skeleton
	for _, ph := range []string{
		"<state the decision now settled>",
		"<what this improves>",
		"<the trade-off accepted>",
	} {
		filled = strings.Replace(filled, ph, "done", 1)
	}
	if err := adr.Valid(filled); err != nil {
		t.Errorf("filled ADR should be valid: %v", err)
	}
	if err := adr.Valid(
		"# Decision: x\n\n**Status:** Accepted\n",
	); adh.ErrorCode(
		err,
	) != adh.EINVALID {
		t.Errorf("ADR missing sections should be EINVALID, got %v", err)
	}
}
