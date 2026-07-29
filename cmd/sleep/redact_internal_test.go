package sleep

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/internal/consolidate"
	"github.com/StevenACoffman/agentic-dev-harness/internal/evidence"
)

// sentinelRedactor replaces the literal "SENSITIVE" so redaction is exercised
// without depending on the real ruleset (that path is covered in internal/redact).
type sentinelRedactor struct{}

func (sentinelRedactor) Redact(text string) string {
	return strings.ReplaceAll(text, "SENSITIVE", "[REDACTED]")
}

func TestSecretRedactorReturnsInjected(t *testing.T) {
	cfg := &Config{redactor: sentinelRedactor{}}
	got, err := cfg.secretRedactor()
	if err != nil {
		t.Fatalf("secretRedactor: %v", err)
	}
	if _, ok := got.(sentinelRedactor); !ok {
		t.Errorf("secretRedactor returned %T, want the injected fake", got)
	}
}

// TestRedactionScrubsStagedFiles: redactCycle + writeStaging leave no secret in the
// staged proposal, longitudinal guidance, evidence, or report.
func TestRedactionScrubsStagedFiles(t *testing.T) {
	t.Chdir(t.TempDir())
	cycle := &consolidate.Cycle{
		StagingID:    "0001",
		Proposed:     "guidance: use token SENSITIVE when calling the API",
		SlowGuidance: "recurring: SENSITIVE showed up again",
		Records: []evidence.Record{
			{Arc: "arc-0001", Status: evidence.StatusKeep, Note: "leaked SENSITIVE in review"},
		},
	}
	redactCycle(sentinelRedactor{}, cycle)
	if err := writeStaging("harness.md", cycle); err != nil {
		t.Fatalf("writeStaging: %v", err)
	}

	dir := filepath.Join(stagingRoot, "0001")
	for _, name := range []string{"harness.md", "longitudinal.md", "report.json", "evidence.jsonl"} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if strings.Contains(string(data), "SENSITIVE") {
			t.Errorf("%s still contains the secret:\n%s", name, data)
		}
		if !strings.Contains(string(data), "[REDACTED]") {
			t.Errorf("%s was not redacted:\n%s", name, data)
		}
	}
}
