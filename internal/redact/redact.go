// Package redact removes secrets from text before it is persisted (SPEC-ADDITIONS
// §18.4): the sleep consolidation cycle harvests arc history and stages guiding
// text that can carry credentials, so every staged string passes through here
// first. Detection is offloaded to betterleaks' maintained ruleset — this package
// is only the thin wrapper that replaces each detected secret with a placeholder.
package redact

import (
	"fmt"
	"strings"

	"github.com/betterleaks/betterleaks/detect"
)

// Placeholder replaces a detected secret in redacted text.
const Placeholder = "[REDACTED]"

// Redactor detects and removes secrets using betterleaks' embedded default
// ruleset. Build one with New and reuse it; Redact holds no per-call state.
type Redactor struct {
	detector *detect.Detector
}

// New builds a Redactor over betterleaks' default ruleset. The detector is
// constructed once (it compiles the embedded rules); reuse the Redactor.
func New() (*Redactor, error) {
	detector, err := detect.NewDetectorDefaultConfig()
	if err != nil {
		return nil, fmt.Errorf("redact: load detector: %w", err)
	}
	return &Redactor{detector: detector}, nil
}

// Redact replaces every detected secret in text with Placeholder, returning text
// unchanged when nothing is detected. It replaces the matched secret value, so the
// surrounding context (e.g. the key name) is preserved for a reader.
func (r *Redactor) Redact(text string) string {
	if text == "" {
		return text
	}
	findings := r.detector.DetectString(text)
	for i := range findings {
		secret := findings[i].Secret
		if secret == "" {
			continue
		}
		text = strings.ReplaceAll(text, secret, Placeholder)
	}
	return text
}
