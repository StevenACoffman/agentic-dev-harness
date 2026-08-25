// Package session normalises foreign agent-session stores into one digest the harness
// can mine, and mines them for corrections that never became arcs.
//
// **It is deliberately not wired to anything.** `consolidate.Harvest` reads closed arcs
// and nothing here reaches it: mining sessions the harness never governed is a heuristic,
// and its output would otherwise land in the held-out splits that score a candidate edit,
// where a wrong inference does not add noise but moves the objective. The decision
// (SPEC-ADDITIONS §18 step 1) is that a signal carries provenance and only arc-derived
// signals enter the splits; the step before that is knowing how often this heuristic is
// wrong, which is what the report exists to show.
//
// Reading someone else's on-disk format will rot, so each format gets one file behind
// this package's types and a format change stays one file.
package session

import (
	"bufio"
	"encoding/json"
	"io"
	"strings"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
)

// bufCap bounds a single JSONL line. Session transcripts embed whole files in a turn,
// so the limit is well above the evidence log's.
const bufCap = 8 << 20

// Roles a normalised turn can carry.
const (
	RoleUser      = "user"
	RoleAssistant = "assistant"
)

// injected are openings of user turns the harness wrote, not a person: a task
// notification, a system reminder, a skill preamble. They arrive on the user channel and
// counting them as prompts would attribute the harness's own noise to the operator.
var injected = []string{ //nolint:gochecknoglobals // a fixed table, never mutated
	"<task-notification>", "<system-reminder>", "<command-name>",
	"caveat: the messages below were generated",
	"base directory for this skill:",
}

// Turn is one conversational turn, reduced to who spoke and what they said.
type Turn struct {
	Role string
	Text string
}

// Digest is one foreign session, normalised. Everything format-specific is resolved
// before this point, so the miners below never learn a vendor's schema.
type Digest struct {
	ID    string
	Turns []Turn
}

// claudeLine is the subset of a Claude Code transcript line this package reads.
type claudeLine struct {
	Type      string `json:"type"`
	SessionID string `json:"sessionId"`
	Message   struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	} `json:"message"`
}

// LoadClaude normalises a Claude Code transcript (one JSON object per line).
//
// Only user and assistant turns survive. A `tool_result` user turn is the harness
// replying to itself, not a person, and counting it as a prompt would make every tool
// call look like a correction — the single largest false positive available here.
//
// A malformed line is skipped rather than fatal, which is the opposite of the evidence
// log's rule and deliberately so: that log is adh's own and its corruption is a defect,
// while this is someone else's format read at a distance, and one unparseable line is
// expected rather than alarming.
func LoadClaude(r io.Reader) (Digest, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), bufCap)
	var digest Digest
	for scanner.Scan() {
		var line claudeLine
		if json.Unmarshal(scanner.Bytes(), &line) != nil {
			continue
		}
		if digest.ID == "" {
			digest.ID = line.SessionID
		}
		if line.Type != RoleUser && line.Type != RoleAssistant {
			continue
		}
		text, ok := claudeText(line.Message.Content)
		if !ok || isInjected(text) {
			continue
		}
		digest.Turns = append(digest.Turns, Turn{Role: line.Message.Role, Text: text})
	}
	if err := scanner.Err(); err != nil {
		return Digest{}, &adh.Error{Op: "session.LoadClaude", Err: err}
	}
	return digest, nil
}

// isInjected reports whether a user turn was written by the harness rather than a person.
func isInjected(text string) bool {
	lowered := strings.ToLower(strings.TrimSpace(text))
	for _, prefix := range injected {
		if strings.HasPrefix(lowered, prefix) {
			return true
		}
	}
	return false
}

// claudeText extracts the spoken text from a content field that is either a bare string
// or a list of typed blocks. It reports false for a turn with no spoken text — a
// tool_use, a tool_result, or a thinking block — so those never reach the miners.
func claudeText(raw json.RawMessage) (string, bool) {
	if len(raw) == 0 {
		return "", false
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		trimmed := strings.TrimSpace(text)
		return trimmed, trimmed != ""
	}
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &blocks) != nil {
		return "", false
	}
	var parts []string
	for i := range blocks {
		if blocks[i].Type == "text" && strings.TrimSpace(blocks[i].Text) != "" {
			parts = append(parts, strings.TrimSpace(blocks[i].Text))
		}
	}
	if len(parts) == 0 {
		return "", false
	}
	return strings.Join(parts, "\n"), true
}
