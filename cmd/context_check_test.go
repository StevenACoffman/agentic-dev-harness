package cmd_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/internal/contextstore"
)

// writeUnit writes a context unit plus its content file into the store.
func writeUnit(t *testing.T, id, kind, content string, labels ...string) {
	t.Helper()
	if err := os.MkdirAll(contextstore.DefaultStoreDir, 0o750); err != nil {
		t.Fatalf("mkdir store: %v", err)
	}
	contentFile := id + ".md"
	if err := os.WriteFile(
		filepath.Join(contextstore.DefaultStoreDir, contentFile),
		[]byte(content),
		0o600,
	); err != nil {
		t.Fatalf("write content: %v", err)
	}
	unit := contextstore.Unit{ID: id, Kind: kind, Labels: labels, ContentPath: contentFile}
	data, _ := json.MarshalIndent(unit, "", "  ")
	if err := os.WriteFile(
		filepath.Join(contextstore.DefaultStoreDir, id+".json"),
		data,
		0o600,
	); err != nil {
		t.Fatalf("write unit: %v", err)
	}
}

// TestContextCheckAssemblesPacket: `context check` gathers the routed units' content
// into one consistency-review packet the relayed agent adjudicates.
func TestContextCheckAssemblesPacket(t *testing.T) {
	t.Chdir(t.TempDir())
	writeUnit(t, "rule", "base-rule", "Always fail secure.", "security")
	writeUnit(t, "skill", "skill", "Prefer fail open for availability.", "security")
	out := mustRun(t, "context", "check")
	for _, want := range []string{"fail secure", "fail open", "rule", "skill", "contradictions"} {
		if !strings.Contains(out, want) {
			t.Errorf("check packet missing %q:\n%s", want, out)
		}
	}
}
