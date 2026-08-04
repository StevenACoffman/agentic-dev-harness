package consolidate_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/consolidate"
	"github.com/StevenACoffman/agentic-dev-harness/internal/verdict"
	gate "github.com/StevenACoffman/skillet/ratchet"
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

func TestReflectFailureWinsConflict(t *testing.T) {
	signals := []consolidate.Signal{
		{Failures: []string{"flaky retries"}, Successes: []string{"flaky retries", "clean merge"}},
	}
	r := consolidate.Reflect(signals)
	if len(r.Failure) != 1 || r.Failure[0].Class != "flaky retries" {
		t.Errorf("failure modes = %+v, want [flaky retries]", r.Failure)
	}
	for _, m := range r.Success {
		if m.Class == "flaky retries" {
			t.Errorf("class present as both — failure must win, got success mode %q", m.Class)
		}
	}
}

func TestReflectRanksByRecurrence(t *testing.T) {
	signals := []consolidate.Signal{
		{Failures: []string{"rare gap", "common gap", "common gap", "common gap"}},
	}
	r := consolidate.Reflect(signals)
	if len(r.Failure) != 2 || r.Failure[0].Class != "common gap" || r.Failure[0].Count != 3 {
		t.Errorf("ranking = %+v, want common gap (3) first", r.Failure)
	}
}

func TestProposeClipsToBudget(t *testing.T) {
	signals := []consolidate.Signal{
		{Failures: []string{"a fail", "b fail", "c fail", "d fail", "e fail"}},
	}
	cfg := consolidate.DefaultConfig() // budget 4
	out := consolidate.Propose(signals, cfg)
	if got := strings.Count(out, "- Guard against:"); got != cfg.EditBudget {
		t.Errorf("proposed %d bullets, want the budget %d", got, cfg.EditBudget)
	}
}

func TestBudgetDecaysWithRound(t *testing.T) {
	signals := []consolidate.Signal{
		{Failures: []string{"a fail", "b fail", "c fail", "d fail"}},
	}
	cfg := consolidate.DefaultConfig() // budget 4
	tests := []struct {
		round, want int
	}{
		{round: 0, want: 4},  // full budget
		{round: 2, want: 2},  // 4 - 2
		{round: 10, want: 1}, // floored at 1
	}
	for _, tt := range tests {
		cfg.Round = tt.round
		if got := strings.Count(
			consolidate.Propose(signals, cfg),
			"- Guard against:",
		); got != tt.want {
			t.Errorf("round %d: proposed %d bullets, want %d", tt.round, got, tt.want)
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

func TestPlanReportsSoftScores(t *testing.T) {
	class := selectionClass(t)
	arcs := []adh.Arc{closedArc("arc-0001", class), closedArc("arc-0002", class)}
	learned := consolidate.Propose(consolidate.Harvest(arcs), consolidate.DefaultConfig())
	cycle, err := consolidate.Plan(
		realisticArtifact,
		learned,
		arcs,
		nil,
		consolidate.DefaultConfig(),
	)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	// A single-check task makes per-task soft == hard, so the split's mean soft
	// tracks its mean hard: the baseline misses the assertion, the candidate meets it.
	if cycle.BaselineSoft != 0 || cycle.CandidateSoft != 1 {
		t.Errorf("soft scores = base %.3f cand %.3f, want 0 then 1",
			cycle.BaselineSoft, cycle.CandidateSoft)
	}
}

func TestPlanSoftMetricAccepts(t *testing.T) {
	class := selectionClass(t)
	arcs := []adh.Arc{closedArc("arc-0001", class), closedArc("arc-0002", class)}
	cfg := consolidate.DefaultConfig()
	cfg.Metric = gate.Soft
	learned := consolidate.Propose(consolidate.Harvest(arcs), cfg)
	cycle, err := consolidate.Plan(realisticArtifact, learned, arcs, nil, cfg)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if cycle.Decision.Action == gate.Reject {
		t.Errorf("soft-metric gate rejected a soft improvement: %+v", cycle.Decision)
	}
}

func TestPlanInvalidMetric(t *testing.T) {
	cfg := consolidate.DefaultConfig()
	cfg.Metric = "bogus"
	arcs := []adh.Arc{closedArc("arc-0001", "missing boundary")}
	if _, err := consolidate.Plan(realisticArtifact, "x", arcs, nil, cfg); err == nil {
		t.Error("Plan with an unknown gate metric should return an error")
	}
}

func TestCategorizeRegressedHighestPriority(t *testing.T) {
	prev := []consolidate.Diagnostic{
		{Task: "a", Hard: 1}, {Task: "b", Hard: 0}, {Task: "c", Hard: 1}, {Task: "d", Hard: 0},
	}
	curr := []consolidate.Diagnostic{
		{Task: "a", Hard: 0}, // regressed
		{Task: "b", Hard: 1}, // improved
		{Task: "c", Hard: 1}, // stable success
		{Task: "d", Hard: 0}, // persistent fail
	}
	long := consolidate.Categorize(prev, curr)
	if long.Regressed != 1 || long.Improved != 1 || long.StableSuccess != 1 ||
		long.PersistentFail != 1 {
		t.Fatalf("counts = %+v, want one of each", long)
	}
	if len(long.Lines) == 0 || !strings.HasPrefix(long.Lines[0], "[regressed]") {
		t.Errorf("Lines = %v, want a regression first", long.Lines)
	}
}

func TestSlowGuidanceMentionsCounts(t *testing.T) {
	g := consolidate.SlowGuidance(
		consolidate.Longitudinal{Improved: 2, Regressed: 1, StableSuccess: 3},
	)
	if !strings.Contains(g, "2 improved") || !strings.Contains(g, "1 regressed") {
		t.Errorf("slow guidance = %q, want the counts named", g)
	}
}

func TestPlanFillsLongitudinal(t *testing.T) {
	class := selectionClass(t)
	arcs := []adh.Arc{closedArc("arc-0001", class), closedArc("arc-0002", class)}
	learned := consolidate.Propose(consolidate.Harvest(arcs), consolidate.DefaultConfig())
	cycle, err := consolidate.Plan(
		realisticArtifact,
		learned,
		arcs,
		nil,
		consolidate.DefaultConfig(),
	)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	// The candidate only adds guidance, so the test split never regresses.
	if cycle.Longitudinal.Regressed != 0 {
		t.Errorf("candidate regressed a test task: %+v", cycle.Longitudinal)
	}
	if cycle.SlowGuidance == "" {
		t.Error("accepted cycle produced no slow-update guidance")
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

func TestSplitForSeed(t *testing.T) {
	// Seed 0 is the canonical partition SplitFor uses.
	if consolidate.SplitForSeed("task-x", 0) != consolidate.SplitFor("task-x") {
		t.Error("seed 0 must equal SplitFor")
	}
	// Different seeds re-partition independently: over many ids, some land differently.
	moved := 0
	for i := range 60 {
		id := fmt.Sprintf("task-%d", i)
		if consolidate.SplitForSeed(id, 1) != consolidate.SplitForSeed(id, 2) {
			moved++
		}
	}
	if moved == 0 {
		t.Error("seeds 1 and 2 produced identical partitions for every id")
	}
}

// TestPlanComputesReplication: an accepted cycle carries a fresh-replication verdict
// (a known taxonomy value) computed over the independent seeded partitions (§18.2).
func TestPlanComputesReplication(t *testing.T) {
	class := selectionClass(t)
	arcs := []adh.Arc{closedArc("arc-0001", class), closedArc("arc-0002", class)}
	learned := consolidate.Propose(consolidate.Harvest(arcs), consolidate.DefaultConfig())
	cycle, err := consolidate.Plan(
		realisticArtifact,
		learned,
		arcs,
		nil,
		consolidate.DefaultConfig(),
	)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	known := map[verdict.Verdict]bool{
		verdict.Elevate: true, verdict.Directional: true,
		verdict.ReplicationMissing: true, verdict.Kill: true,
	}
	if !known[cycle.Replication] {
		t.Errorf("cycle.Replication = %q, want a known verdict", cycle.Replication)
	}
}

// TestProposePromptTrace: the relay prompt surfaces the held-out assertions the
// current artifact fails (the reflective trace), so the worker targets them (§18.6).
func TestProposePromptTrace(t *testing.T) {
	class := selectionClass(t)
	signals := consolidate.Harvest([]adh.Arc{closedArc("arc-0001", class)})
	cfg := consolidate.DefaultConfig()

	// An artifact missing the class: the held-out assertion fails and is surfaced.
	missing := consolidate.ProposePrompt(signals, "# Empty\n", cfg)
	if !strings.Contains(missing, "Currently-failing held-out assertions") ||
		!strings.Contains(missing, class) {
		t.Errorf("prompt should surface the failing assertion %q:\n%s", class, missing)
	}

	// An artifact that already mentions the class: nothing fails.
	passing := consolidate.ProposePrompt(signals, "# Guide\n\nGuard against "+class+".\n", cfg)
	if !strings.Contains(passing, "passes every held-out assertion") {
		t.Errorf("a satisfied artifact should show no failing assertions:\n%s", passing)
	}
}
