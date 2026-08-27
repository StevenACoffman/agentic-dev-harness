package critic

import (
	"strings"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/skillet/finding"
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

	// Unexamined is what a critic declared it did not look at (§19.2). Advisory by
	// construction: finding.Result.HasBlocking iterates diagnostics only, so a
	// declared gap cannot change a verdict however Dispose is written.
	Unexamined []finding.Unexamined
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
		findings, unexamined, err := ParseFindings(text)
		if err != nil {
			return Reply{}, err
		}
		return Reply{Findings: findings, Unexamined: unexamined, Text: text}, nil
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
		// **Whether is a question, not a default.** An omitted line used to become a
		// code change in stage.Apply, so an arc that never considered building silently
		// became a build -- the one thing adh's resolution vocabulary exists to make
		// answerable, since "investigation", "experiment" and "decision" already
		// express not-a-change outcomes and a decision closes with an ADR. Depth is
		// negotiable and skipping is not: answering "change" immediately is fine,
		// arriving there by not answering is not.
		//
		// Strategy runs once per arc -- rework returns to Execution, not here -- so
		// there is no legitimate second reply that could already hold a resolution and
		// reasonably omit the line.
		//
		// stage.Apply still defaults, and that is not an inconsistency: it is the mock
		// drive's only source of a resolution, because that path never parses one.
		return Reply{}, &adh.Error{
			Code: adh.EINVALID,
			Message: "strategy reply must begin with `" + resolutionPrefix +
				" <change|investigation|experiment|decision>`",
		}
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
