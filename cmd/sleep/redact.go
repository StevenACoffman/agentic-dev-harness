package sleep

import (
	"fmt"

	"github.com/StevenACoffman/agentic-dev-harness/internal/consolidate"
	"github.com/StevenACoffman/agentic-dev-harness/internal/redact"
)

// redactor removes secrets from staged text (§18.4). It is the point-of-use seam:
// the real one wraps internal/redact (betterleaks' ruleset); a test injects a fake.
type redactor interface {
	Redact(text string) string
}

// secretRedactor returns the configured redactor, building the real betterleaks
// one when none was injected. Redaction is always on for staged text — the safe
// default (§18.5 redact = true); an opt-out is a deferred follow-up.
func (cfg *Config) secretRedactor() (redactor, error) {
	if cfg.redactor != nil {
		return cfg.redactor, nil
	}
	r, err := redact.New()
	if err != nil {
		return nil, fmt.Errorf("sleep: %w", err)
	}
	return r, nil
}

// redactCycle scrubs secrets from every free-text field a cycle stages: the
// proposed artifact, the longitudinal guidance, and each evidence note. The
// scores and identifiers around them are not secret-bearing.
func redactCycle(r redactor, cycle *consolidate.Cycle) {
	cycle.Proposed = r.Redact(cycle.Proposed)
	cycle.SlowGuidance = r.Redact(cycle.SlowGuidance)
	for i := range cycle.Records {
		cycle.Records[i].Note = r.Redact(cycle.Records[i].Note)
	}
}
