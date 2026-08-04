package oracle

import "strings"

// CommandDivergence is the first line at which two command outputs differ (§2.1): the
// 1-based line number and both sides' text. It is the general form of the board oracle's
// divergence — two implementations that should agree are each other's oracle, so any
// difference convicts one of them.
type CommandDivergence struct {
	Line      int
	Reference string
	Candidate string
}

// DiffOutputs compares two command outputs line by line and returns the first
// divergence, or nil when they match (§2.1): the pure core of the command-level
// differential oracle. A line present on only one side diverges against the empty
// string; trailing newlines are ignored so a command that adds one still matches. It is
// pure — the caller runs the commands and captures their output.
func DiffOutputs(reference, candidate string) *CommandDivergence {
	ref := splitOutputLines(reference)
	cand := splitOutputLines(candidate)
	for i := range max(len(ref), len(cand)) {
		r, c := lineAt(ref, i), lineAt(cand, i)
		if r != c {
			return &CommandDivergence{Line: i + 1, Reference: r, Candidate: c}
		}
	}
	return nil
}

// splitOutputLines splits command output into lines, ignoring a single trailing
// newline so equal outputs are not judged different for it. Empty output is no lines.
func splitOutputLines(output string) []string {
	trimmed := strings.TrimSuffix(output, "\n")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "\n")
}

// lineAt returns the line at index i, or the empty string when i is past the end.
func lineAt(lines []string, i int) string {
	if i < len(lines) {
		return lines[i]
	}
	return ""
}
