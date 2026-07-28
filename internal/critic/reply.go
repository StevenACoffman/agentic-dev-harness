package critic

import (
	"strings"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
)

// resolutionPrefix marks the optional leading line by which a strategy reply
// chooses the arc's resolution (§12): "resolution: <word>".
const resolutionPrefix = "resolution:"

// Reply is a relayed stage reply after validation (§19.2). Findings is set only
// for a critic reply; Resolution only when a strategy reply chose one; Text is
// what the stage records in its history. A Reply is produced only after the reply
// passed its stage's contract, so a malformed answer never advances an arc.
type Reply struct {
	Findings   []adh.Finding
	Resolution adh.Resolution
	Text       string
}

// ParseReply validates a relayed reply for the arc's current stage and extracts
// what that stage needs (§19.2). An empty reply is always EINVALID. A critic reply
// must be findings JSON (ParseFindings). A strategy reply may begin with a
// "resolution: <word>" line that chooses the resolution (§12) — an unknown word is
// EINVALID, and no such line leaves it unset so the loop defaults it to a change.
// Every other stage carries prose. It is pure: it neither reads I/O nor mutates.
func ParseReply(stg adh.Stage, text string) (Reply, error) {
	if strings.TrimSpace(text) == "" {
		return Reply{}, &adh.Error{Code: adh.EINVALID, Message: "empty relay reply"}
	}
	switch stg {
	case adh.StageCritic:
		findings, err := ParseFindings(text)
		if err != nil {
			return Reply{}, err
		}
		return Reply{Findings: findings, Text: text}, nil
	case adh.StageStrategy:
		return parseStrategyReply(text)
	default:
		return Reply{Text: text}, nil
	}
}

// parseStrategyReply reads an optional leading "resolution: <word>" line, choosing
// the arc's resolution (§12) and dropping that line from the recorded plan text.
func parseStrategyReply(text string) (Reply, error) {
	const op = "critic.parseStrategyReply"
	first, rest, split := strings.Cut(text, "\n")
	after, ok := strings.CutPrefix(strings.TrimSpace(first), resolutionPrefix)
	if !ok {
		return Reply{Text: text}, nil
	}
	res, err := adh.ParseResolution(strings.TrimSpace(after))
	if err != nil {
		return Reply{}, &adh.Error{Op: op, Err: err}
	}
	plan := ""
	if split {
		plan = strings.TrimSpace(rest)
	}
	return Reply{Resolution: res, Text: plan}, nil
}
