package critic_test

import (
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

func TestParseReplyStrategyPlainPlanLeavesResolutionUnset(t *testing.T) {
	got, err := critic.ParseReply(adh.StageStrategy, "just widen the column")
	if err != nil {
		t.Fatalf("ParseReply: %v", err)
	}
	if got.Resolution != "" {
		t.Errorf("resolution = %q, want unset for a plain plan", got.Resolution)
	}
	if got.Text != "just widen the column" {
		t.Errorf("plan text = %q, want the whole reply", got.Text)
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
