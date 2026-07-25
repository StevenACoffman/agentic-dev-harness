package cmd_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/cmd"
	"github.com/StevenACoffman/agentic-dev-harness/cmd/root"
)

func TestHarnessEvalDispatch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "skill.md")
	doc := "# Skill\nIf the build fails, retry.\n## Boundary\nNot for zero-to-one work.\n"
	if err := os.WriteFile(path, []byte(doc), 0o600); err != nil {
		t.Fatalf("write artifact: %v", err)
	}
	out, err := run(t, "harness", "eval", path)
	if err != nil {
		t.Fatalf("harness eval returned error: %v", err)
	}
	if !strings.Contains(out, "det score: 100.0/100") {
		t.Errorf("harness eval output = %q, want a full det score", out)
	}
}

func TestHarnessEvalMinFloorPasses(t *testing.T) {
	path := filepath.Join(t.TempDir(), "skill.md")
	doc := "# Skill\nIf the build fails, retry.\n## Boundary\nNot for zero-to-one.\n"
	if err := os.WriteFile(path, []byte(doc), 0o600); err != nil {
		t.Fatalf("write artifact: %v", err)
	}
	if _, err := run(t, "harness", "--min", "50", "eval", path); err != nil {
		t.Errorf("eval --min 50 on a 100-scoring doc should pass, got %v", err)
	}
}

func TestHarnessEvalMinFloorFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "skill.md")
	doc := "# Skill\nIf the build fails, retry.\n## Boundary\nNot for zero-to-one.\n"
	if err := os.WriteFile(path, []byte(doc), 0o600); err != nil {
		t.Fatalf("write artifact: %v", err)
	}
	_, err := run(t, "harness", "--min", "101", "eval", path)
	var exit root.ExitError
	if !errors.As(err, &exit) || int(exit) != 1 {
		t.Fatalf("eval below --min floor = %v, want ExitError(1)", err)
	}
}

func TestHarnessUnknownVerb(t *testing.T) {
	if _, err := run(t, "harness", "frobnicate"); err == nil {
		t.Errorf("unknown harness verb should return an error")
	}
}

func run(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var out, errb bytes.Buffer
	err := cmd.Run(context.Background(), args, strings.NewReader(""), &out, &errb)
	return out.String(), err
}

func TestGateAcceptDispatch(t *testing.T) {
	out, err := run(t, "gate", "--candidate", "90", "--current", "84")
	if err != nil {
		t.Fatalf("gate accept returned error: %v", err)
	}
	if !strings.Contains(out, "accept_new_best") {
		t.Errorf("gate accept output = %q, want accept_new_best", out)
	}
}

func TestGateRejectExitCode(t *testing.T) {
	_, err := run(t, "gate", "--candidate", "80", "--current", "84")
	var exit root.ExitError
	if !errors.As(err, &exit) || int(exit) != 1 {
		t.Fatalf("gate reject error = %v, want ExitError(1)", err)
	}
}

func TestOracleSelfTestDispatch(t *testing.T) {
	out, err := run(t, "oracle", "selftest")
	if err != nil {
		t.Fatalf("oracle selftest returned error: %v", err)
	}
	if !strings.Contains(out, "passed") {
		t.Errorf("oracle selftest output = %q, want a pass", out)
	}
}

func TestUnknownVerb(t *testing.T) {
	if _, err := run(t, "arc", "frobnicate"); err == nil {
		t.Errorf("unknown arc verb should return an error")
	}
}
