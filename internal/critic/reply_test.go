package critic_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/critic"
)

func TestParseReplyEmptyIsInvalid(t *testing.T) {
	for _, stg := range []adh.Stage{adh.StageStrategy, adh.StageExecution, adh.StageCritic} {
		if _, err := critic.ParseReply(stg, "  \n "); adh.ErrorCode(err) != adh.EINVALID {
			t.Errorf("ParseReply(%s, empty) = %v, want EINVALID", stg, err)
		}
	}
}

func TestParseReplyCriticFindings(t *testing.T) {
	reply := `{"findings":[{"summary":"clears differ","kind":"oracle","ref":"corpus"}]}`
	got, err := critic.ParseReply(adh.StageCritic, reply)
	if err != nil {
		t.Fatalf("ParseReply: %v", err)
	}
	if len(got.Findings) != 1 || got.Findings[0].Kind != adh.FindingOracle {
		t.Errorf("findings = %+v, want one oracle finding", got.Findings)
	}
	if got.Resolution != "" {
		t.Errorf("critic reply set a resolution: %q", got.Resolution)
	}
}

func TestParseReplyCriticMalformedIsInvalid(t *testing.T) {
	_, err := critic.ParseReply(adh.StageCritic, "not findings json")
	if adh.ErrorCode(err) != adh.EINVALID {
		t.Errorf("malformed critic reply = %v, want EINVALID", err)
	}
}

func TestParseReplyStrategyChoosesResolution(t *testing.T) {
	reply := "resolution: investigation\nlook at the crash logs and report"
	got, err := critic.ParseReply(adh.StageStrategy, reply)
	if err != nil {
		t.Fatalf("ParseReply: %v", err)
	}
	if got.Resolution != adh.ResolutionInvestigation {
		t.Errorf("resolution = %q, want investigation", got.Resolution)
	}
	// The resolution line is dropped from the recorded plan text.
	if got.Text != "look at the crash logs and report" {
		t.Errorf("plan text = %q, want the resolution line stripped", got.Text)
	}
}

func TestParseReplyStrategyUnknownResolutionIsInvalid(t *testing.T) {
	_, err := critic.ParseReply(adh.StageStrategy, "resolution: teleport\nplan")
	if adh.ErrorCode(err) != adh.EINVALID {
		t.Errorf("unknown resolution = %v, want EINVALID", err)
	}
}

// wantStrategyRefusal asserts a strategy reply is refused as EINVALID and that the
// message tells the author which mistake they made. Extracted before the table because
// the two refusals are easy to confuse: one means the line is missing, the other that
// the word on it is not a resolution.
func wantStrategyRefusal(t *testing.T, reply, wantIn string) {
	t.Helper()
	_, err := critic.ParseReply(adh.StageStrategy, reply)
	if adh.ErrorCode(err) != adh.EINVALID {
		t.Fatalf("ParseReply(%q) = %v, want EINVALID", reply, err)
	}
	if !strings.Contains(err.Error(), wantIn) {
		t.Errorf("message %q does not contain %q", err, wantIn)
	}
}

// TestParseReplyStrategyRequiresAResolution replaces a test that asserted the opposite.
//
// A plain plan used to parse, leaving the resolution unset for stage.Apply to default to
// a code change — so an arc that never considered *whether* to build silently became a
// build. The resolution vocabulary already expresses not-a-change outcomes, and a
// decision closes on a written ADR; what was missing is that the question had to be
// answered. Depth is negotiable, skipping is not.
func TestParseReplyStrategyRequiresAResolution(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		reply  string
		wantIn string
	}{{
		name:  "a plain plan is refused and says what is missing",
		reply: "just widen the column", wantIn: "must begin with",
	}, {
		name:  "the refusal names the resolutions that would satisfy it",
		reply: "just widen the column", wantIn: "investigation",
	}, {
		// Distinct from the above: the author reached for the line and mistyped the
		// word, and telling them "must begin with" would send them to fix the wrong
		// thing.
		name:  "a mistyped resolution is a different refusal",
		reply: "resolution: teleport\nplan", wantIn: "unknown resolution",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			wantStrategyRefusal(t, tt.reply, tt.wantIn)
		})
	}
}

// TestParseReplyStrategyTolerantOfLeadingSpace is separate from the refusal table
// because it asserts the opposite outcome, and a table with a sentinel selecting
// between "refused" and "accepted" reads as machinery rather than as either claim.
func TestParseReplyStrategyTolerantOfLeadingSpace(t *testing.T) {
	t.Parallel()
	got, err := critic.ParseReply(adh.StageStrategy, "  resolution: decision\nrecord it")
	if err != nil {
		t.Fatalf("ParseReply: %v", err)
	}
	if got.Resolution != adh.ResolutionDecision {
		t.Errorf("resolution = %q, want decision", got.Resolution)
	}
	if got.Text != "record it" {
		t.Errorf("plan text = %q, want the resolution line stripped", got.Text)
	}
}

func TestParseReplyExecutionIsProse(t *testing.T) {
	got, err := critic.ParseReply(adh.StageExecution, "widened the column to 64 chars")
	if err != nil {
		t.Fatalf("ParseReply: %v", err)
	}
	if got.Text != "widened the column to 64 chars" || got.Resolution != "" || got.Findings != nil {
		t.Errorf("execution reply = %+v, want prose only", got)
	}
}
