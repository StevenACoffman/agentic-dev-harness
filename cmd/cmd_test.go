package cmd_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/cmd"
	"github.com/StevenACoffman/agentic-dev-harness/cmd/root"
)

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
