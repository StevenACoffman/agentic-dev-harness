package root_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/cmd/root"
	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
)

// newConfig builds a root.Config writing to out for envelope tests.
func newConfig(out *bytes.Buffer) *root.Config {
	return root.New(func(string) string { return "" }, strings.NewReader(""), out, &bytes.Buffer{})
}

// decodeOutcome parses the single JSONL line as an outcome envelope.
func decodeOutcome(t *testing.T, out string) map[string]any {
	t.Helper()
	var rec map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &rec); err != nil {
		t.Fatalf("outcome does not parse: %v (%q)", err, out)
	}
	return rec
}

func TestEmitOK(t *testing.T) {
	var out bytes.Buffer
	cfg := newConfig(&out)
	if err := cfg.EmitOK(map[string]string{"arc": "arc-0001"}); err != nil {
		t.Fatalf("EmitOK: %v", err)
	}
	rec := decodeOutcome(t, out.String())
	if rec["status"] != root.StatusOK || rec["code"] != 0.0 {
		t.Errorf("envelope = %v, want status ok / code 0", rec)
	}
	data, ok := rec["data"].(map[string]any)
	if !ok || data["arc"] != "arc-0001" {
		t.Errorf("data = %v, want the wrapped payload", rec["data"])
	}
}

func TestEmitBlockedAndError(t *testing.T) {
	var out bytes.Buffer
	cfg := newConfig(&out)
	if err := cfg.EmitBlocked(4, root.ReasonGate, "pass --phrase"); err != nil {
		t.Fatalf("EmitBlocked: %v", err)
	}
	blocked := decodeOutcome(t, out.String())
	if blocked["status"] != root.StatusBlocked || blocked["code"] != 4.0 ||
		blocked["reason"] != root.ReasonGate {
		t.Errorf("blocked envelope = %v", blocked)
	}
	// A blocked/error outcome carries no data field.
	if _, has := blocked["data"]; has {
		t.Errorf("blocked outcome should omit data: %v", blocked)
	}

	out.Reset()
	if err := cfg.EmitError(8, root.ReasonProof, "digest mismatch"); err != nil {
		t.Fatalf("EmitError: %v", err)
	}
	errored := decodeOutcome(t, out.String())
	if errored["status"] != root.StatusError || errored["code"] != 8.0 {
		t.Errorf("error envelope = %v", errored)
	}
}

func TestLogLevel(t *testing.T) {
	tests := []struct {
		name           string
		verbose, quiet bool
		want           slog.Level
	}{
		{"default is warn", false, false, slog.LevelWarn},
		{"verbose is debug", true, false, slog.LevelDebug},
		{"quiet is error", false, true, slog.LevelError},
		{"quiet wins over verbose", true, true, slog.LevelError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := root.LogLevel(tt.verbose, tt.quiet); got != tt.want {
				t.Errorf("LogLevel(%v, %v) = %v, want %v", tt.verbose, tt.quiet, got, tt.want)
			}
		})
	}
}

func TestNewLoggerJSONHonorsLevel(t *testing.T) {
	var out bytes.Buffer
	ctx := context.Background()
	log := root.NewLogger(&out, true, slog.LevelWarn)
	log.InfoContext(ctx, "hidden below the level")
	log.WarnContext(ctx, "shown", "op", "test")

	line := strings.TrimSpace(out.String())
	if strings.Contains(line, "hidden") {
		t.Errorf("info logged below the warn threshold: %q", line)
	}
	var rec map[string]any
	if err := json.Unmarshal([]byte(line), &rec); err != nil {
		t.Fatalf("log line is not JSON: %v (%q)", err, line)
	}
	if rec["level"] != "WARN" || rec["msg"] != "shown" || rec["op"] != "test" {
		t.Errorf("log record = %v, want a WARN line with op=test", rec)
	}
}

func TestCodeForError(t *testing.T) {
	invalid := &adh.Error{Code: adh.EINVALID, Message: "bad"}
	if got := root.CodeForError(invalid); got != 2 {
		t.Errorf("CodeForError(EINVALID) = %d, want 2", got)
	}
	other := &adh.Error{Code: adh.ENOTFOUND, Message: "gone"}
	if got := root.CodeForError(other); got != 1 {
		t.Errorf("CodeForError(ENOTFOUND) = %d, want 1", got)
	}
}

func TestEmitJSONLWritesOneCompactLine(t *testing.T) {
	var out bytes.Buffer
	cfg := root.New(func(string) string { return "" }, strings.NewReader(""), &out, &bytes.Buffer{})

	type record struct {
		Arc   string `json:"arc"`
		Stage string `json:"stage"`
	}
	if err := cfg.EmitJSONL(record{Arc: "arc-0001", Stage: "critic"}); err != nil {
		t.Fatalf("EmitJSONL: %v", err)
	}

	got := out.String()
	// Exactly one line, newline-terminated, and compact (no indentation).
	if strings.Count(got, "\n") != 1 || strings.Contains(got, "\n  ") {
		t.Errorf("output is not one compact line: %q", got)
	}
	var back map[string]string
	if err := json.Unmarshal([]byte(strings.TrimSpace(got)), &back); err != nil {
		t.Fatalf("line does not parse as JSON: %v (%q)", err, got)
	}
	if back["arc"] != "arc-0001" || back["stage"] != "critic" {
		t.Errorf("decoded = %v, want the emitted fields", back)
	}
}

// TestEmitJSONLMultipleRecords confirms that N calls produce N independently
// parseable lines — the list-command contract.
func TestEmitJSONLMultipleRecords(t *testing.T) {
	var out bytes.Buffer
	cfg := root.New(func(string) string { return "" }, strings.NewReader(""), &out, &bytes.Buffer{})

	for _, id := range []string{"arc-0001", "arc-0002"} {
		if err := cfg.EmitJSONL(map[string]string{"arc": id}); err != nil {
			t.Fatalf("EmitJSONL: %v", err)
		}
	}
	lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2: %q", len(lines), out.String())
	}
	for _, line := range lines {
		var back map[string]string
		if err := json.Unmarshal([]byte(line), &back); err != nil {
			t.Errorf("line %q does not parse: %v", line, err)
		}
	}
}
