// Package adr is the architecture-decision-record format (SPEC-ADDITIONS §12): the
// durable, routable home for a team's local trade-off decisions — Status, Context,
// Decision, and Consequences split into Easier and Harder. It is both the skeleton
// a promoted `decision` lesson renders and the structural proof a decision-
// resolution arc closes with. Both Render and Valid are pure.
package adr

import (
	"fmt"
	"strings"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
)

// Placeholder tokens the skeleton emits for the parts a human must fill in. Valid
// rejects any that remain, so an unfilled skeleton cannot ship as proof (§12).
const (
	placeholderDecision = "<state the decision now settled>"
	placeholderEasier   = "<what this improves>"
	placeholderHarder   = "<the trade-off accepted>"
)

// Render returns an ADR skeleton (§12): Status / Context / Decision / Consequences
// split into Easier and Harder. contextBody is the forcing context the caller
// supplies; the Decision and its trade-offs are left as placeholders a human
// completes before the ADR is a valid proof. It is pure.
func Render(title, contextBody string) string {
	var b strings.Builder
	_, _ = fmt.Fprintf(&b, "# Decision: %s\n\n**Status:** Accepted\n\n## Context\n\n", title)
	b.WriteString(strings.TrimRight(contextBody, "\n"))
	b.WriteString("\n\n## Decision\n\n" + placeholderDecision + "\n\n")
	b.WriteString("## Consequences\n\n### Easier\n\n" + placeholderEasier + "\n\n")
	b.WriteString("### Harder\n\n" + placeholderHarder + "\n")
	return b.String()
}

// Valid reports whether text is a complete ADR: it carries a Status and the Context,
// Decision, and Consequences (Easier + Harder) sections, and no skeleton placeholder
// remains (§12). It is the structural proof a `decision`-resolution arc closes with —
// an unfilled skeleton is rejected so an undelivered decision cannot ship. It returns
// EINVALID naming the first defect.
func Valid(text string) error {
	required := []struct{ token, name string }{
		{"Status:", "a Status"},
		{"## Context", "a Context section"},
		{"## Decision", "a Decision section"},
		{"## Consequences", "a Consequences section"},
		{"### Easier", "an Easier consequence"},
		{"### Harder", "a Harder consequence"},
	}
	for _, req := range required {
		if !strings.Contains(text, req.token) {
			return &adh.Error{Code: adh.EINVALID, Message: "ADR is missing " + req.name}
		}
	}
	for _, placeholder := range []string{placeholderDecision, placeholderEasier, placeholderHarder} {
		if strings.Contains(text, placeholder) {
			return &adh.Error{
				Code:    adh.EINVALID,
				Message: "ADR still has an unfilled placeholder: " + placeholder,
			}
		}
	}
	return nil
}
