package consolidate_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/consolidate"
	"github.com/StevenACoffman/agentic-dev-harness/internal/gate"
)

// realisticArtifact is a few hundred bytes — the size a real guiding doc runs —
// so a bounded learned block fits within the 1.5× size budget (§18.2). A
// toy-sized artifact would trip the budget on any real edit.
const realisticArtifact = "# Skill\n\n" +
	"Read the target's instructions and architecture before acting.\n" +
	"Inspect the domain model, tests, and permissions for a local owner.\n" +
	"Name the governing decision that local evidence leaves unresolved.\n" +
	"Let target-local truth govern the implementation.\n" +
	"Verify the result through the target's native checks.\n"

// selectionClass returns the first synthetic failure class that hashes to the
// selection split, so a test can seed an arc whose task is held out for
// acceptance. It is deterministic — no clock, no randomness.
func selectionClass(t *testing.T) string {
	t.Helper()
	for i := range 200 {
		class := fmt.Sprintf("missing-widget-%d", i)
		if consolidate.SplitFor(class) == consolidate.SplitSelection {
			return class
		}
	}
	t.Fatal("no synthetic class hashed to the selection split")
	return ""
}

func closedArc(id, failure string) adh.Arc {
	return adh.Arc{
		ID:      id,
		Status:  adh.StatusClosed,
		History: []string{"critic: " + failure},
	}
}

func TestHarvestClosedOnly(t *testing.T) {
	arcs := []adh.Arc{
		closedArc("arc-0001", "missing boundary"),
		{ID: "arc-0002", Status: adh.StatusOpen, History: []string{"critic: missing boundary"}},
	}
	signals := consolidate.Harvest(arcs)
	if len(signals) != 1 {
		t.Fatalf("Harvest kept %d signals, want 1 (closed only)", len(signals))
	}
	if signals[0].Arc != "arc-0001" || len(signals[0].Failures) != 1 {
		t.Errorf("Harvest signal = %+v, want arc-0001 with one failure", signals[0])
	}
}

func TestMineOneTaskPerClass(t *testing.T) {
	signals := []consolidate.Signal{
		{Arc: "a", Failures: []string{"missing boundary", "missing boundary"}},
		{Arc: "b", Failures: []string{"error handling gap"}},
	}
	tasks := consolidate.Mine(signals, consolidate.DefaultConfig())
	if len(tasks) != 2 {
		t.Fatalf("Mine produced %d tasks, want 2 distinct classes", len(tasks))
	}
	for i := range tasks {
		if len(tasks[i].Checks) != 1 {
			t.Errorf("task %q has %d checks, want 1", tasks[i].ID, len(tasks[i].Checks))
		}
	}
}

func TestSplitStable(t *testing.T) {
	for _, id := range []string{"alpha", "beta", "gamma", "missing-widget-0"} {
		first := consolidate.SplitFor(id)
		second := consolidate.SplitFor(id)
		if first != second {
			t.Errorf("SplitFor(%q) not stable: %q then %q", id, first, second)
		}
		switch first {
		case consolidate.SplitSelection, consolidate.SplitTest, consolidate.SplitTrain:
		default:
			t.Errorf("SplitFor(%q) = %q, not a known split", id, first)
		}
	}
}

func TestPlanAcceptsImprovement(t *testing.T) {
	class := selectionClass(t)
	arcs := []adh.Arc{closedArc("arc-0001", class), closedArc("arc-0002", class)}
	artifact := realisticArtifact
	learned := consolidate.Propose(consolidate.Harvest(arcs), consolidate.DefaultConfig())

	cycle, err := consolidate.Plan(artifact, learned, arcs, nil, consolidate.DefaultConfig())
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if cycle.Decision.Action == gate.Reject {
		t.Fatalf("Plan rejected an improving candidate: %+v", cycle.Decision)
	}
	if cycle.Proposed == "" || cycle.StagingID == "" {
		t.Errorf(
			"accepted cycle staged nothing: proposed=%q id=%q",
			cycle.Proposed,
			cycle.StagingID,
		)
	}
	if !strings.Contains(cycle.Proposed, class) {
		t.Errorf("proposed artifact does not contain the guarded class %q", class)
	}
	if cycle.Candidate <= cycle.Baseline {
		t.Errorf("candidate %.3f did not beat baseline %.3f", cycle.Candidate, cycle.Baseline)
	}
}

func TestPlanRejectsNoImprovement(t *testing.T) {
	// A closed arc with no failure in its history yields no task and no proposal.
	arcs := []adh.Arc{
		{ID: "arc-0001", Status: adh.StatusClosed, History: []string{"critic: looks good"}},
	}
	cycle, err := consolidate.Plan("# Skill\n", "", arcs, nil, consolidate.DefaultConfig())
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if cycle.Proposed != "" {
		t.Errorf("Plan staged %q with nothing to learn", cycle.Proposed)
	}
	if len(cycle.Records) != 1 || cycle.Records[0].Note == "" {
		t.Errorf("a no-improvement cycle must self-explain, got %+v", cycle.Records)
	}
}

func TestPlanRejectsOverBudget(t *testing.T) {
	class := selectionClass(t)
	arcs := []adh.Arc{closedArc("arc-0001", class)}
	cfg := consolidate.DefaultConfig()
	cfg.SizeRatio = 0.1 // any learned block dwarfs 10% of a tiny artifact
	learned := consolidate.Propose(consolidate.Harvest(arcs), cfg)

	cycle, err := consolidate.Plan("# S\n", learned, arcs, nil, cfg)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if cycle.Proposed != "" {
		t.Errorf("over-budget candidate was staged: %q", cycle.Proposed)
	}
	if !strings.Contains(cycle.Records[0].Note, "size budget") {
		t.Errorf("note = %q, want a size-budget explanation", cycle.Records[0].Note)
	}
}

func TestPlanSkipsRejectedCandidate(t *testing.T) {
	class := selectionClass(t)
	arcs := []adh.Arc{closedArc("arc-0001", class), closedArc("arc-0002", class)}
	artifact := realisticArtifact
	learned := consolidate.Propose(consolidate.Harvest(arcs), consolidate.DefaultConfig())

	first, err := consolidate.Plan(artifact, learned, arcs, nil, consolidate.DefaultConfig())
	if err != nil {
		t.Fatalf("Plan (first): %v", err)
	}
	if first.StagingID == "" {
		t.Fatal("first Plan did not stage a candidate to reject")
	}
	rejected := map[string]bool{first.StagingID: true}
	second, err := consolidate.Plan(artifact, learned, arcs, rejected, consolidate.DefaultConfig())
	if err != nil {
		t.Fatalf("Plan (second): %v", err)
	}
	if second.Proposed != "" {
		t.Errorf("a previously rejected candidate was re-proposed: %q", second.Proposed)
	}
	if !strings.Contains(second.Records[0].Note, "rejected") {
		t.Errorf("note = %q, want a rejected-buffer explanation", second.Records[0].Note)
	}
}
