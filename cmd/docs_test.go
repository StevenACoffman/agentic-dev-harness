package cmd_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDocsToStdout: `adh docs` writes the enriched root man page to stdout.
func TestDocsToStdout(t *testing.T) {
	out := mustRun(t, "docs")
	for _, want := range []string{".TH", "run", "EXIT STATUS", "REASON TOKENS", "docs"} {
		if !strings.Contains(out, want) {
			t.Errorf("docs stdout missing %q", want)
		}
	}
}

// TestDocsWritesPerCommandPages: `adh docs --dir <tmp>` writes the root page and
// one page per registered subcommand into the directory.
func TestDocsWritesPerCommandPages(t *testing.T) {
	dir := t.TempDir()
	if _, err := run(t, "docs", "--dir", dir); err != nil {
		t.Fatalf("docs --dir: %v", err)
	}
	// The root page and a representative subcommand's page both land in the dir.
	for _, name := range []string{"adh.1", "adh-run.1", "adh-eval.1"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("expected %s to be written: %v", name, err)
		}
	}
	// A per-command page documents that command; the root page carries the reference.
	rootPage, err := os.ReadFile(filepath.Join(dir, "adh.1"))
	if err != nil {
		t.Fatalf("read adh.1: %v", err)
	}
	if !strings.Contains(string(rootPage), "EXIT STATUS") {
		t.Errorf("root page missing the EXIT STATUS reference")
	}
}
