package session

import (
	"sort"
	"strings"
)

// markers are the openings a person uses when the previous attempt was wrong.
//
// **Deliberately narrow, and anchored to the start of a turn.** "no" appears inside
// ordinary prose constantly; "no, " opening a reply after an assistant turn is a
// correction. The rule errs toward finding fewer: an undercount is a weak signal, an
// overcount is a wrong one, and this exists to be measured before it is trusted.
var markers = []string{ //nolint:gochecknoglobals // a fixed table, never mutated
	"no,", "no.", "nope", "that's wrong", "thats wrong", "that is wrong",
	"not quite", "not right", "incorrect", "wrong ", "actually,", "still ",
	"you missed", "you forgot", "try again", "revert", "undo",
	"doesn't work", "does not work", "didn't work", "did not work",
	"that broke", "this broke", "regression",
}

// Retry is one detected correction: a user turn that answered an assistant turn by
// saying it was wrong.
type Retry struct {
	Session string
	// Turn is the index in the digest, so a reader can find it in the transcript.
	Turn int
	// Marker is which opening matched, so the report can show which rule fired and a
	// bad rule can be identified rather than merely suspected.
	Marker string
	// Prompt is the corrective turn, truncated.
	Prompt string
}

// Report is what a corpus of sessions yields, before anything is done with it.
type Report struct {
	Sessions int            `json:"sessions"`
	Turns    int            `json:"turns"`
	Retries  []Retry        `json:"retries"`
	ByMarker map[string]int `json:"by_marker"`
}

// Retries finds corrective turns in one digest. Pure.
//
// The rule: a user turn that directly follows an assistant turn and opens with a
// negative-feedback marker. Requiring the assistant turn first is what distinguishes *the
// last attempt was wrong* from a user simply beginning a message with "no" — without it,
// the opening turn of a session can match.
//
// **Known false positives, stated because the next reader's instinct is to trust the
// count.** A user quoting an error message that opens with one of these words; a
// correction aimed at the user's own previous instruction rather than at the assistant;
// and "still " matching "still running" as readily as "still broken". **Known false
// negatives:** any correction phrased politely, which is most of them. The count is a
// lower bound on corrections and an upper bound on nothing.
func Retries(d Digest) []Retry {
	out := make([]Retry, 0)
	for i := 1; i < len(d.Turns); i++ {
		if d.Turns[i].Role != RoleUser || d.Turns[i-1].Role != RoleAssistant {
			continue
		}
		if marker, ok := opensWithMarker(d.Turns[i].Text); ok {
			out = append(out, Retry{
				Session: d.ID,
				Turn:    i,
				Marker:  marker,
				Prompt:  truncate(d.Turns[i].Text),
			})
		}
	}
	return out
}

// Summarise folds digests into one report. Pure; the caller reads the files.
func Summarise(digests []Digest) Report {
	rep := Report{
		Sessions: len(digests),
		Retries:  make([]Retry, 0),
		ByMarker: make(map[string]int),
	}
	for i := range digests {
		rep.Turns += len(digests[i].Turns)
		for _, r := range Retries(digests[i]) {
			rep.Retries = append(rep.Retries, r)
			rep.ByMarker[r.Marker]++
		}
	}
	sort.Slice(rep.Retries, func(i, j int) bool {
		if rep.Retries[i].Session != rep.Retries[j].Session {
			return rep.Retries[i].Session < rep.Retries[j].Session
		}
		return rep.Retries[i].Turn < rep.Retries[j].Turn
	})
	return rep
}

// opensWithMarker reports whether text begins with a negative-feedback marker.
func opensWithMarker(text string) (string, bool) {
	lowered := strings.ToLower(strings.TrimSpace(text))
	for _, m := range markers {
		if strings.HasPrefix(lowered, m) {
			return strings.TrimSpace(m), true
		}
	}
	return "", false
}

// truncate bounds a prompt so a report stays readable.
func truncate(s string) string {
	const limit = 120
	flat := strings.Join(strings.Fields(s), " ")
	if len(flat) <= limit {
		return flat
	}
	return flat[:limit] + "…"
}
