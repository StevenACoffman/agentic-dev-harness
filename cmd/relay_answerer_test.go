package cmd_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/state"
	"github.com/StevenACoffman/agentic-dev-harness/internal/vcs"
)

// answerFromPrompt is the scripted answerer: a pure function from the emitted prompt to
// a reply, which is what a "scripted model" means where the seam is a file rather than a
// socket. adh invokes no model — relay.Emit parks a prompt and `--response` feeds a reply
// back — so there is no protocol to speak and no server to start.
//
// **It derives every reply from the prompt and refuses when it cannot**, which is the
// whole point. A fixture that returns hardcoded replies is a playback: it would pass
// against an empty prompt, so it asserts the parser and says nothing about whether the
// prompt carries what the next stage needs. Deriving inverts that — the reply exists only
// if the request was complete, so a prompt regression fails the test instead of passing
// quietly.
func answerFromPrompt(stage adh.Stage, prompt string) (string, error) {
	switch stage {
	case adh.StageStrategy:
		return answerStrategy(prompt)
	case adh.StageCritic:
		return answerCritic(prompt)
	case adh.StageExecution, adh.StageEvaluation, adh.StageOps:
		// Prose stages: quote a token from the prompt so an empty one is detectable.
		id, err := arcIDIn(prompt)
		if err != nil {
			return "", err
		}
		return "completed " + string(stage) + " for " + id, nil
	default:
		return "", fmt.Errorf("answerer: no script for stage %q", stage)
	}
}

// answerStrategy picks a resolution the prompt actually offered.
//
// It refuses rather than defaulting to "change": the resolution line is required
// (critic.parseStrategyReply), so a prompt that stops offering the vocabulary must break
// the arc rather than be papered over by an answerer that knows the answer anyway.
func answerStrategy(prompt string) (string, error) {
	offered := []string{"investigation", "experiment", "decision", "change"}
	for _, res := range offered {
		if !strings.Contains(prompt, res) {
			return "", fmt.Errorf(
				"answerer: the strategy prompt no longer offers %q, so no resolution "+
					"can be chosen from it", res)
		}
	}
	id, err := arcIDIn(prompt)
	if err != nil {
		return "", err
	}
	return "resolution: investigation\nread the logs for " + id, nil
}

// answerCritic emits findings JSON, and refuses if the prompt did not ground the review.
func answerCritic(prompt string) (string, error) {
	if !strings.Contains(prompt, "Acceptance bar") {
		return "", errors.New("answerer: the critic prompt carries no acceptance bar")
	}
	if _, err := arcIDIn(prompt); err != nil {
		return "", err
	}
	// A clean review. The finding vocabulary is exercised elsewhere; what this asserts
	// is that a reply derived from the prompt is one the next stage accepts.
	return `{"findings":[]}`, nil
}

// arcIDIn extracts the arc id the prompt is about, so a reply cannot be written without
// having read one.
func arcIDIn(prompt string) (string, error) {
	i := strings.Index(prompt, "arc-")
	if i < 0 {
		return "", errors.New("answerer: the prompt names no arc")
	}
	id := prompt[i:]
	if j := strings.IndexAny(id, " \n\t.,;:—"); j > 0 {
		id = id[:j]
	}
	return id, nil
}

// relayOneStage emits the arc's current prompt, answers it from that prompt, and feeds
// the reply back. It returns the prompt it answered so a caller can assert on it.
func relayOneStage(t *testing.T, id string) string {
	t.Helper()
	out, runErr := run(t, "step", "--relay", "--jsonl", id)
	if runErr != nil {
		t.Fatalf("emit: %v\nstdout: %s", runErr, out)
	}
	emitted := decodeStep(t, out)
	if emitted.Status != "awaiting" {
		t.Fatalf("emit at %s = %q, want awaiting", emitted.Stage, emitted.Status)
	}
	reply, err := answerFromPrompt(adh.Stage(emitted.Stage), emitted.Prompt)
	if err != nil {
		t.Fatalf("the answerer could not answer the %s prompt: %v\n%s",
			emitted.Stage, err, emitted.Prompt)
	}
	path := filepath.Join(t.TempDir(), "reply.txt")
	if err := os.WriteFile(path, []byte(reply), 0o600); err != nil {
		t.Fatalf("write reply: %v", err)
	}
	mustRun(t, "step", "--relay", "--response", path, "--jsonl", id)
	return emitted.Prompt
}

// TestRelayJourneyWithAScriptedAnswerer drives an arc through the relay with replies
// derived from the real emitted prompts.
//
// The existing relay tests hand `--response` a hardcoded string, which asserts the parser
// accepts a reply and would pass if the prompt were empty. This asserts the prompt is
// answerable — that each stage emits enough for the next one to be satisfied — which is
// the gap the entry describes as "nothing establishes that an agent handed a real emitted
// prompt produces a reply the next stage accepts".
func TestRelayJourneyWithAScriptedAnswerer(t *testing.T) {
	t.Chdir(t.TempDir())
	// Execution grounds from a diff, so the arc needs a real repository to walk past
	// strategy.
	if _, err := vcs.Init("."); err != nil {
		t.Fatalf("git init: %v", err)
	}
	mustRun(t, "init")
	id := strings.TrimSpace(mustRun(t, "arc", "new", "widen the column"))

	strategyPrompt := relayOneStage(t, id)
	if !strings.Contains(strategyPrompt, "resolution:") {
		t.Error("the strategy prompt no longer asks for a resolution line")
	}
	arc, err := state.Default().Get(id)
	if err != nil {
		t.Fatalf("get arc: %v", err)
	}
	// The answerer chose investigation from the prompt's own vocabulary; if the arc
	// holds "change" instead, the reply was ignored and the default applied.
	if arc.Resolution != adh.ResolutionInvestigation {
		t.Fatalf("resolution = %q, want the one the answerer chose", arc.Resolution)
	}
	if arc.Stage != adh.StageExecution {
		t.Fatalf("stage after strategy = %q, want execution", arc.Stage)
	}

	// Walk on. Each stage's prompt must be answerable from itself; the critic's is the
	// one that carries grounding, so reaching it is what makes this more than a
	// strategy test.
	for arc.Stage != adh.StageOps && arc.Status == adh.StatusOpen {
		before := arc.Stage
		if arc.Stage == adh.StageEvaluation {
			// Evaluation is not relayed: it adjudicates the critic's findings against
			// repository artifacts rather than asking anyone anything. `step --relay`
			// says so and points here, which is itself part of the contract.
			mustRun(t, "eval", id)
		} else {
			relayOneStage(t, id)
		}
		if arc, err = state.Default().Get(id); err != nil {
			t.Fatalf("get arc: %v", err)
		}
		if arc.Stage == before {
			t.Fatalf("arc stuck at %s after a relayed reply", before)
		}
	}
	if arc.Stage != adh.StageOps {
		t.Errorf("arc reached %s (%s), want the ops gate", arc.Stage, arc.Status)
	}
}

// TestScriptedAnswererRefusesABlindedPrompt is the control, and it asserts the
// *fixture's* honesty rather than adh's behaviour.
//
// Without it, an answerer that quietly stopped reading the prompt would keep passing
// forever, and the journey above would degrade into the playback it exists to replace.
// It is the same relationship `oracle selftest` has to the selection gate: prove the
// instrument discriminates before trusting what it says.
func TestScriptedAnswererRefusesABlindedPrompt(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		stage  adh.Stage
		prompt string
	}{{
		name:  "a strategy prompt that stops offering the vocabulary",
		stage: adh.StageStrategy, prompt: "Plan the work for arc arc-0001.",
	}, {
		name:  "a critic prompt with no acceptance bar",
		stage: adh.StageCritic, prompt: "Review arc arc-0001 in a cold context.",
	}, {
		name:  "any prompt that names no arc",
		stage: adh.StageExecution, prompt: "Do the thing.",
	}, {
		name:  "an empty prompt",
		stage: adh.StageStrategy, prompt: "",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if reply, err := answerFromPrompt(tt.stage, tt.prompt); err == nil {
				t.Errorf("the answerer produced %q from a blinded prompt; it is not "+
					"reading the prompt, so the journey test proves nothing", reply)
			}
		})
	}
}

// TestRelayHistoryRecordsStagesInOrder is the ordering discipline, from the third
// method: not only that the required step happened, but that nothing later happened
// first. Ordering is not a property a reply can be trusted to report about itself.
func TestRelayHistoryRecordsStagesInOrder(t *testing.T) {
	t.Chdir(t.TempDir())
	mustRun(t, "init")
	id := strings.TrimSpace(mustRun(t, "arc", "new", "widen the column"))
	relayOneStage(t, id)

	arc, err := state.Default().Get(id)
	if err != nil {
		t.Fatalf("get arc: %v", err)
	}
	order := map[string]int{"strategy": 0, "execution": 1, "critic": 2, "evaluation": 3}
	last := -1
	for _, entry := range arc.History {
		stage, _, found := strings.Cut(entry, ":")
		rank, known := order[stage]
		if !found || !known {
			continue
		}
		if rank < last {
			t.Errorf("history records %s after a later stage:\n%v", stage, arc.History)
		}
		last = rank
	}
}
