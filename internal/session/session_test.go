package session_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/internal/session"
)

// wantTurns loads a transcript and asserts the roles that survived normalisation, which
// is where every format-specific decision shows up. Extracted before the table so each
// case reads as a claim about what counts as a turn.
func wantTurns(t *testing.T, transcript string, want []string) {
	t.Helper()
	digest, err := session.LoadClaude(strings.NewReader(transcript))
	if err != nil {
		t.Fatalf("LoadClaude: %v", err)
	}
	got := make([]string, 0, len(digest.Turns))
	for _, turn := range digest.Turns {
		got = append(got, turn.Role)
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("roles = %v, want %v", got, want)
	}
}

func TestLoadClaudeKeepsOnlySpokenTurns(t *testing.T) {
	t.Parallel()
	const (
		userStr   = `{"type":"user","sessionId":"s1","message":{"role":"user","content":"hello"}}`
		asstBlock = `{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"hi"}]}}`
		toolUse   = `{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","name":"Bash"}]}}`
		toolResp  = `{"type":"user","message":{"role":"user","content":[{"type":"tool_result","content":"out"}]}}`
		thinking  = `{"type":"assistant","message":{"role":"assistant","content":[{"type":"thinking","thinking":"hm"}]}}`
		notif     = `{"type":"user","message":{"role":"user","content":"<task-notification>done</task-notification>"}}`
		other     = `{"type":"file-history-snapshot","sessionId":"s1"}`
	)
	tests := []struct {
		name       string
		transcript string
		want       []string
	}{{
		name:       "a string turn and a text block both count",
		transcript: userStr + "\n" + asstBlock,
		want:       []string{"user", "assistant"},
	}, {
		// The harness replying to itself. Counting it would make every tool call look
		// like a prompt, which is the largest false positive available here.
		name:       "a tool_result user turn is not a person speaking",
		transcript: strings.Join([]string{userStr, asstBlock, toolResp}, "\n"),
		want:       []string{"user", "assistant"},
	}, {
		name:       "tool_use and thinking carry no spoken text",
		transcript: strings.Join([]string{userStr, toolUse, thinking}, "\n"),
		want:       []string{"user"},
	}, {
		// Arrives on the user channel and was written by the harness.
		name:       "an injected notification is not a prompt",
		transcript: strings.Join([]string{userStr, asstBlock, notif}, "\n"),
		want:       []string{"user", "assistant"},
	}, {
		name:       "non-conversational line types are skipped",
		transcript: other + "\n" + userStr,
		want:       []string{"user"},
	}, {
		// Someone else's format read at a distance: one unparseable line is expected,
		// not a corruption of adh's own data.
		name:       "a malformed line is skipped rather than fatal",
		transcript: "{not json" + "\n" + userStr,
		want:       []string{"user"},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			wantTurns(t, tt.transcript, tt.want)
		})
	}
}

// wantRetries asserts how many corrective turns a sequence of turns yields.
func wantRetries(t *testing.T, turns []session.Turn, want int) {
	t.Helper()
	got := session.Retries(session.Digest{ID: "s1", Turns: turns})
	if len(got) != want {
		t.Errorf("retries = %d (%v), want %d", len(got), got, want)
	}
}

func TestRetriesNeedsAnAssistantTurnFirst(t *testing.T) {
	t.Parallel()
	user := func(s string) session.Turn { return session.Turn{Role: session.RoleUser, Text: s} }
	asst := session.Turn{Role: session.RoleAssistant, Text: "done"}
	tests := []struct {
		name  string
		turns []session.Turn
		want  int
	}{{
		name:  "a correction after an assistant turn counts",
		turns: []session.Turn{user("do it"), asst, user("no, that's the wrong file")},
		want:  1,
	}, {
		// Without requiring the assistant turn, a session opening with "no" would
		// register as a correction of nothing.
		name:  "an opening turn is not a correction",
		turns: []session.Turn{user("no, use the other file")},
		want:  0,
	}, {
		name:  "a new instruction is not a correction",
		turns: []session.Turn{user("do it"), asst, user("now do the next one")},
		want:  0,
	}, {
		// The marker must open the turn: "no" inside prose is ordinary English.
		name:  "a marker mid-sentence does not count",
		turns: []session.Turn{user("do it"), asst, user("there is no rush on this")},
		want:  0,
	}, {
		name:  "matching is case-insensitive",
		turns: []session.Turn{user("do it"), asst, user("Not quite — try the other one")},
		want:  1,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			wantRetries(t, tt.turns, tt.want)
		})
	}
}

func TestSummariseTalliesByMarker(t *testing.T) {
	t.Parallel()
	asst := session.Turn{Role: session.RoleAssistant, Text: "done"}
	digest := session.Digest{ID: "s1", Turns: []session.Turn{
		{Role: session.RoleUser, Text: "do it"},
		asst,
		{Role: session.RoleUser, Text: "no, wrong file"},
		asst,
		{Role: session.RoleUser, Text: "no, still wrong"},
	}}
	rep := session.Summarise([]session.Digest{digest})
	if rep.Sessions != 1 || rep.Turns != 5 {
		t.Errorf("sessions/turns = %d/%d, want 1/5", rep.Sessions, rep.Turns)
	}
	if rep.ByMarker["no,"] != 2 {
		t.Errorf("by_marker[no,] = %d, want 2", rep.ByMarker["no,"])
	}
}
