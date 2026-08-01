// Package consolidate is the pure core of the offline self-optimization cycle
// (SPEC-ADDITIONS §18.4): harvest closed arcs, mine recurring checkable tasks
// and assign each a stable held-out split, then reflect-and-gate a bounded
// candidate edit on a strict held-out improvement. It performs no I/O and reads
// no clock — the staging id is the content hash of the proposal, so a cycle is
// fully reproducible. The command shell (cmd/sleep) supplies the artifact, the
// harvested arcs, and the optimizer's proposed learned text, and writes staging
// from the returned Cycle; it makes no decision of its own.
//
// Scoring is deterministic without a live worker: a mined task is a checkable
// assertion about the guiding artifact (e.g. "the artifact should mention this
// failure class"). A split's score is the mean hard rule-judge score over its
// tasks, so learned text that makes more assertions pass scores strictly higher
// on the held-out selection split and the ratchet accepts it — while an empty or
// harmful candidate does not (mirrored by harness.SelfTest's negative control).
package consolidate

import (
	"fmt"
	"sort"
	"strings"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/edit"
	"github.com/StevenACoffman/agentic-dev-harness/internal/evidence"
	"github.com/StevenACoffman/agentic-dev-harness/internal/gate"
	"github.com/StevenACoffman/agentic-dev-harness/internal/harness"
	"github.com/StevenACoffman/agentic-dev-harness/internal/identity"
	"github.com/StevenACoffman/agentic-dev-harness/internal/judge"
	"github.com/StevenACoffman/agentic-dev-harness/internal/lesson"
	"github.com/StevenACoffman/agentic-dev-harness/internal/verdict"
)

// Split values partition mined tasks. Selection is held out for acceptance;
// test is reported on but never used to accept; train may grow from recalled or
// synthetic tasks (§18.4). A task's split is stable across cycles.
const (
	SplitSelection Split = "selection"
	SplitTest      Split = "test"
	SplitTrain     Split = "train"
)

// Mode kinds classify a reflected class (§18.3): a failure mode to avoid or a
// success mode to reinforce.
const (
	KindFailure ModeKind = "failure"
	KindSuccess ModeKind = "success"
)

// longitudinalLineCap bounds the number of regressed/persistent lines carried in
// a Longitudinal summary, keeping the slow-update guidance short.
const longitudinalLineCap = 8

// DefaultReplicationRuns is how many independent seeded partitions the fresh-
// replication eval scores a candidate over (§18.2) — enough for ≥2 independent
// passing runs to earn an ELEVATE.
const DefaultReplicationRuns = 3

// Split is the held-out partition a mined task belongs to.
type Split string

// ModeKind is whether a reflected class is a failure or a success mode.
type ModeKind string

// Mode is one reflected class and how often it recurred — the recurrence Count
// is the ranking key the edit budget clips against (rank_and_select).
type Mode struct {
	Class string   `json:"class"`
	Kind  ModeKind `json:"kind"`
	Count int      `json:"count"`
}

// Reflection is the optimizer's read of the harvested history (§18.3): failure
// modes to avoid and success modes to reinforce, each ranked by recurrence.
// Failure modes win a conflict — a class that is both is kept only as a failure.
type Reflection struct {
	Failure []Mode `json:"failure"`
	Success []Mode `json:"success"`
}

// Longitudinal compares a task's outcome under the previous artifact against the
// candidate (§18.3, SkillOpt's slow-update summary): how many improved,
// regressed, stayed green, or kept failing. Regressed is the highest priority,
// so Lines leads with regressions ahead of persistent failures.
type Longitudinal struct {
	Improved       int      `json:"improved"`
	Regressed      int      `json:"regressed"`
	StableSuccess  int      `json:"stable_success"`
	PersistentFail int      `json:"persistent_fail"`
	Lines          []string `json:"lines,omitempty"`
}

// Config carries the §18.5 levers: the protected-region marker (only that
// region is machine-edited), the edit budget (the learning rate — how many
// classes become guidance per round), the size ratio (a candidate may not
// exceed ratio × the current artifact), and the gate metric (how a hard/soft
// score pair projects onto one comparable number, ported from SkillOpt's
// select_gate_score).
type Config struct {
	Marker      string      `json:"marker"`
	EditBudget  int         `json:"edit_budget"`
	SizeRatio   float64     `json:"size_ratio"`
	Metric      gate.Metric `json:"metric"`
	MixedWeight float64     `json:"mixed_weight"`
	Round       int         `json:"round"`
}

// SplitScore is a split's aggregate rule-judge score — the mean hard and mean
// soft over its tasks, ported from SkillOpt's aggregate_scores (compute_score).
type SplitScore struct {
	Hard float64 `json:"hard"`
	Soft float64 `json:"soft"`
}

// Signal is one harvested closed arc's learnable content: the misses to avoid
// and the affirmative outcomes to reinforce.
type Signal struct {
	Arc        string   `json:"arc"`
	Resolution string   `json:"resolution,omitempty"`
	Failures   []string `json:"failures,omitempty"`
	Successes  []string `json:"successes,omitempty"`
}

// Task is a mined, checkable assertion about the artifact, assigned a stable
// split. Its id is the failure class it guards against.
type Task struct {
	ID     string        `json:"id"`
	Split  Split         `json:"split"`
	Checks []judge.Check `json:"checks"`
}

// Diagnostic is one held-out task's self-explanation (§18.6): its hard score and
// the reason, so a cycle that accepts nothing is not a black box.
type Diagnostic struct {
	Task   string  `json:"task"`
	Split  Split   `json:"split"`
	Hard   float64 `json:"hard"`
	Soft   float64 `json:"soft"`
	Reason string  `json:"reason"`
}

// Cycle is the pure outcome of one consolidation pass: the harvested signals,
// the mined tasks, the baseline and candidate selection scores, the gate
// decision, the proposed artifact (set only on acceptance), the candidate's
// content-addressed staging id (set whenever a candidate was scored, so a
// rejected one can be remembered), the per-task diagnostics, and the evidence
// records to append. Records carry no timestamp — the shell stamps them.
type Cycle struct {
	Signals       []Signal          `json:"signals"`
	Tasks         []Task            `json:"tasks"`
	Baseline      float64           `json:"baseline"`
	Candidate     float64           `json:"candidate"`
	BaselineSoft  float64           `json:"baseline_soft"`
	CandidateSoft float64           `json:"candidate_soft"`
	Decision      gate.Result       `json:"decision"`
	Proposed      string            `json:"proposed,omitempty"`
	StagingID     string            `json:"staging_id,omitempty"`
	Diags         []Diagnostic      `json:"diagnostics"`
	Longitudinal  Longitudinal      `json:"longitudinal"`
	SlowGuidance  string            `json:"slow_guidance,omitempty"`
	Verdict       verdict.Verdict   `json:"verdict,omitempty"`
	Replication   verdict.Verdict   `json:"replication,omitempty"`
	Records       []evidence.Record `json:"records"`
}

// DefaultConfig returns the §18.5 defaults: the ADH:LEARNED marker, an edit
// budget of 4, a 1.5× size bound, and the hard gate metric.
func DefaultConfig() Config {
	return Config{Marker: "ADH:LEARNED", EditBudget: 4, SizeRatio: 1.5, Metric: gate.Hard}
}

// effectiveBudget anneals the edit budget across cycles — the learning-rate
// schedule (SkillOpt's lr_scheduler): a linear decay of one edit per round with
// a floor of 1, so later cycles make smaller, more attributable changes. A
// non-positive base budget means unlimited and is left untouched.
func (c Config) effectiveBudget() int {
	if c.EditBudget <= 0 {
		return c.EditBudget
	}
	return max(1, c.EditBudget-c.Round)
}

// Harvest reduces closed arcs to their learnable signals; open arcs are skipped.
func Harvest(arcs []adh.Arc) []Signal {
	signals := make([]Signal, 0, len(arcs))
	for i := range arcs {
		if arcs[i].Status != adh.StatusClosed {
			continue
		}
		signals = append(signals, Signal{
			Arc:        arcs[i].ID,
			Resolution: string(arcs[i].Resolution),
			Failures:   failureLines(arcs[i].History),
			Successes:  successLines(arcs[i].History),
		})
	}
	return signals
}

// Mine turns recurring failure classes into one checkable task each, assigning a
// stable split by the class text. The task asserts the artifact should mention
// the class it guards against.
func Mine(signals []Signal, _ Config) []Task {
	lessons := lesson.Distill(collectFailures(signals))
	tasks := make([]Task, 0, len(lessons))
	for i := range lessons {
		class := lessons[i].Class
		tasks = append(tasks, Task{
			ID:     class,
			Split:  SplitFor(class),
			Checks: []judge.Check{{Op: judge.OpContains, Arg: class}},
		})
	}
	return tasks
}

// Reflect reads the harvested signals into ranked failure and success modes
// (§18.3): each class is ranked by recurrence (rank_and_select), and a class
// that recurs as both a failure and a success is kept only as a failure —
// failure modes win the conflict.
func Reflect(signals []Signal) Reflection {
	failure := modesFrom(collectFailures(signals), KindFailure)
	inFailure := make(map[string]bool, len(failure))
	for i := range failure {
		inFailure[failure[i].Class] = true
	}
	var success []Mode
	for _, mode := range modesFrom(collectSuccesses(signals), KindSuccess) {
		if !inFailure[mode.Class] {
			success = append(success, mode)
		}
	}
	return Reflection{Failure: failure, Success: success}
}

// Propose is the mock optimizer: it renders the reflected modes as guidance
// bullets for the protected region — failure modes ("Guard against") first so
// they win the edit budget, then success modes ("Keep doing"). The budget (the
// learning rate) clips the ranked remainder. An empty result means there was
// nothing to learn.
func Propose(signals []Signal, cfg Config) string {
	reflection := Reflect(signals)
	budget := cfg.effectiveBudget()
	var b strings.Builder
	used := 0
	render := func(modes []Mode, lead string) {
		for i := range modes {
			if budget > 0 && used >= budget {
				return
			}
			b.WriteString(lead)
			b.WriteString(modes[i].Class)
			b.WriteString("\n")
			used++
		}
	}
	render(reflection.Failure, "- Guard against: ")
	render(reflection.Success, "- Keep doing: ")
	return b.String()
}

// ProposePrompt renders the relay prompt for an agent-driven proposal (§18): the
// ranked reflection modes (what recurred), the artifact's current protected region,
// and the instruction to reply with the one bounded edit to that region. It is the
// relay counterpart to the mock Propose — the agent supplies the edit and may
// consult `adh tool run skillsaw-eval` for dimension scores, while adh's held-out
// gate still decides whether the reply is kept. It is pure.
func ProposePrompt(signals []Signal, artifact string, cfg Config) string {
	reflection := Reflect(signals)
	var b strings.Builder
	b.WriteString("Propose the single bounded edit to the LEARNED region below (§18).\n")
	b.WriteString(
		"Reply with only the new region body; adh's held-out gate decides if it is kept.\n",
	)
	b.WriteString("You may consult `adh tool run skillsaw-eval` for dimension scores first.\n\n")
	b.WriteString("Recurring failure modes to guard against (ranked by recurrence):\n")
	writeModes(&b, reflection.Failure)
	b.WriteString("\nSuccess modes to reinforce:\n")
	writeModes(&b, reflection.Success)
	b.WriteString("\nCurrently-failing held-out assertions — target these (§18.6):\n")
	writeFailingTasks(&b, signals, artifact, cfg)
	b.WriteString("\nCurrent LEARNED region:\n")
	if current := currentLearned(artifact, cfg.Marker); current != "" {
		b.WriteString(current)
		b.WriteString("\n")
	} else {
		b.WriteString("(empty)\n")
	}
	return b.String()
}

// writeFailingTasks renders the held-out assertions the current artifact fails —
// the reflective trace (§18.6, GEPA): scoring the mined selection-split tasks against
// the artifact so the worker targets a bounded edit at what actually fails, not just
// the recurring modes. It is pure (judge.Score reads no I/O); a task whose score
// cannot be computed is skipped rather than surfaced.
func writeFailingTasks(b *strings.Builder, signals []Signal, artifact string, cfg Config) {
	failing := 0
	for _, task := range Mine(signals, cfg) {
		if task.Split != SplitSelection {
			continue
		}
		result, err := judge.Score(artifact, task.Checks)
		if err != nil || result.Hard >= 1.0 {
			continue
		}
		_, _ = fmt.Fprintf(b, "- %s (%s)\n", task.ID, reasonFor(result.Hard))
		failing++
	}
	if failing == 0 {
		b.WriteString("- (none — the current artifact passes every held-out assertion)\n")
	}
}

// writeModes lists reflected modes as ranked bullets, or "(none)" when empty.
func writeModes(b *strings.Builder, modes []Mode) {
	if len(modes) == 0 {
		b.WriteString("- (none)\n")
		return
	}
	for i := range modes {
		_, _ = fmt.Fprintf(b, "- %s (x%d)\n", modes[i].Class, modes[i].Count)
	}
}

// currentLearned returns the trimmed body of the artifact's protected region, or ""
// when the region is absent — the editable text the relay shows the worker.
func currentLearned(artifact, marker string) string {
	start := "<!-- " + marker + " START -->"
	end := "<!-- " + marker + " END -->"
	_, after, found := strings.Cut(artifact, start)
	if !found {
		return ""
	}
	body, _, found := strings.Cut(after, end)
	if !found {
		return ""
	}
	return strings.TrimSpace(body)
}

// modesFrom distills items into classes and ranks them by recurrence descending,
// ties broken by class name for determinism.
func modesFrom(items []string, kind ModeKind) []Mode {
	lessons := lesson.Distill(items)
	modes := make([]Mode, 0, len(lessons))
	for i := range lessons {
		modes = append(modes, Mode{
			Class: lessons[i].Class,
			Kind:  kind,
			Count: len(lessons[i].Instances),
		})
	}
	sort.SliceStable(modes, func(i, j int) bool {
		if modes[i].Count != modes[j].Count {
			return modes[i].Count > modes[j].Count
		}
		return modes[i].Class < modes[j].Class
	})
	return modes
}

// SplitFor maps a task id to its stable split by hashing the id, so a real
// task's split never changes across cycles and no clock or randomness is used.
func SplitFor(id string) Split {
	h := identity.Hash(id)
	if h == "" {
		return SplitTrain
	}
	switch h[0] % 3 {
	case 0:
		return SplitSelection
	case 1:
		return SplitTest
	default:
		return SplitTrain
	}
}

// SplitForSeed is SplitFor over an independent partition selected by seed, so the
// same mined tasks can be re-partitioned N ways for a fresh-replication eval (§18.2)
// — each seed is an independent held-out selection set, and no clock or randomness is
// used. Seed 0 is the canonical partition SplitFor uses.
func SplitForSeed(id string, seed uint64) Split {
	if seed == 0 {
		return SplitFor(id)
	}
	return SplitFor(fmt.Sprintf("%d:%s", seed, id))
}

// Plan runs one consolidation cycle purely. Given the current artifact, the
// optimizer's proposed learned text, the harvested arcs, and the set of
// previously rejected candidate hashes, it mines tasks, scores the baseline and
// the candidate over the selection split, and accepts only a strict held-out
// improvement (§18.2). It stages a proposal (Proposed/StagingID set) only on
// acceptance; every outcome carries diagnostics and an evidence record.
func Plan(
	artifact, learned string,
	arcs []adh.Arc,
	rejected map[string]bool,
	cfg Config,
) (Cycle, error) {
	signals := Harvest(arcs)
	tasks := Mine(signals, cfg)
	baseSplit, baseDiags, err := scoreSplit(artifact, tasks, SplitSelection)
	if err != nil {
		return Cycle{}, err
	}
	baseScore, err := metricScore(cfg, baseSplit)
	if err != nil {
		return Cycle{}, err
	}
	cycle := Cycle{
		Signals:      signals,
		Tasks:        tasks,
		Baseline:     baseScore,
		Candidate:    baseScore,
		BaselineSoft: baseSplit.Soft,
		Decision:     gate.Evaluate(baseScore, baseScore, baseScore, 0, 0),
		Diags:        baseDiags,
	}
	candidate, note, proceed := proposal(artifact, learned, rejected, cfg)
	if !proceed {
		if err := cycle.setLongitudinal(artifact, artifact, tasks); err != nil {
			return Cycle{}, err
		}
		cycle.Records = record(cycle.Decision, baseScore, baseScore, evidence.StatusBaseline, note)
		return cycle, nil
	}
	candSplit, candDiags, err := scoreSplit(candidate, tasks, SplitSelection)
	if err != nil {
		return Cycle{}, err
	}
	candScore, err := metricScore(cfg, candSplit)
	if err != nil {
		return Cycle{}, err
	}
	cycle.Candidate = candScore
	cycle.CandidateSoft = candSplit.Soft
	cycle.Decision = harness.Accept(candScore, baseScore, baseScore)
	cycle.Diags = candDiags
	if err := cycle.setLongitudinal(artifact, candidate, tasks); err != nil {
		return Cycle{}, err
	}
	if err := cycle.setVerdicts(artifact, candidate, tasks, baseScore, candScore, cfg); err != nil {
		return Cycle{}, err
	}
	// A scored candidate is identified by its content hash whatever the verdict,
	// so the shell can remember a rejected one in the negative-feedback buffer
	// (§18.3); only an accepted candidate is proposed for staging.
	cycle.StagingID = identity.Hash(candidate)
	if cycle.Decision.Action == gate.Reject {
		cycle.Records = record(cycle.Decision, baseScore, candScore, evidence.StatusBaseline,
			fmt.Sprintf("candidate %.3f did not beat baseline %.3f", candScore, baseScore))
		return cycle, nil
	}
	cycle.Proposed = candidate
	cycle.Records = record(cycle.Decision, baseScore, candScore, evidence.StatusKeep,
		fmt.Sprintf("candidate improved selection %.3f -> %.3f", baseScore, candScore))
	return cycle, nil
}

// verdictFor computes the replication-gated verdict (§18.2) for a scored candidate:
// the primary is the selection-split improvement (candScore − baseScore) the gate
// ratchets on; the replication is the held-out test split's paired outcomes, so a
// candidate is ELEVATE only when its selection gain also replicates significantly on
// the test split. It never changes staging — the strict gate already decided that;
// the verdict labels how much to trust the change.
func verdictFor(baseScore, candScore float64, long Longitudinal) verdict.Verdict {
	_, significant := verdict.McNemar(long.Improved, long.Regressed)
	hasReplication := long.Improved+long.Regressed+long.StableSuccess+long.PersistentFail > 0
	return verdict.Decide(
		verdict.Outcome{Delta: candScore - baseScore},
		verdict.Outcome{Delta: float64(long.Improved - long.Regressed), Significant: significant},
		verdict.DefaultMinEffect,
		hasReplication,
	)
}

// setVerdicts records the two trust labels on the cycle (§18.2): the single held-out
// verdict (selection primary vs test-split replication) and the fresh multi-run
// replication verdict over independent seeded partitions. Neither changes staging —
// the strict gate already decided that; they label how much to trust the change.
func (c *Cycle) setVerdicts(
	artifact, candidate string,
	tasks []Task,
	baseScore, candScore float64,
	cfg Config,
) error {
	c.Verdict = verdictFor(baseScore, candScore, c.Longitudinal)
	replication, err := replicationVerdict(artifact, candidate, tasks, cfg, DefaultReplicationRuns)
	if err != nil {
		return err
	}
	c.Replication = replication
	return nil
}

// replicationVerdict is the fresh-replication verdict (§18.2): it scores the
// candidate against the baseline over `runs` independent seeded partitions of the
// mined tasks — each an independent held-out selection set — and requires the
// candidate to win significantly on enough of them (verdict.Replicate). This is the
// fresh replication adh's own deterministic evaluator can run without a live model
// worker: a real multi-run check, not the single held-out split verdictFor uses.
func replicationVerdict(
	artifact, candidate string,
	tasks []Task,
	cfg Config,
	runs int,
) (verdict.Verdict, error) {
	outcomes := make([]verdict.Outcome, 0, runs)
	for seed := 1; seed <= runs; seed++ {
		outcome, err := runOutcome(artifact, candidate, tasks, cfg, uint64(seed))
		if err != nil {
			return "", err
		}
		outcomes = append(outcomes, outcome)
	}
	return verdict.Replicate(outcomes, verdict.DefaultMinEffect), nil
}

// runOutcome scores one independent seeded partition: the candidate's selection
// improvement over the baseline (the delta) and whether the paired before/after
// outcomes are significant (McNemar) — one independent run for verdict.Replicate.
func runOutcome(
	artifact, candidate string,
	tasks []Task,
	cfg Config,
	seed uint64,
) (verdict.Outcome, error) {
	baseSplit, baseDiags, err := scoreSeeded(artifact, tasks, seed)
	if err != nil {
		return verdict.Outcome{}, err
	}
	baseScore, err := metricScore(cfg, baseSplit)
	if err != nil {
		return verdict.Outcome{}, err
	}
	candSplit, candDiags, err := scoreSeeded(candidate, tasks, seed)
	if err != nil {
		return verdict.Outcome{}, err
	}
	candScore, err := metricScore(cfg, candSplit)
	if err != nil {
		return verdict.Outcome{}, err
	}
	improved, regressed := pairedCounts(baseDiags, candDiags)
	_, significant := verdict.McNemar(improved, regressed)
	return verdict.Outcome{Delta: candScore - baseScore, Significant: significant}, nil
}

// pairedCounts compares each task's hard outcome before and after over the same
// seeded selection set: improved counts fail→pass, regressed counts pass→fail.
func pairedCounts(before, after []Diagnostic) (improved, regressed int) {
	prev := make(map[string]float64, len(before))
	for i := range before {
		prev[before[i].Task] = before[i].Hard
	}
	for i := range after {
		was, ok := prev[after[i].Task]
		if !ok {
			continue
		}
		switch {
		case after[i].Hard > was:
			improved++
		case after[i].Hard < was:
			regressed++
		}
	}
	return improved, regressed
}

// setLongitudinal scores both artifacts over the report-only test split and
// records the longitudinal categories plus the durable slow-update guidance
// (§18.3). The test split never gates; it is compared for regressions.
func (c *Cycle) setLongitudinal(before, after string, tasks []Task) error {
	_, beforeDiags, err := scoreSplit(before, tasks, SplitTest)
	if err != nil {
		return err
	}
	_, afterDiags, err := scoreSplit(after, tasks, SplitTest)
	if err != nil {
		return err
	}
	c.Longitudinal = Categorize(beforeDiags, afterDiags)
	c.SlowGuidance = SlowGuidance(c.Longitudinal)
	return nil
}

// Categorize compares each task's hard outcome under the previous artifact
// against the candidate, counting improved/regressed/stable/persistent-fail
// (SkillOpt's slow-update summary). Lines leads with regressions — the highest
// priority — then persistent failures, capped for brevity.
func Categorize(prev, curr []Diagnostic) Longitudinal {
	prevHard := make(map[string]float64, len(prev))
	for i := range prev {
		prevHard[prev[i].Task] = prev[i].Hard
	}
	var (
		long       Longitudinal
		regressed  []string
		persistent []string
	)
	for i := range curr {
		before, ok := prevHard[curr[i].Task]
		if !ok {
			continue
		}
		switch after := curr[i].Hard; {
		case after > before:
			long.Improved++
		case after < before:
			long.Regressed++
			regressed = append(regressed, "[regressed] "+curr[i].Task)
		case after >= 1.0:
			long.StableSuccess++
		default:
			long.PersistentFail++
			persistent = append(persistent, "[persistent-fail] "+curr[i].Task)
		}
	}
	long.Lines = capLines(append(regressed, persistent...), longitudinalLineCap)
	return long
}

// SlowGuidance renders durable cross-cycle guidance from the longitudinal
// categories (§18.3, SkillOpt's slow update) — deterministic, no backend.
func SlowGuidance(long Longitudinal) string {
	var b strings.Builder
	_, _ = fmt.Fprintf(&b,
		"Longitudinal: %d improved, %d regressed, %d stable, %d persistent-fail.\n",
		long.Improved, long.Regressed, long.StableSuccess, long.PersistentFail)
	if long.StableSuccess > 0 {
		_, _ = fmt.Fprintf(&b, "Preserve the %d behaviors that stayed green.\n", long.StableSuccess)
	}
	for _, line := range long.Lines {
		b.WriteString("- avoid: ")
		b.WriteString(line)
		b.WriteString("\n")
	}
	return b.String()
}

func capLines(lines []string, limit int) []string {
	if len(lines) > limit {
		return lines[:limit]
	}
	return lines
}

// proposal builds the candidate artifact and reports whether the cycle should
// score it. It short-circuits (proceed=false) on an empty optimizer reply, a
// no-op edit, a size-budget violation, or a previously rejected candidate,
// returning a §18.6 self-explanation in note.
func proposal(
	artifact, learned string,
	rejected map[string]bool,
	cfg Config,
) (candidate, note string, proceed bool) {
	if strings.TrimSpace(learned) == "" {
		return "", "optimizer proposed no edit", false
	}
	candidate = applyLearned(artifact, learned, cfg.Marker)
	switch {
	case edit.IsNoOp(artifact, candidate):
		return "", "learned region unchanged (no-op edit)", false
	case !edit.WithinSizeBudget(len(artifact), len(candidate), cfg.SizeRatio):
		return "", "candidate exceeds the size budget", false
	case rejected[identity.Hash(candidate)]:
		return "", "candidate was rejected in an earlier cycle", false
	default:
		return candidate, "", true
	}
}

// scoreSplit is the split's aggregate score — the mean hard and mean soft
// rule-judge score over its tasks (SkillOpt's aggregate_scores) — plus a
// per-task diagnostic. An empty split scores zero with no diagnostics.
func scoreSplit(artifact string, tasks []Task, split Split) (SplitScore, []Diagnostic, error) {
	return scoreWhere(artifact, tasks, split, func(i int) bool { return tasks[i].Split == split })
}

// scoreSeeded scores the selection split of an independent seeded partition (§18.2),
// used by the multi-run replication eval — the same tasks, re-partitioned by seed.
func scoreSeeded(artifact string, tasks []Task, seed uint64) (SplitScore, []Diagnostic, error) {
	return scoreWhere(artifact, tasks, SplitSelection, func(i int) bool {
		return SplitForSeed(tasks[i].ID, seed) == SplitSelection
	})
}

// scoreWhere is the split's aggregate score — the mean hard and mean soft rule-judge
// score over the tasks include selects (SkillOpt's aggregate_scores) — plus a
// per-task diagnostic labeled with split. An empty selection scores zero.
func scoreWhere(
	artifact string,
	tasks []Task,
	split Split,
	include func(int) bool,
) (SplitScore, []Diagnostic, error) {
	var (
		diags   []Diagnostic
		sumHard float64
		sumSoft float64
		count   int
	)
	for i := range tasks {
		if !include(i) {
			continue
		}
		count++
		result, err := judge.Score(artifact, tasks[i].Checks)
		if err != nil {
			return SplitScore{}, nil, &adh.Error{Op: "consolidate.scoreWhere", Err: err}
		}
		sumHard += result.Hard
		sumSoft += result.Soft
		diags = append(diags, Diagnostic{
			Task:   tasks[i].ID,
			Split:  split,
			Hard:   result.Hard,
			Soft:   result.Soft,
			Reason: reasonFor(result.Hard),
		})
	}
	if count == 0 {
		return SplitScore{}, diags, nil
	}
	return SplitScore{Hard: sumHard / float64(count), Soft: sumSoft / float64(count)}, diags, nil
}

// metricScore projects a split score onto the single comparable number the
// ratchet uses, via the configured metric (SkillOpt's select_gate_score). An
// unset metric defaults to hard.
func metricScore(cfg Config, score SplitScore) (float64, error) {
	metric := cfg.Metric
	if metric == "" {
		metric = gate.Hard
	}
	projected, err := gate.SelectScore(score.Hard, score.Soft, metric, cfg.MixedWeight)
	if err != nil {
		return 0, &adh.Error{Op: "consolidate.metricScore", Err: err}
	}
	return projected, nil
}

// record builds the single evidence line for a cycle outcome. Timestamp is left
// for the shell to stamp, keeping this core clock-free.
func record(
	decision gate.Result,
	oldScore, newScore float64,
	status evidence.Status,
	note string,
) []evidence.Record {
	return []evidence.Record{{
		GateAction: string(decision.Action),
		OldScore:   oldScore,
		NewScore:   newScore,
		Status:     status,
		Note:       note,
	}}
}

func collectFailures(signals []Signal) []string {
	var all []string
	for i := range signals {
		all = append(all, signals[i].Failures...)
	}
	return all
}

func collectSuccesses(signals []Signal) []string {
	var all []string
	for i := range signals {
		all = append(all, signals[i].Successes...)
	}
	return all
}

// successLines keeps affirmative history entries (a resolved outcome to
// reinforce), stripping the optional "stage: " prefix. A line that reads as a
// miss is never a success — failure wins at the line level too.
func successLines(history []string) []string {
	var out []string
	for _, line := range history {
		text := line
		if i := strings.Index(line, ": "); i >= 0 {
			text = strings.TrimSpace(line[i+2:])
		}
		if !looksLikeFailure(text) && looksLikeSuccess(text) {
			out = append(out, text)
		}
	}
	return out
}

func looksLikeSuccess(s string) bool {
	low := strings.ToLower(s)
	for _, kw := range []string{"pass", "ok", "clean", "merged", "shipped", "verified", "green"} {
		if strings.Contains(low, kw) {
			return true
		}
	}
	return false
}

// failureLines keeps the history entries that read as a miss, stripping an
// optional "stage: " prefix so the finding text — not the stage — is the class.
func failureLines(history []string) []string {
	var out []string
	for _, line := range history {
		text := line
		if i := strings.Index(line, ": "); i >= 0 {
			text = strings.TrimSpace(line[i+2:])
		}
		if looksLikeFailure(text) {
			out = append(out, text)
		}
	}
	return out
}

func looksLikeFailure(s string) bool {
	low := strings.ToLower(s)
	for _, kw := range []string{"fail", "error", "missing", "timeout", "regress"} {
		if strings.Contains(low, kw) {
			return true
		}
	}
	return false
}

// applyLearned splices learned into the artifact's protected region, preserving
// all hand-authored content outside it. When the region is absent a fresh block
// is appended (§18.1: the optimizer writes only the protected region).
func applyLearned(artifact, learned, marker string) string {
	start := "<!-- " + marker + " START -->"
	end := "<!-- " + marker + " END -->"
	block := start + "\n" + strings.TrimRight(learned, "\n") + "\n" + end
	si := strings.Index(artifact, start)
	ei := strings.Index(artifact, end)
	if si >= 0 && ei > si {
		return artifact[:si] + block + artifact[ei+len(end):]
	}
	if artifact == "" {
		return block + "\n"
	}
	sep := "\n"
	if !strings.HasSuffix(artifact, "\n") {
		sep = "\n\n"
	}
	return artifact + sep + block + "\n"
}

func reasonFor(hard float64) string {
	if hard >= 1.0 {
		return "all checks pass"
	}
	return "one or more checks fail"
}
