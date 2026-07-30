# Harness-Engineering Improvements for Adh

*Temporary planning document. Goal: let a Claude/Gemini agent use the adh skill to
operate the adh CLI as the harness for a team whose only levers are the **context**
they expose and the **tools** they make available — turning every corrective
interaction into an accretive change to that environment.*

> **Revised per `harness-engineering/AGENTS.md`.** The first draft was a five-item
> backlog. The working loop and the `improve-harness` playbook reshaped it into
> bounded operational loops — baseline → earliest failed handoff → smallest owning
> intervention → native verification → fresh rerun → retain/revise/remove — each
> classified by gap, routed to one thesis, and stated as a falsifiable hypothesis.
> The AGENTS.md deltas are listed at the end.
>
> **Then folded in `modelith`** (Apache-2.0, stacklok) as the domain-modeling leg:
> it owns the domain model A routes and F gates, supplies the deterministic half of
> E (`lint`), and adds Loop F (a context-integrity anti-drift gate via `render
> --check`) — a capability adh lacked. Boundary: modelith owns domain *semantics*,
> not NFR *governance*; see the boundary note.

## The Ecosystem This Must Orchestrate (Adapt, Do Not Copy)

The team already built the mechanistic pieces as standalone CLIs — distillation,
skill-optimization, and domain-modeling. Per AGENTS.md ("adapt the applicable ideas
without copying file layouts, policies, version pins, or fixtures"), adh
**orchestrates** them; it does not re-implement or vendor them.

- **exegesis** (`github.com/StevenACoffman/exegesis`, MIT, Go/ff/v4) — distillation:
  a curated knowledge base → RIA++ skill packs with triple-validation gates,
  Zettelkasten links, `test-prompts.json`, and a `## Provenance` line
  (`Source: 《title》 author — chapter`). `distill --driver agent` emits JSON prompts
  and pauses for model responses — adh's own relay pattern. Verbs: `distill`,
  `verify`, `lint`, `tests`, `index`, `link`.
- **skillsaw** (`github.com/StevenACoffman/skillsaw`, MIT, Go/ff/v4) —
  the SkillOpt/SkillLens engine: `eval` (9-dim rubric, deterministic +
  `needs_judge`), `diagnose`, `scan` (runtime-neutrality), `gate` (strict `>`
  ratchet), `judge` (rule checks), `hash` (no-op guard), `history`. All `--json`;
  the CLI never calls a model — judgment is deferred to the agent.
- **modelith** (`github.com/stacklok/modelith`, Apache-2.0, Go/cobra) — the
  domain-modeling leg: an agent authors a canonical domain model
  (`model.modelith.yaml` — glossary, enums, entities with relationships/attributes/
  actions/**invariants**, scenarios, model-level invariants, `imports`) by
  conversation. `lint` (validate + completeness, `--format json`) does real
  cross-item consistency (reciprocal-cardinality conflicts, mutual-ownership,
  duplicate invariant ids, subtypeOf cycles, dangling references); `render`
  emits Markdown + a Mermaid ER diagram with a `--check` byte-for-byte **drift
  gate**; `deps import` vendors a shared model with a provenance header
  (origin/ref/commit/**digest**) that `lint` re-verifies. Its `plugin/` ships three
  Claude skills — including `domain-model-context`, which loads a model into context
  before a task (prior art for Loop A). NFRs are **deliberately not** first-class
  (see the boundary note below).

**Supply chain (target-local authority governs):** exegesis and skillsaw are MIT,
modelith is Apache-2.0 — all three are freely recommendable and pinnable. The
external-tool integration is the right call on **architecture, not licensing**:
skillsaw's scoring logic lives entirely in `internal/` packages (Go forbids any
other module from importing them, so there is no library API to vendor even with a
license), modelith is cobra-based (and adh's depguard bans cobra in first-party
code), and invoking all three through the §13 tool registry (`Run` commands the
operator installs) matches adh's own model and the team's "offload the mechanistic
to CLI tools" discipline while keeping adh's build decoupled from their release
cadence. The licenses only change that adh may now freely recommend, require, and
pin them; the earlier `eon` blocker applies to none of them.

**Boundary — what modelith is not.** modelith owns *domain semantics* (what the
system **is**: concepts, relationships, invariants). It **deliberately omits**
first-class nonfunctional requirements, quality attributes, and prioritization/
trade-off decisions — its own docs say so, and its `lint` does not validate them.
So it is the durable owner for the team's *domain model and rule invariants*, not
for their *NFR governance*. NFRs still need their own owner: exegesis-distilled
skills/rules, §10 NFR-check context units (executable checks in the tool registry),
or a parallel policy artifact — the `domain-modeling` thesis's "represent NFRs as
context, examples, types, tests, or policy." Do not over-route NFR authority into
modelith; keep it the domain-semantics leg.

## Scope, Authority, and the Job Contract

Per `improve-harness.md` ("Establish scope and authority" / "Record the job
contract"). adh's contracts are the local truth the tools sharpen but cannot
override; the harness-engineering corpus is read-only context.

```text
Target and revision:      adh @ implement/adh-spec (this repo)
Fixed worker:             Claude/Gemini driving adh via .claude/skills/adh-relay
Representative job:       Route a team's distilled security base-rule + skill into
                          a change arc's cold critic so the worker applies the
                          local NFR, then make a human correction to that arc
                          accretive — a durable, routed unit the next arc inherits.
Accepted outcome:         The critic reasons over the actual rule text (not just a
                          unit id); the correction becomes a promoted context unit
                          with provenance that a later arc routes and applies.
Evidence:                 arc history (Arc.Context), the staged/created unit + its
                          provenance, a fresh arc that routes and applies it.
Authority envelope:       §5.2 human-approval gates and the model-gate remain
                          binding; skillsaw's gate feeds Evaluation but never
                          replaces NO-PROOF-NO-CLOSE or human approval. No
                          auto-adopt (§18 auto_adopt = false).
Budget / stop:            One intervention per loop; smallest reversible change.
Suspected gaps:           context delivers references not content; provenance lost
                          on entry; promotion doesn't materialize; no cross-unit
                          consistency; the improvement proposer is mock.
```

## Baseline: the Earliest Failed Handoffs (Gap-Classified)

Traced with the `improve-harness` taxonomy; each names the earliest authoritative
owner and the one thesis route that resolves the unresolved decision.

| #   | Observed gap                                                                                                                                                    | Earliest owner                      | Gap class                    | Thesis route                              |
| --- | --------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------- | ---------------------------- | ----------------------------------------- |
| 1   | Unit is `{id,kind,labels,paths,owner}`; `Load` reads only metadata; prompt renders `- <id> (<kind>)`. The rule/skill **text** is never delivered or pointed at. | `internal/contextstore` + grounding | **Context**                  | `just-in-time-context`                    |
| 5   | No `provenance`/source field on a unit; exegesis's `## Provenance` is dropped on entry.                                                                         | `contextstore.Unit`                 | **Proof / Domain ownership** | `proof`, `domain-modeling`                |
| 3   | `lesson promote` gates + prints "promoted" but creates no durable owner.                                                                                        | `cmd/lessoncmd`                     | **Feedback / delivery**      | `feedback`, `last-mile-deployment`        |
| 4   | `context lint` checks id/kind only — no conflict between skills or against base rules.                                                                          | `cmd/contextcmd`                    | **Domain ownership**         | `domain-modeling`, `feedback`             |
| 2   | `consolidate.Propose` is canned; `sleep` isn't relay-wired — Claude can't drive improvement.                                                                    | §18 `sleep`                         | **Capability**               | `continuous-maintenance`, `effectiveness` |

## Intervention Loops

Each is one bounded pass. New machinery must earn its maintenance cost against the
observed gap; prefer the smallest reversible change at the earliest owner.

### Loop a — Context Units Carry a Content Route + Provenance (Gaps 1, 5)

*Route: `just-in-time-context` — "tell the worker what kind of context exists and
where to find it, and let the task pull the next relevant slice." Do **not**
stuff full text into the prompt (deterministic context stuffing fails across
compaction).*

**Hypothesis.** If a content route (`Path`) and a `Provenance` field are added to
`contextstore.Unit`, `context show <id>` returns the unit's text + provenance, and
the grounding previews `- <id> (<kind>) — <one-line> — see <path>`, then the fixed
worker will reason over the actual base-rule/skill text on the representative job
(retrieving the slice when the stage reaches the decision it governs), because the
routing layer now makes the curated slice retrievable at its source rather than
naming an id the worker cannot open.

- *Evidence for:* the critic cites the rule's specifics; `Arc.Context` records the
  retrieved unit. *Against:* the worker still ignores the routed unit → the gap is
  worker-limitation, not context.
- *Content that drops in:* an exegesis skill pack (`SKILL.md` + `## Provenance`) or
  a **modelith-rendered domain model** (`*.modelith.md`, kind `domain-model`) is a
  ready routable unit — the `modelith` `domain-model-context` skill already loads
  one into context, so adh's job is only to *route* it. For provenance, mirror
  modelith's **proven-provenance** model (origin / ref / commit / **digest**,
  re-verified on read) rather than a bare source string — proven artifact identity,
  not prose. Carrying cost: one content-route field, one read path, one digest.
- *Verify (two layers):* (1) native — `go test -race`, `golangci-lint run ./...`;
  (2) journey — `context route <arc>` previews the right unit, `context show`
  returns text + provenance, a relayed critic stage retrieves and applies it.
- *Fresh rerun:* a new arc in an isolated workspace routes the same unit and the
  critic applies the NFR without human relay. *Retain* if applied and cited.

### Loop B — Register Exegesis, Skillsaw, and Modelith as §13 Tools (Tool Legibility)

*Route: `tool-legibility` — the capability exists; the gap is discovery, invocation,
and interpretation inside a trajectory.*

**Hypothesis.** If exegesis, skillsaw, and modelith are registered as tool-registry
entries whose `Run` invokes the installed binaries with `--json` where available
(and `Verifies`/`RepairHint` set — e.g. `modelith lint --format json`,
`modelith render --check`, `skillsaw eval --json`), then the worker will invoke them
via `adh tool run <id>` and interpret their output in-loop, because the registry
makes an already-capable but undiscoverable tool legible where the arc needs it.

- *Adapt, don't copy:* external-tool `Run` commands only — no Go dependency, because
  the logic is in unimportable `internal/` packages (skillsaw) or a cobra CLI adh's
  depguard bans (modelith), and CLI invocation is adh's model regardless of license.
  *Verify:* `adh tool run modelith-lint`/`skillsaw-eval` returns JSON; the tools
  appear in a review stage's routed tool set.

### Loop D — Lesson Promotion Materializes the Durable Owner (Feedback/delivery)

*Route: `feedback` — an accepted lesson must survive the trajectory as a durable
owner, not a printed intent.*

**Hypothesis.** If `lesson promote --to context|skill|check` writes the artifact
(a `.adh/context/<id>.json` unit with provenance per A; an exegesis skill-pack
scaffold; a §13 tool entry) behind the existing §11.2 approval gate, then a human
correction becomes a routed unit the next arc inherits, because promotion now
performs the accretion instead of recording the decision.

- *Target-local authority governs:* the executable-owner approval gate (exit 13)
  stays; `--yes` never satisfies it. *Verify:* `lesson promote --to context …
  --approve` creates a unit that `context route` (Loop A) then selects and applies.

### Loop E — Cross-Unit Consistency Check (Domain Ownership)

*Route: `domain-modeling` — "repeated interpretation indicates missing semantic
ownership"; a conflict between a skill and a base rule is exactly that.*

Split by what can be made deterministic vs. what is irreducibly judgment — the
team's own offload discipline:

- **Deterministic layer (owned by modelith).** For any concern expressed as a
  domain model, `modelith lint` already catches real cross-item conflicts
  (reciprocal-cardinality, mutual-ownership, duplicate invariant ids, subtypeOf
  cycles, dangling references). Route it as the §13 check (Loop B); it is a proven
  owner for *reference integrity* within a model — no LLM needed.
- **Semantic layer (relayed critic).** modelith's lint does **not** detect
  contradictions between *unrelated* rules, nor across *documents* (a skill vs a
  base rule vs a domain invariant). **Hypothesis:** if `context lint` (or a new
  `context check`) routes the cold critic over the resolved unit set to flag those
  contradictions, then the worker surfaces a conflict before it silently governs an
  arc, because the consistency judgment gains an owner and a gate. *Verify:* a
  planted contradiction (skill asserts X, base rule not-X, or a domain invariant
  not-X) is surfaced; a modelith reference-integrity error is caught by the
  deterministic check without reaching the critic.

### Loop F — Context-Integrity (Anti-Drift) Gate (Proof; Strengthens A)

*Route: `proof` — the evidence boundary includes the identity of an artifact
across source and render; routed context must match its canonical source.*

**Hypothesis.** If `modelith render --check` (and `modelith lint`, and exegesis's
`verify`) are registered as §13 NFR-checks that an arc runs over its routed
domain-model / skill context, then a stage fails when the routed Markdown has
drifted from its canonical YAML/source, because the arc now *proves* the context it
reasoned over is the current, valid one — not a stale copy. This is a capability
adh lacked entirely: today nothing guarantees a routed unit still matches its
source. *Verify:* editing `model.modelith.yaml` without re-rendering makes the arc's
context-integrity check fail (exit non-zero); re-rendering clears it.

### Loop C — Skillsaw + Relay Improvement Loop (Capability; Compose B and the Gate)

*Route: `continuous-maintenance` — run known work as a loop with owned state,
distillation, and gardening; answer the five closure questions. `effectiveness` —
ratchet on measured improvement.*

**Hypothesis.** If §18 `sleep` (or a §15 loop) drives skillsaw — `eval` baseline →
`diagnose` next dimension → relay asks Claude for the one-dimension edit and the
`needs_judge` scores → `skillsaw gate` (strict `>`) accepts or reverts → record to
adh evidence — replacing `consolidate.Propose`, then Claude drives real,
gated skill improvement through adh, because the mechanistic scoring stays in
skillsaw and only judgment is relayed, with skillsaw's `gate`/`hash`/`history`
supplying the held-out ratchet, no-op guard, and audit that adh's mock lacked.

- *Do not weaken the grader / widen authority:* skillsaw's `gate` feeds adh's
  Evaluation but never replaces human approval or NO-PROOF-NO-CLOSE; the loop
  **stages** (auto_adopt = false). *Verify:* a strict score increase is kept; a
  non-improving edit is reverted (negative control) — adh's existing self-test
  discipline, now backed by a real proposer.

## Sequencing as Successive Bounded Loops

Run A first — it is the smallest change, owns the team's primary lever, and every
other loop composes on it (D materializes into A's unit shape; E, F, and C read
routed content). Then B (tools legible — the exegesis/skillsaw/modelith registry
that F and C then call). Then D and E (accretion + consistency), F (context-
integrity gate, once modelith-rendered models are routed by A), and C last (the
improvement loop composes B + the gate). Each loop closes with its own
retain/revise/remove decision recorded in adh's history before the next begins;
one before/after run is a bounded claim, not a general treatment effect.

modelith's role across these: it is the durable **owner** of the domain model that
A routes and F gates; its `lint` is the deterministic half of E; and its
`domain-model-context` skill is prior art A can mirror. It does **not** own the
team's NFRs (per the boundary note) — those remain exegesis skills, §10 NFR-check
units, or policy artifacts.

## Guardrails Carried from AGENTS.md

- **Target-local truth governs.** adh's gates, authority (§5.2 approval,
  model-gate), and NO-PROOF-NO-CLOSE bind; the tools and corpus sharpen decisions,
  never override contracts.
- **Adapt, don't copy.** Register exegesis/skillsaw as external tools and mirror
  exegesis's provenance *model* — do not import their layouts, fixtures, or pins.
- **Do not widen authority or weaken a grader.** skillsaw's strict-`>` gate feeds
  Evaluation; it does not replace human approval or proof. No auto-adopt.
- **New machinery earns its cost.** Each loop is justified against an observed gap
  and is the smallest reversible change at the earliest owner.
- **Verify at the claim boundary.** Both target-native checks *and* the user
  journey (a real arc that routes/applies the unit), with fresh reruns in isolated
  sessions so the rerun inherits no hidden help.

## What the AGENTS.md Pass Changed Vs. The First Draft

1. Reframed a flat five-item backlog into bounded `improve-harness` loops
   (baseline → earliest gap → smallest owning intervention → verify → rerun →
   retain/revise/remove).
2. Added a job contract and a single bounded representative job, so success is a
   rerun of a real trajectory, not "all gaps closed."
3. Classified every gap with the playbook taxonomy and routed each to exactly one
   thesis (the unresolved decision selects the route).
4. Stated each intervention as a falsifiable hypothesis with evidence-for/against
   and carrying cost, and split verification into native + journey layers.
5. Hardened the guardrails: target-local authority governs, adapt-don't-copy, no
   grader-weakening/authority-widening, and external-tool integration by
   architecture (skillsaw's logic is `internal/`-only; now MIT, so freely
   recommendable as an installed tool).
