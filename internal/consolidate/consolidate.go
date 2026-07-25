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
	"strings"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/edit"
	"github.com/StevenACoffman/agentic-dev-harness/internal/evidence"
	"github.com/StevenACoffman/agentic-dev-harness/internal/gate"
	"github.com/StevenACoffman/agentic-dev-harness/internal/harness"
	"github.com/StevenACoffman/agentic-dev-harness/internal/identity"
	"github.com/StevenACoffman/agentic-dev-harness/internal/judge"
	"github.com/StevenACoffman/agentic-dev-harness/internal/lesson"
)

// Split values partition mined tasks. Selection is held out for acceptance;
// test is reported on but never used to accept; train may grow from recalled or
// synthetic tasks (§18.4). A task's split is stable across cycles.
const (
	SplitSelection Split = "selection"
	SplitTest      Split = "test"
	SplitTrain     Split = "train"
)

// Split is the held-out partition a mined task belongs to.
type Split string

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
}

// SplitScore is a split's aggregate rule-judge score — the mean hard and mean
// soft over its tasks, ported from SkillOpt's aggregate_scores (compute_score).
type SplitScore struct {
	Hard float64 `json:"hard"`
	Soft float64 `json:"soft"`
}

// Signal is one harvested closed arc's learnable content.
type Signal struct {
	Arc        string   `json:"arc"`
	Resolution string   `json:"resolution,omitempty"`
	Failures   []string `json:"failures,omitempty"`
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
	Records       []evidence.Record `json:"records"`
}

// DefaultConfig returns the §18.5 defaults: the ADH:LEARNED marker, an edit
// budget of 4, a 1.5× size bound, and the hard gate metric.
func DefaultConfig() Config {
	return Config{Marker: "ADH:LEARNED", EditBudget: 4, SizeRatio: 1.5, Metric: gate.Hard}
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

// Propose is the mock optimizer: it renders the distilled failure classes as
// guidance bullets for the protected region, clipping to the edit budget (the
// learning rate). An empty result means there was nothing to learn.
func Propose(signals []Signal, cfg Config) string {
	lessons := lesson.Distill(collectFailures(signals))
	if cfg.EditBudget > 0 && len(lessons) > cfg.EditBudget {
		lessons = lessons[:cfg.EditBudget]
	}
	var b strings.Builder
	for i := range lessons {
		b.WriteString("- Guard against: ")
		b.WriteString(lessons[i].Class)
		b.WriteString("\n")
	}
	return b.String()
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
	var (
		diags   []Diagnostic
		sumHard float64
		sumSoft float64
		count   int
	)
	for i := range tasks {
		if tasks[i].Split != split {
			continue
		}
		count++
		result, err := judge.Score(artifact, tasks[i].Checks)
		if err != nil {
			return SplitScore{}, nil, &adh.Error{Op: "consolidate.scoreSplit", Err: err}
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
