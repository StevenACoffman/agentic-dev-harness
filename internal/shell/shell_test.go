package shell_test

import (
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
