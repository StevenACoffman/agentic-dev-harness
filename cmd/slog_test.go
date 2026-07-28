package cmd_test

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/cmd"
)

// runStderr runs a command through cmd.Run and returns what it wrote to the
// diagnostic stream (stderr), discarding the stdout data plane.
func runStderr(t *testing.T, args ...string) string {
	t.Helper()
	var out, errb bytes.Buffer
	_ = cmd.Run(context.Background(), args, func(string) string { return "" },
		strings.NewReader(""), &out, &errb)
	return errb.String()
}

// findLog returns the first stderr line whose parsed JSON has msg, or "" if none.
// The diagnostic stream is JSON under --jsonl, so each line is a log record.
func findLog(t *testing.T, stderr, msg string) map[string]any {
	t.Helper()
	for _, line := range strings.Split(strings.TrimSpace(stderr), "\n") {
		if line == "" {
			continue
		}
		var rec map[string]any
		if err := json.Unmarshal([]byte(line), &rec); err == nil && rec["msg"] == msg {
			return rec
		}
	}
	return nil
}

func TestRunStageAdvanceLogsUnderVerbose(t *testing.T) {
	t.Chdir(t.TempDir())
	seedOpenArc(t) // arc-0001 at strategy/open

	// --verbose unlocks Info: the stage-advance trace lands on stderr as JSON,
	// while stdout (the data plane) is unaffected.
	rec := findLog(t, runStderr(t, "--jsonl", "--verbose", "run", "arc-0001"), "stage advanced")
	if rec == nil {
		t.Fatal("no stage-advance log under --verbose")
	}
	if rec["level"] != "INFO" || rec["op"] != "run" || rec["arc"] != "arc-0001" {
		t.Errorf("stage-advance log = %v, want INFO op=run arc=arc-0001", rec)
	}
}

func TestRunStageAdvanceHiddenAtDefaultLevel(t *testing.T) {
	t.Chdir(t.TempDir())
	seedOpenArc(t)

	// Default level is Warn, so Info traces stay off the diagnostic stream.
	if rec := findLog(t, runStderr(t, "--jsonl", "run", "arc-0001"), "stage advanced"); rec != nil {
		t.Errorf("info trace leaked at the default level: %v", rec)
	}
}
