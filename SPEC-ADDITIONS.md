# Agentic Development Harness — SPEC Additions

These sections extend [`SPEC.md`](./SPEC.md) to close the gaps between `adh` and
the harness-engineering practice it instantiates. The base spec is a strong
verification-and-authority harness; these additions supply the two levers that
practice treats as primary — **context** and **tools** — and the **compounding
loop** that lets corrections improve later runs instead of being re-caught every
arc.

The practice is Ryan Lopopolo's *Harness Engineering* (see the
[README lineage](./README.md#lineage)): hold the worker constant, improve context
and tools, make the repository teach the agent, prove the outcome, and turn
feedback into infrastructure. Each addition below maps to one of those areas —
§10 context and §13 tools are the two levers; §11 lessons, §16 effectiveness, and
§18 harness self-optimization are the feedback loop; §14 worker requalification is
the fixed-worker epoch. The mapping is drawn out in the
[README](./README.md#the-practice--ryan-lopopolos-harness-engineering).

> Ryan Lopopolo, *Harness Engineering*, CC BY 4.0,
> <https://github.com/lopopolo/harness-engineering>.

They follow the base spec's conventions (command tables, `.adh/config.toml`
sections, `state.json` model, behavioral gate specs, exit codes) and are numbered
to fold in as §10–§17, before §9 Non-goals, which remains last. Each section
names the gap it closes.

Design rule carried from the base spec: a control belongs in `adh` only when it
is deterministic or gated. Judgment — which context a task needs, whether a
lesson generalizes — is proposed by an agent and **confirmed at a gate**, never
self-applied.

______________________________________________________________________

## 10. Context Store and Routing

**Closes:** just-in-time context; make the repository teach the agent. Today a
stage receives the change, proof packet, and acceptance bar but no routed domain
context, so the critic and oracle compensate for an environment that does not
teach.

The harness keeps a large navigable **context store** and gives each stage a
small **active working set** selected by the arc's labels and target paths.

### 10.1 Commands

| Command                      | Purpose                                                                                |
| ---------------------------- | -------------------------------------------------------------------------------------- |
| `adh context list`           | List context units (id, kind, routing labels, owner).                                  |
| `adh context show <id>`      | Print a context unit and where it routes.                                              |
| `adh context route <arc-id>` | Print the working set `adh` would load for an arc, and why.                            |
| `adh context lint`           | Check that every routed unit resolves and every executable constraint it names exists. |

### 10.2 Configuration

```toml
[context]
store       = ".adh/context"     # runbooks, skills, domain notes, NFR checks
max_units   = 8                  # cap on a stage's active working set
route_by    = ["labels", "paths"]  # arc labels and touched paths select units
```

A context unit is one of: a **runbook** (a procedure), a **skill** (an approach
for a class of task), a **domain note** (a canonical owner's decision, e.g. the
approved crypto library), an **NFR check** (an executable constraint — a lint,
type, or test that encodes a nonfunctional requirement), or a **decision** (a
recorded ADR — see §12 — carrying the context, the decision, and its explicit
`Easier`/`Harder` trade-offs). The decision kind is the durable, routable home for
a team's *local choices about how to prioritize, trade off, and satisfy* their
nonfunctional requirements — the governance a domain model deliberately omits. A
later arc routes the relevant decisions and does not re-litigate them. Units
declare routing labels and an owner; Strategy and Execution load the routed set
before acting.

### 10.3 State

Each arc records `context: [unit-id, …]`, the exact working set it loaded, so a
self-eval can ask whether a missed requirement was un-routed (a context gap) or
routed-and-ignored (a worker gap).

**Exit code 12** — a routed context unit is missing or fails to resolve.

### 10.4 Content, Provenance, and Integrity

Routing selects *which* units apply; the worker must also be able to reach *what*
they say. A unit therefore carries a **content route** (the file holding its text)
and **provenance** (its source identity — for a vendored or distilled unit, an
origin / ref / commit / content digest, re-verified on read). The stage grounding
**previews** each routed unit (id, kind, one-line, where to read it) and the worker
pulls the full slice via `adh context show <id>` when it reaches the decision the
unit governs — just-in-time retrieval, not prompt-stuffing, so the route survives
compaction.

- **Integrity (anti-drift).** When a unit's text is rendered or derived from a
  canonical source, the harness can verify the routed content still matches that
  source (a drift check), so an arc proves the context it reasoned over is current,
  not stale. Registered as a check (§13), a failing integrity check blocks the arc.
- **Cross-unit consistency.** `adh context lint` (or `context check`) surfaces
  contradictions across the routed set: deterministic reference-integrity is owned
  by whatever tool renders a structured unit (e.g. a domain-model validator), and
  irreducibly-semantic conflict — one unit's rule contradicting another's, or a
  skill contradicting a base rule — is adjudicated by the cold critic and gated.
- **Harness integrity.** Beyond a single unit, the harness can verify itself: every
  routed unit resolves to a real artifact, every named tool exists, agent-facing
  guidance (boundaries, architecture map, decision index) references only modules
  and commands that exist, and the pieces do not contradict. This is a cheap
  persistent self-check — path existence, command/tool resolution, section
  presence, dangling-reference detection — run in CI and at session start so a
  half-edited or drifted harness fails fast rather than silently misguiding a run.

These capabilities are tool-agnostic: a unit's content may be hand-written, or
produced and validated by an external §13 tool (a domain-model renderer, a
knowledge-base distiller, a skill optimizer). The harness owns routing, retrieval,
integrity, and consistency; it does not own how a unit's text is authored.

### 10.5 Nonfunctional Requirements: Taxonomy and Quantified Spec (Planguage)

An **NFR-check** unit (§10.2) is not free prose — "should be fast" cannot be
tested or gated. An NFR is named by an agreed **taxonomy** (ISO/IEC 25010 or
FURPS+ — usability, performance, reliability, availability, security,
maintainability, portability, …) so its category is standard, not invented, and it
is *quantified* in **Planguage** (Tom Gilb) so it becomes measurable and gateable:

| Keyword    | Role in adh                                                                       |
| ---------- | --------------------------------------------------------------------------------- |
| `Tag`      | taxonomy-qualified structured name (e.g. `Performance.Latency`) — the unit id     |
| `Gist`     | one-line description                                                              |
| `Ambition` | the rationale — links to the governing decision/ADR (§12) that accepted it        |
| `Scale`    | unit of measure with context                                                      |
| `Meter`    | the **measurement method** → the executable check (§13) that produces the SLI     |
| `Baseline` | the current level (the ratchet's starting point, cf. §16 effectiveness)           |
| `Fail`     | the threshold that **fails the Evaluation gate** — the acceptance bar (§SPEC 3.1) |
| `Goal`     | the target that fully satisfies                                                   |
| `Stretch`  | the point past which further ROI is negligible                                    |

So a single NFR unit binds the four things adh otherwise scatters: its *category*
(taxonomy), its *check* (`Meter` → §13 tool → SLI), its *gate* (`Fail` → the
proof-contract acceptance bar and Evaluation threshold), and its *rationale*
(`Ambition` → a `decision`/ADR, §12). This is the durable, testable representation
of a nonfunctional requirement the domain model omits and the ADR only decides.

The distinction this makes precise: **validation** asks whether the requirements
are the *right* ones and mutually consistent — the cross-unit consistency check
(§10.4) and the cold critic — while **verification** asks whether the system is
built *right* against them — proof and the `Meter`-driven Evaluation gate. NFRs
may be *allocated* down onto components (a top-level `Fail` split into per-module
budgets) by routing the derived unit to the component's labels/paths (§10).

The taxonomy and the Planguage format are the adopted **vocabulary and schema**; a
guide that teaches them is a curated *source* to distill (via a §13 distiller) into
routable NFR units, not something the harness embeds.

______________________________________________________________________

## 11. Lessons: Promote Corrections into the Environment

**Closes:** turn feedback into infrastructure (the deepest gap). The base
`failures.json` records per-arc root-cause fixes, so `adh` re-catches the same
class of mistake every arc. A lesson moves a recurring correction into its
**smallest durable owner** so later arcs never reach the critic with it.

### 11.1 Commands

| Command                                | Purpose                                                                                                                                 |
| -------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------- |
| `adh lesson list`                      | Candidate lessons distilled from the failure registry, cold-critic findings, and human corrections, grouped by governing failure class. |
| `adh lesson show <id>`                 | The class, its instances, and the proposed durable owner.                                                                               |
| `adh lesson promote <id> --to <owner>` | Move the lesson to a durable owner (see below). Gated.                                                                                  |
| `adh lesson gc`                        | Flag stale lessons and exemptions whose triggering behavior no longer recurs.                                                           |

`--to <owner>` is one of: `context` (a domain note or runbook), `skill`,
`check` (a new lint or test), `invariant` (a new property-based rule on the
engine), `type` (a domain-model change), `doc`, or `decision` (an ADR recording a
now-settled trade-off, §12) — the last for a correction that is a judgment call
rather than an executable rule.

### 11.2 Behavioral Spec — the Promotion Gate

Promoting a lesson to an **executable owner** (`check`, `invariant`, `type`)
changes the harness's own gates and is therefore consequential. It requires human
approval exactly like an irreversible action (§5.2): `adh` proposes the class,
the owner, and the diff; a human approves. Promotion to `context`, `skill`, or
`doc` is reversible and proceeds under the current autonomy level.

A promotion **materializes the durable owner** — it writes the artifact, it does
not merely record the intent: a `context` promotion creates a routable §10 context
unit (with provenance, §10.4); `skill` produces or scaffolds a skill; `check`
registers a §13 tool entry; `doc` writes the guidance. Only then has the correction
become accretive — the next arc inherits it without re-arguing. An external tool
may perform the authoring (a distiller for a skill, a domain-model renderer for a
context unit); the harness owns the gate and the resulting registration.

A promotion is complete only when it carries proof that it covers the class:
the new check fails on the recorded instances and passes on accepted work.
This is NO-PROOF-NO-CLOSE applied to the harness's own learning.

**Exit code 13** — a lesson promotion to an executable owner is pending approval.

### 11.3 Configuration

```toml
[lessons]
recurrence_threshold = 2         # instances of a class before it becomes a candidate
promote_executable   = "gated"   # gated | manual — executable owners are never automatic
```

______________________________________________________________________

## 12. Arc Resolution Types: Let the Outcome Choose the Artifact

**Closes:** give one agent the whole job. Today every arc flows
Strategy→Execution→…→Ops toward a code change. Some jobs should close as an
investigation, an instrumented experiment, or a decision not to build.

An arc gains a `resolution` chosen at Strategy (revisable):

| Resolution      | Closes with                                | Proof required                                                             |
| --------------- | ------------------------------------------ | -------------------------------------------------------------------------- |
| `change`        | a merged/deployed code change              | the deployment's configured change contract (§SPEC 3.1 `[proof.contract]`) |
| `investigation` | an analysis or answer, no code             | the sources inspected and the reproducible finding                         |
| `experiment`    | an instrumented painted-door surface       | the instrumentation and the readout that answers the product question      |
| `decision`      | a recorded decision (often "do not build") | the evidence and the rationale behind the call                             |

NO-PROOF-NO-CLOSE still holds; the **proof type** varies with the resolution.
`adh arc close <id> --as <resolution>` records it, and `adh proof verify` selects
the matching proof contract. Stages that do not apply to a non-`change`
resolution (Execution, Ops) are skipped, and the skip is recorded.

A `decision` arc's proof is an **ADR** in the standard format — *Status*, *Context*
(what forced the decision), *Decision* (what was chosen), and *Consequences* split
into **Easier** (what improves) and **Harder** (the trade-off accepted), with
supersession links between ADRs. The Easier/Harder split is deliberate: it records
not just the choice but its cost, which is exactly what a nonfunctional-requirement
trade-off is. The recorded ADR is also routable as a `decision` context unit
(§10.2) — so a decision made once teaches every later arc, the accretive loop
applied to judgment rather than code.

The `change` contract is **generic by default and configurable per deployment**
(§SPEC 3.1 `[proof.contract]`): the harness holds a `change` arc to whatever
acceptance bar the repository declares. The original oracle + invariant +
on-device triad (§SPEC 4–5) is one such profile — a mobile-port deployment's
choice — not a built-in requirement. A deployment with no game and no device
simply defines a code-level contract (tests, review, CI), and the adb/device and
differential-oracle checks fall away as that domain's optional plugins.

**Exit code 8** (proof failure) already covers a missing resolution-matched
proof; no new code needed.

______________________________________________________________________

## 13. Tool Registry and Legibility

**Closes:** make capabilities legible and operable. Today `oracle diff`,
`invariants`, and `device validate` are hardcoded. A registry lets a stage
discover, select, invoke, interpret, and repair capabilities through one loop.

### 13.1 Commands

| Command                    | Purpose                                                                                                               |
| -------------------------- | --------------------------------------------------------------------------------------------------------------------- |
| `adh tool list`            | Registered tools with declared inputs, outputs, and verification.                                                     |
| `adh tool show <id>`       | A tool's contract and last-run health.                                                                                |
| `adh tool run <id> [args]` | Invoke a tool; return its structured result.                                                                          |
| `adh tool doctor`          | Check each tool is discoverable, invocable, and returns a parseable result; report the repair hint for any that fail. |

### 13.2 Configuration

```toml
[[tools]]
id          = "oracle-diff"
run         = "make oracle-diff"
result      = "json"             # structured result the harness can interpret
verifies    = "reference-vs-native equivalence"
repair_hint = "rebuild both targets; see docs/oracle.md"
```

The built-in oracle, invariant, and device checks are re-expressed as registry
entries so the surface is uniform and extensible. A stage selects a tool by what
it `verifies`, not by a hardcoded command, and a failed invocation returns its
`repair_hint` instead of an opaque error.

**Exit code 10** — a required tool is unavailable or returned an uninterpretable
result.

______________________________________________________________________

## 14. Worker Requalification

**Closes:** hold the worker constant. The model-gate (§SPEC 5.1) is a floor; it
does not requalify the environment when the worker changes. A model or agent
change opens a new adoption epoch that must be requalified before normal runs.

### 14.1 Commands

| Command                | Purpose                                                                                                                                          |
| ---------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------ |
| `adh worker show`      | Current epoch, the model bound to each role, and the last requalification.                                                                       |
| `adh worker requalify` | Run the qualification suite on the current models; record a baseline (ambition ceiling, inner-loop latency, tool reliability, failure taxonomy). |

### 14.2 Behavioral Spec

When any model in `[models]` changes from the value recorded for the current
epoch, `adh` opens a new epoch and **`adh run` refuses** until
`adh worker requalify` records a baseline for it. Requalification re-runs a fixed
benchmark arc set, recalibrates the ambition ceiling until proof fails, and
records inner-loop latency as part of usable capability. Learned scaffolding the
new worker no longer needs is flagged for retirement.

```toml
[worker]
epoch          = "2026-07-24-a"
requalify_suite = ".adh/requalify"   # benchmark arcs re-run on worker change
```

**Exit code 9** — worker changed; requalification required before `adh run`.

______________________________________________________________________

## 15. Maintenance Loops

**Closes:** run known work as a continuous loop. Periodic self-eval scores
health but is not a durable loop with owned state and a retirement condition. A
maintenance loop keeps a named invariant true and can run autonomously up to its
gates, like the daily dependency scanner the practice describes.

### 15.1 Commands

| Command                | Purpose                                                          |
| ---------------------- | ---------------------------------------------------------------- |
| `adh loop list`        | Registered loops, their goal, last run, and health.              |
| `adh loop run <id>`    | Run one iteration; it may spawn arcs under the loop's authority. |
| `adh loop retire <id>` | Close a loop whose retirement condition is met.                  |

### 15.2 Configuration

```toml
[[loops]]
id         = "dep-scan"
goal       = "no known-vulnerable dependency ships"
sensor     = "adh tool run dep-scan"
on_finding = "open arc"            # authorized autonomous action, up to the arc's gates
schedule   = "daily"
retire_when = "dependency policy moves into a repository-owned check"
owner      = "security"
```

A loop answers the five closure questions the practice names: the invariant to
keep true, the signal of departure, the evidence of restoration, which actions
proceed autonomously versus at a gate, and the durable state carried to the next
run. A loop never removes a gate; it only automates the safe launches between
them (§SPEC 6).

______________________________________________________________________

## 16. Effectiveness Accounting

**Closes:** optimize for measured effectiveness. Self-eval reports health and
counts; effectiveness is useful outcomes per unit of scarce human attention, at
acceptable cost.

`adh selfeval` gains, and `adh metrics` reports, the following per period and as
a trend keyed to the harness revision:

| Dimension   | Measures                                                           |
| ----------- | ------------------------------------------------------------------ |
| attention   | human minutes per closed arc: steering, review, approval, recovery |
| flow        | worker duration, wall-clock, and acceptance-rate percentiles       |
| rework      | attempts, reversions, and critic/CI cycles per accepted arc        |
| compute     | tokens, inference cost, and CI minutes per accepted arc            |
| compounding | the above as a delta versus the prior harness revision             |

```toml
[metrics]
track_by = "harness_revision"     # attribute trends to environment changes, not vibes
```

The self-eval regression signal (§SPEC README) is redefined against
**effectiveness**, not activity: more arcs at higher attention cost is a
regression, not progress.

______________________________________________________________________

## 17. Credential Custody and Agent Identities

**Closes:** the authority residuals. The base gates are strong on *irreversible
actions* but hold credentials as environment variables. Custody belongs outside
the trajectory, and grants belong to identities.

### 17.1 Configuration

```toml
[authority.broker]
enabled = true                    # resolve scoped credentials at the action boundary
# secrets are never written to config, state, logs, or model context

[[authority.identities]]
id     = "critic"
access = "read-only"              # read-only | mutating
scopes = ["repo:read", "ci:read"]
revocable_endpoint = true         # disable this route without rotating the key
```

### 17.2 Behavioral Spec

A tool receives credentials from the broker at the action boundary; the model
context and `state.json` never contain key material. Each stage runs under a
named identity whose grant is scoped, endpoint-revocable, and audited. A grant
can be withdrawn while an arc is still running, and the next action under it
fails closed at the gate rather than proceeding on a cached secret.

This preserves the base spec's rule (the agent cannot self-grant) while adding
least privilege and revocation the current env-var model cannot express.

______________________________________________________________________

## 18. Harness Self-Optimization

**Closes:** the harness never trains itself. adh proves a *code* change correct
but has no ratcheting, held-out, reflect-driven loop for improving its own
guiding artifacts, so it re-catches the same class of mistake every arc. This
section ports the skill-optimization machinery — a guiding document trained like
frozen-weight model weights, gated on a held-out split — so the harness
compounds: an accepted change makes later arcs cheaper or better, and one that
does not is reverted. It is the engine that generates and validates the
promotions of §11 and the context units of §10.

The worker stays frozen (§14): self-optimization tunes the *environment*, not
the model. A worker change opens a new epoch and invalidates the held-out
baseline, so requalification precedes self-optimization.

### 18.1 the Trainable Artifact and Two Roles

The **harness artifact** is the editable guiding text — the stage prompts
(Strategy, Critic), the context units and skills (§10), and any managed memory
doc. The game code is not the artifact here; this loop optimizes how the harness
*guides* work, not the product.

Two model roles, configured independently:

- **worker** — runs the stages and produces arcs (the frozen target); never
  edited by this loop.
- **optimizer** — reflects on arc history, proposes edits, and scores the
  judge-only rubric dimensions; never runs an arc.

Edits are bounded and confined. A **protected region** marked
`<!-- ADH:LEARNED START --> … END -->` is the only part of a managed doc the
optimizer may write; hand-authored content outside it is preserved. An **edit
budget** (the "learning rate") caps edits per round; the optimizer ranks its
proposals and the budget clips the rest.

### 18.2 the Improvement Gate (Comparative Ratchet)

adh's existing gates are pass/fail on one arc (§SPEC 5). Self-optimization adds a
**comparative ratchet** on a held-out split.

| Command                                               | Purpose                                                                                                                                                         |
| ----------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `adh harness eval`                                    | Score the current artifact: a deterministic floor plus explicit judge-only dimensions, over the held-out selection split.                                       |
| `adh harness gate --candidate S --current S --best S` | Keep/revert. Strict `>` at both comparisons: accept only if the candidate beats current; promote to best only if it also beats best. Exit 0 = keep, 1 = revert. |
| `adh harness hash <artifact>`                         | Content identity (`sha256[:16]`); an unchanged hash is a no-op edit and skips re-evaluation.                                                                    |

Rules, ported directly:

- **Held-out split.** Accept a candidate only on a **selection** set held out
  from the arcs that motivated the edit; report on a separate **test** set never
  used for acceptance. A change that only helps the arcs it was written from is
  overfitting, not improvement.
- **Strict improvement.** Ties reject. The ratchet is monotonic; noise never
  accumulates.
- **Bounded, attributable rounds.** The edit budget caps how many edits a round
  applies and the gate accepts or rejects the candidate as a whole; keep rounds
  small (ideally one region) so a score change is attributable to it.
- **No-op and size guards.** An edit whose hash is unchanged is rewritten, not
  committed; an artifact that grows past `size_bound` is trimmed first.
- **Plateau stop.** Two consecutive kept rounds below a delta threshold, or a
  revert, ends the loop for that artifact — a local ceiling, not a failure.

The rubric makes the judgment boundary explicit: the deterministic floor docks
only detectable defects (missing sections, broken intra-artifact links, banned
phrasing) and assumes the judge dimensions are perfect; the optimizer supplies
the judge-only bases. Compare candidates on the same metric within a run.

**Outcome eval and replication — a strict `>` is not proof.** The rubric/floor
gate above is a *structural* screen (does the artifact score better); it is cheap
and necessary but **not sufficient** to adopt a change, because a single
comparison cannot separate a real effect from stochastic noise — accepting on it
is arguing with a random number generator. Adoption additionally requires an
**outcome** eval and a replication guardrail:

- **Outcome, not structure.** Measure whether the change actually improves the
  agent's *result* on a task set — a **condition-blind, paired** comparison
  (control = the run without the change, treatment = with it) over the held-out
  selection split, with the treatment **token-budget-balanced** against the control
  so the effect is not merely "more text." Objective outcomes (correctness,
  abstention, calibration/Brier, localization) are preferred; a **validated**
  judge scores what only judgment can.
- **Effect size + a paired test.** Require a minimum effect (e.g. ≥5 percentage
  points) *and* a passing paired significance test (McNemar for paired binary
  outcomes; cluster-robust resampling when items are not independent), with a
  multiple-comparison correction across dimensions.
- **Replication verdict.** The disposition is a taxonomy, not a boolean:
  **ELEVATE** only when a primary pass **and an independent fresh replication**
  both hold; a lone pass is **DIRECTIONAL-NOT-REPLICATED** (do not adopt); a
  missing replication **refuses** to elevate (REPLICATION-MISSING). The verdict is
  a pure, unit-testable function (FCIS). **KILL** and not-replicated outcomes are
  recorded honestly (§18.6), never hidden — publishing a negative result is the
  point of the gate.
- **Validate the graders and the splits.** The judge is calibrated before its
  verdicts are trusted, and the selection/test splits are checked for leakage — a
  grader you can weaken, or a split that leaks, makes the whole ratchet a proxy.

This composes with an external skill optimizer (§13, e.g. skillsaw): its rubric
`gate` is the cheap structural floor, and this outcome+replication verdict is the
adoption bar on top of it. The same discipline governs §16 effectiveness claims —
a comparative, causal, or longitudinal claim needs repeated conditions, parity,
condition-blind grading, and evidence it generalizes, not one before/after run.

### 18.3 Reflect Discipline

The optimizer reflects over arc history to propose edits, following four rules
the failure registry (§SPEC) does not encode:

- **Success and failure modes.** Extract both what to reinforce (success modes)
  and what to avoid (failure modes); failure modes win a conflict.
- **Defect vs lapse.** Classify each miss as a **harness defect** (a stage prompt
  or context unit is wrong, missing, or underspecified → edit the artifact) or an
  **execution lapse** (a correct instruction the worker did not follow → do not
  rewrite the rule; record a reminder). When uncertain, treat it as a lapse and
  protect the artifact. This stops the loop from churning correct guidance.
- **Longitudinal categories.** Compare the candidate against the previous best
  per arc: **regressed** (was passing, now fails) is the highest priority, ahead
  of persistent failures and new successes. A change that fixes one class by
  breaking another is caught here, not in production.
- **Rejected-edit memory.** Record every reverted edit with its score delta so a
  later round does not re-propose it. The negative-feedback buffer is part of the
  optimizer's context.

### 18.4 Offline Consolidation (`adh sleep`)

A scheduled, offline loop that turns operating history into staged, gated harness
improvements without touching live files.

| Command                                       | Purpose                                                                                                                           |
| --------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------- |
| `adh sleep run`                               | One consolidation cycle: harvest → mine → reflect+gate → stage.                                                                   |
| `adh sleep adopt <staging-id>`                | Apply a staged, accepted proposal to the live artifacts, backing them up first. The only command that mutates live guiding docs.  |
| `adh sleep status`                            | Show staged proposals and the last cycle.                                                                                         |
| `adh sleep schedule add <name> <cron> <cmd…>` | Register a cron job firing an adh command (e.g. `sleep run`, `loop run dep-scan`). Quote a multi-field cron.                      |
| `adh sleep schedule list`                     | List scheduled jobs with cadence, next fire, and last outcome.                                                                    |
| `adh sleep schedule remove <name>`            | Remove a scheduled job.                                                                                                           |
| `adh sleep schedule tick`                     | Run every job due now, record each outcome, and advance its next fire. Drive it from one system-cron line (or the agent).         |
| `adh sleep schedule run`                      | Blocking daemon: sleep to the next deadline, fire due jobs, repeat until SIGINT/SIGTERM. Launch with `--repo`; run one per store. |

The cycle:

1. **Harvest** closed arcs and their proof, reviews, and human corrections since
   the last cycle. Read-only; no network.
2. **Mine** recurring, checkable tasks from that history and assign each to a
   stable split. Real tasks are archived so later cycles can recall similar ones;
   recalled and synthetic tasks may enlarge the *training* set only, never enter
   the held-out splits, and a real task's split is stable across cycles.
3. **Reflect and gate** (§18.2–18.3): propose bounded edits, apply them to a
   candidate, and accept only a strict held-out improvement.
4. **Stage** the accepted proposal — proposed artifacts, a report, and a manifest
   — under `.adh/sleep/staging/<id>/`. Live files are untouched here, and all
   staged text passes through secret redaction.
5. **Adopt** only on explicit human approval (`adh sleep adopt`), which backs up
   the live files first. Adoption is a §5.2 human gate; `--yes` never satisfies
   it, and a staged proposal awaiting adoption exits **14**.

A deterministic **negative-control self-test** proves the gate has teeth before
the loop is trusted: a planted harmful edit, a known regression, is fed through
the same held-out gate and must be rejected because it fails to improve the
selection score. Run it at startup or in CI; if the gate ever accepts the
regression it is untrustworthy and self-optimization stops with exit **15**. This
is the differential-oracle negative control (§SPEC 4) applied to the
self-optimization gate.

### 18.5 Configuration

```toml
[harness]
artifact_roots   = [".adh/context", ".adh/prompts"]  # editable guiding text (§10)
protected_marker = "ADH:LEARNED"    # only this region is machine-edited
edit_budget      = 4                # max edits per round (the learning rate)
size_bound       = 1.5              # candidate ≤ 150% of the current artifact

[harness.gate]
selection_split = ".adh/eval/selection"   # arcs held out for acceptance
test_split      = ".adh/eval/test"        # arcs reported on, never used to accept
metric          = "effectiveness"         # ratchet on §16 effectiveness, not arc count
strict          = true                    # ties reject

[sleep]
schedule   = "nightly"
backend    = "mock"     # mock runs the full cycle deterministically, incl. the gate self-test
redact     = true
auto_adopt = false      # staging-only unless a human opts in
```

### 18.6 Evidence Log and No-Improvement Diagnostics

Every cycle appends a redacted, size-capped **evidence log**
(`.adh/sleep/staging/<id>/evidence.jsonl`) binding a staged proposal to its full
provenance: the config snapshot, the harvested arcs and their signals, each mined
task with its split assignment and checks, the baseline and candidate held-out
scores, the applied and rejected edits, and tokens spent. An accepted harness
edit is therefore auditable — a reviewer reads the chain to see *why* it was
accepted, the way §SPEC 4 proof binds a release artifact to the evidence that
covers it.

A cycle that accepts nothing must **self-explain**. It records, per held-out
task, the hard/soft score and the reason (empty optimizer reply, failing checks,
or no proposed edit), plus any backend call error. Without this a `0.0 → 0.0`
cycle is a black box and the loop cannot be debugged. The evidence log and these
diagnostics stage alongside the proposal and never touch live files.

______________________________________________________________________

## 19. Cold-Critic Grounding and Finding Disposition

**Closes:** the repository must teach the critic, and a critic's finding is a
hypothesis the repository adjudicates, not a verdict the model is trusted for.
§SPEC 5.3 makes the critic *cold* — a fresh context with no builder transcript —
but leaves two things unstated: what grounds the critic, and what its findings
do. The stage seam shows the gap. The critic runs on a placeholder prompt, and
its output is appended to arc history with no adjudication. A finding neither
blocks the arc nor becomes a durable check.

The standard of good lives in the repository, not in the model's priors. A cold
critic is grounded by repository-owned artifacts and withholds only the build
narrative; its findings are confirmed against those same artifacts before they
change an arc's course.

### 19.1 the Critic's Grounding Contract

`adh critic <id>` loads its working set from repository-owned state and denies
exactly one input, the Execution transcript. The working set is:

- the change under review — the diff and the arc's touched paths;
- the proof packet the builder left and the arc's acceptance bar (§SPEC 5.4);
- the context units routed for the arc's labels and paths (§10), including any
  NFR check that encodes a nonfunctional requirement.

"Cold" is an isolation boundary on the builder's reasoning (§SPEC 5.3), not a
context boundary on the repository. A critic forced to reason from its own
priors because the environment did not teach it records a routing gap (§10, exit
12), not a property of cold review.

A routing gap presupposes an *environment* to have failed: it applies only when a
context store exists (holds units) yet routes nothing for a declared arc that
also left no proof. A repository with no context store is simply ungrounded —
grounding is not configured — and the critic runs on the change and its proof
alone, which the prompt states plainly; this is not exit 12. `adh init`
scaffolds a starter store so grounding is on by default, and Execution labels an
arc by the areas it touched (§SPEC 5.4) so its context routes.

### 19.2 Finding Disposition — Confirm Against the Repository

A critic emits findings. Each finding names the repository artifact that would
confirm it: an oracle divergence, an invariant, an on-device check (§SPEC 2.4),
an NFR check (§10), or a named local contract. The Evaluation stage that follows
the critic (§SPEC 1) runs that artifact.

| Finding                                           | Adjudication                       | Effect                                                                                           |
| ------------------------------------------------- | ---------------------------------- | ------------------------------------------------------------------------------------------------ |
| confirmed — the named artifact fails              | a deterministic Evaluation failure | returns the arc to Execution and records a failure-registry entry (§SPEC 4.1; exit 5–7)          |
| unconfirmed — no artifact fails, or none is named | a lesson candidate (§11)           | does not block the arc; recorded for promotion to a repository-owned check once the class recurs |

A finding never blocks on the critic's text alone. This is §9's non-goal applied
to review: the harness does not grade a change with an LLM-as-judge where a
deterministic check can decide, so a critic's judgment either points to a check
that already decides the change or becomes one. A bad prior invents a
requirement the repository does not hold, and the gate drops it; a prior that
holds up is a real gap in the acceptance bar, and §11 turns it into a durable
check the next arc meets deterministically.

The human gate at Ops (§SPEC 5.2) is unchanged. An operator may still reject, on
judgment, a change that every automated check passed. Authority to block on
taste stays with the human, never the critic.

### 19.3 Convergence

The critic runs once per arc (§SPEC 1), so it cannot widen a change across review
rounds today. A critic-and-author revision loop or an auto-advancing review, if
added later, carries a merge bias and a bounded set of author responses per
finding so review ends. Until then this is a recorded design constraint,
not a runtime gate.

### 19.4 Configuration

```toml
[critic]
ground_from = ["diff", "proof", "acceptance_bar", "context"]  # §10 routed set
deny        = ["transcript"]   # the one input cold review withholds
unconfirmed = "lesson"         # a finding with no failing artifact is a §11 candidate, never a blocker
```

This section adds no exit code. A confirmed finding surfaces through the existing
Evaluation gates (exit 5–7); an unconfirmed one is a §11 lesson candidate, and
its later promotion to an executable owner is the existing gated path (exit 13).

______________________________________________________________________

## Exit Codes (Additions)

| Code | Meaning                                                                                                                   |
| ---- | ------------------------------------------------------------------------------------------------------------------------- |
| `9`  | Worker changed; requalification required (§14).                                                                           |
| `10` | Required tool unavailable or uninterpretable (§13).                                                                       |
| `12` | Routed context unit missing or unresolved (§10).                                                                          |
| `13` | Lesson promotion to an executable owner pending approval (§11).                                                           |
| `14` | Harness change staged; human adoption required (§18.4).                                                                   |
| `15` | Self-optimization gate self-test failed — a planted regression was not rejected; the gate is untrustworthy, stop (§18.4). |

Code `11` is reserved to avoid collision with a future gate class; `8` (proof
failure) already covers resolution-matched proof (§12). §19 adds no code: a
confirmed critic finding surfaces through the Evaluation gates (exit 5–7) and an
unconfirmed one through the lesson path (§11).

______________________________________________________________________

## What These Deliberately Do Not Add

Consistent with §9 Non-goals, these additions keep judgment out of the tool. The
context store routes units but does not decide a task's intent; `adh lesson`
proposes a durable owner but a human approves any executable one; resolution
types record how an arc closed but Strategy chooses it; requalification measures
capability but does not set ambition for the operator. Self-optimization (§18)
proposes and scores harness edits, but a deterministic strict-`>` gate on a
held-out split accepts them and a human adopts them; the worker stays frozen
throughout. The cold critic proposes findings, but a repository artifact
confirms them or a human rejects at Ops; it never blocks on its own text. The
harness gains the context and tool levers and a compounding loop; it does not
gain the authority to apply judgment on its own.
