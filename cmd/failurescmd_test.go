package cmd_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/internal/failures"
)

func TestFailuresListEmpty(t *testing.T) {
	t.Chdir(t.TempDir())
	out, err := run(t, "failures", "list")
	if err != nil {
		t.Fatalf("failures list: %v", err)
	}
	if !strings.Contains(out, "no failures recorded") {
		t.Errorf("empty registry = %q, want the empty notice", out)
	}
}

func TestFailuresListGroupsByClass(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := failures.Append(
		failures.RegistryFile,
		"contract: proof missing",
		"contract: digest mismatch",
	); err != nil {
		t.Fatalf("seed registry: %v", err)
	}
	if err := failures.Append(failures.CandidatesFile, "oracle: suspected divergence"); err != nil {
		t.Fatalf("seed candidates: %v", err)
	}
	out, err := run(t, "failures", "list")
	if err != nil {
		t.Fatalf("failures list: %v", err)
	}
	if !strings.Contains(out, "contract") || !strings.Contains(out, "(2 instance(s))") {
		t.Errorf("confirmed class/count missing:\n%s", out)
	}
	if !strings.Contains(out, "oracle") {
		t.Errorf("candidate class missing:\n%s", out)
	}
}
