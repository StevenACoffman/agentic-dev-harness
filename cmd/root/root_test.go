package root_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/cmd/root"
)

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
