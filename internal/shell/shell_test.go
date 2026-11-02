package shell_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/internal/shell"
)

func TestRun(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		command  string
		wantCode int
		wantRan  bool
	}{
		{"clean exit", "exit 0", 0, true},
		{"non-zero exit ran", "exit 3", 3, true},
		{"failing command ran", "false", 1, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			code, ran := shell.Runner{}.Run(context.Background(), tt.command, "")
			if code != tt.wantCode || ran != tt.wantRan {
				t.Errorf("Run(%q) = (%d, %v), want (%d, %v)",
					tt.command, code, ran, tt.wantCode, tt.wantRan)
			}
		})
	}
}

// TestRunIOCapturesStreams: RunIO wires the child's stdout and stderr to the
// given writers, so a caller can surface or capture a declared tool's output.
func TestRunIOCapturesStreams(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	code, ran := shell.Runner{}.RunIO(
		context.Background(),
		"echo out; echo err 1>&2; exit 2",
		"",
		&stdout,
		&stderr,
	)
	if code != 2 || !ran {
		t.Errorf("RunIO = (%d, %v), want (2, true)", code, ran)
	}
	if got := stdout.String(); got != "out\n" {
		t.Errorf("stdout = %q, want %q", got, "out\n")
	}
	if got := stderr.String(); got != "err\n" {
		t.Errorf("stderr = %q, want %q", got, "err\n")
	}
}

// TestRunCanceledDidNotRun: a command that cannot start (here, an already-canceled
// context) reports ran=false with exitCode -1 — not a non-zero exit.
func TestRunCanceledDidNotRun(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	code, ran := shell.Runner{}.Run(ctx, "exit 0", "")
	if ran || code != -1 {
		t.Errorf("canceled Run = (%d, %v), want (-1, false)", code, ran)
	}
}

// TestExecArgvBypassesTheShell: an argv command runs with no shell, so shell syntax in
// an argument is passed through literally rather than interpreted.
func TestExecArgvBypassesTheShell(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		cmd      shell.Command
		wantOut  string
		wantCode int
		wantRan  bool
	}{
		"argv passes shell syntax literally": {
			cmd:     shell.Command{Argv: []string{"echo", "$(id)", "|", "sh"}},
			wantOut: "$(id) | sh\n", wantRan: true,
		},
		"argv non-zero exit ran": {
			cmd:      shell.Command{Argv: []string{"false"}},
			wantCode: 1, wantRan: true,
		},
		"argv missing binary did not run": {
			cmd:      shell.Command{Argv: []string{"adh-no-such-binary"}},
			wantCode: -1,
		},
		"script still runs through the shell": {
			cmd:     shell.Command{Script: "echo a | tr a b"},
			wantOut: "b\n", wantRan: true,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var out bytes.Buffer
			code, ran := shell.Runner{}.Exec(context.Background(), tc.cmd, "", &out, nil)
			if code != tc.wantCode || ran != tc.wantRan || out.String() != tc.wantOut {
				t.Errorf("Exec(%v) = (%d, %v, %q), want (%d, %v, %q)",
					tc.cmd, code, ran, out.String(), tc.wantCode, tc.wantRan, tc.wantOut)
			}
		})
	}
}

func TestCommandString(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		cmd  shell.Command
		want string
	}{
		"script": {shell.Command{Script: "go test ./... | tee log"}, "go test ./... | tee log"},
		"argv":   {shell.Command{Argv: []string{"go", "test", "./..."}}, "go test ./..."},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := tc.cmd.String(); got != tc.want {
				t.Errorf("String() = %q, want %q", got, tc.want)
			}
		})
	}
}
