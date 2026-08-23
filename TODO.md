# TODO — Outstanding Work on the Adh CLI

Tracks what remains after Phases 0–9 (see [`PLAN.md`](./PLAN.md)). Everything
committed passes the phase gate: `golangci-lint run ./...` clean and
`go test ./...` green.

## Harness-Engineering Improvements — Context/Tools Levers (§10, §11, §13, §18)

Plan and rationale: [`harness_engineering_improvements.md`](./harness_engineering_improvements.md)
(shaped by `harness-engineering/AGENTS.md`). These close the gaps that would hamper
a team using the adh skill to operate adh as their harness, whose only levers are
the **context** they expose and the **tools** they make available, and who need
every correction to be **accretive**. adh **orchestrates** an external, MIT/Apache
toolchain via the §13 registry — it does not vendor or re-implement it:
**exegesis** (distill a knowledge base → skills, with provenance), **skillsaw**
(SkillOpt/SkillLens skill optimization), **modelith** (author/validate/render the
domain model). All three are external CLIs by architecture (internal-only or
cobra-based) — install-and-invoke, never Go dependencies. **Boundary:** modelith
owns domain *semantics*, not NFR *governance*; NFRs remain exegesis skills, §10
NFR-check units, or policy artifacts. Ordered by value to the stated goals; run
each as one bounded baseline → intervention → verify → fresh-rerun → retain loop.

- [x] **Loop A — context units carry content + provenance (§10.4).** DONE (the
      foundation everything composes on). `contextstore.Unit` gained `ContentPath`
      (the store-relative route to the unit's text) + `Provenance`;
      `contextstore.Content` reads it (I/O shell; `Route` stays the pure core; a
      path escaping the store is refused). `adh context show <id>` returns the text
      + provenance (`--jsonl` carries them as one outcome); the `_guidance.tmpl`
      preview now renders `- <id> (<kind>) — see <path>` and tells the worker to
      pull it with `context show` (JIT preview+retrieve, not stuffing); `context
      lint` flags a dangling `content_path`. Verified at the claim boundary — unit
      tests + a journey (a routed strategy prompt previews the content route). An
      exegesis skill pack or a modelith-rendered `*.modelith.md` now drops in as a
      unit's content file. **Follow-up (own loop):** switch the unit *format* to OKF
      markdown+frontmatter with the trust tier + freshness (the separate "Adopt OKF"
      item) — deferred so this cut stayed the smallest reversible change.
- [x] **Loop B — register exegesis/skillsaw/modelith as §13 tools.** DONE. A
      `toolreg.StarterRegistry` declares the four external capabilities as `Run`
      commands with `--json`/`--format json` where available (`modelith lint
      --format json`, `modelith render --check`, `skillsaw eval --json`, `exegesis
      verify`), each with `Verifies` + a `RepairHint` (the install line); `adh init`
      seeds `.adh/tools.json` from it (idempotent — kept if present) so the tools are
      legible out of the box. `adh tool run <id>` resolves the entry and invokes it
      through the shared shell edge (`shell.Runner.RunIO`, added so the child's
      stdout/stderr reach the worker; `Run` now delegates to it, one gosec edge):
      non-`--jsonl` streams the tool's output live and propagates its exit code;
      `--jsonl` captures stdout/stderr/exit into one outcome envelope (`data`) the
      worker parses. An unknown id or an uninstalled binary (shell exit 126/127) is a
      registry-level problem (exit 10, reason `unknown_tool`/`tool_unavailable` +
      repair hint), not a failing check; a tool that ran and exited non-zero
      propagates its own code (reason `tool_failed`). `list`/`doctor` now honor
      `--repo` too (an absent registry is a valid empty one). Verified: unit tests
      (shell/toolreg/toolcmd) + a journey (`init` → `tool doctor`/`list`/`run`,
      including a real `modelith` invocation captured under `--jsonl`). **Follow-up
      (own loops):** appending extra args to `tool run <id> -- <args>`; and wiring
      these entries into F (an arc's context-integrity NFR-check over its routed
      context) and C (the skillsaw improvement loop).
- [x] **Loop D — lesson promotion materializes the durable owner (§11.2).** DONE
      for the reversible content owners. `lesson --to context|doc|decision promote
      <class>` now writes a routable §10 context unit (a content file + its unit
      JSON, `content_path`+`provenance`, routes by the class label) under
      `.adh/context` — so a correction is inherited by the next arc (Loop A routes
      it), not just printed. `decision` writes an ADR skeleton (Status/Context/
      Decision/Consequences: Easier|Harder, §12) — folding in the ADR/`decision`
      item. Pure renderer in `internal/lesson` (`Render`/`Slug`/`Kind`/
      `Materializes`), thin write shell in `lessoncmd`. The executable owners
      (`check`/`invariant`/`type`) and `skill` **keep the §11.2 gate** (exit 13) and
      are *not* auto-written — adh cannot author a correct check/type/skill from a
      class. Verified: unit tests + a journey (promote → `context show`/`route` see
      it; an executable owner still gates, writes nothing). **Follow-up (own loops):**
      materialize an executable owner (scaffold + register a §13 check) and `skill`
      (via exegesis).
- [x] **Loop E — cross-unit consistency (§10.4).** DONE for both halves. The
      *deterministic* layer: `context lint` now catches duplicate unit ids across the
      store (`contextstore.DuplicateIDs`, a pure core), and `modelith` reference-
      integrity is available via `adh tool run modelith-lint` (Loop B) and as a unit's
      `integrity` check (Loop F). The *semantic* layer: `adh context check [arc]`
      assembles the routed unit set + each unit's content into one consistency-review
      packet (a pure assembly; adh gathers deterministically, the relayed agent judges
      contradictions — a skill vs a base rule vs a domain invariant). It surfaces for
      judgment (the agent then promotes a lesson / opens an arc); it is not itself a
      gate. Verified: unit tests (duplicate-id lint, packet assembly) + a journey.
      **Follow-up (own loop):** a *live* relay critic stage over the units (a parked-
      turn adjudication) — `check` gives the packet today; the relay flow is separate.
- [x] **Loop F — context-integrity / anti-drift gate (§10.4, proof).** DONE — a
      capability adh lacked entirely. `contextstore.Unit` gained an `integrity` field
      (a §13 tool id that proves the unit's content has not drifted from its canonical
      source, e.g. `modelith-render-check`, registered in Loop B). `adh context verify
      [arc]` routes the units (all, or an arc's routed set) and runs each declared
      integrity check via the shared `shell` edge: a check that ran and failed is
      **drift** (exit 14, reason `context_drift`); an uninstalled check tool is
      *unverified* (reported, not a gate failure — the `unrunnable = unconfirmed` rule
      the Evaluation stage uses, centralized as `shell.NotRun`); a unit naming an
      undeclared tool is a store misconfiguration (EINVALID). Verified: unit tests
      (clean/drift/unverified/misconfigured) + a journey (editing the check to fail
      exits 14, fixing it clears). The `context-drift` §15 loop (below) runs it as a
      sensor.
- [x] **Loop C — skillsaw + relay improvement loop (§18).** DONE for the relay half.
      `sleep run --relay` sources the optimizer edit from the driving agent instead of
      the mock `consolidate.Propose`: with no reply it emits a proposal prompt (the
      ranked reflection modes + the artifact's current LEARNED region, `consolidate.
      ProposePrompt`, a pure core) and parks statelessly; `--relay --response <file|->`
      resumes with the agent's edit and feeds it to `Plan`. adh's own held-out ratchet
      still gates it (skillsaw's `gate` feeds, never replaces — the agent consults `adh
      tool run skillsaw-eval` from Loop B for dimension scores) and staging is unchanged
      (auto_adopt = false, exit 14), so a non-improving relay edit is still rejected.
      Verified: `--relay` emits a prompt naming the reflected class; an empty edit is
      gated out (stages nothing). The skillsaw ratchet path now ships: `harness gate`
      already *is* the strict-`>` ratchet, and `internal/skillsaw.Decode` (a pure
      parse-at-the-boundary decoder of skillsaw's `eval --json`) + `harness eval
      --skillsaw <file>` surface skillsaw's score and needs-judge dimensions beside
      adh's floor, so the worker runs `tool run skillsaw-eval > s.json` (Loop B) then
      feeds the score to `harness gate` — skillsaw as the cheap floor under adh's bar.
      The decoder is now **aligned to skillsaw's real schema** (verified against the
      upstream `internal/rubric` source, since skillsaw is not installed here): the
      first cut assumed `{score, dimensions[{name,score}]}` but the real `eval --json`
      emits `Evaluation{deterministic_score, full_score, has_full_score, dims[{num,
      name, final (int 1–10), needs_judge}]}` — fixed, with `Eval.Score()` preferring
      `full_score` once the judge dimensions are scored. **Follow-up (deferred):**
      re-verify against an *installed* skillsaw when available; `diagnose`/`history`
      stay `tool run`; the mock `Propose` remains the non-relay default.

### From `agentic-harness-bootstrap` (Data Formats / Processes Worth Folding In)

The bootstrap system (MIT, prose+templates: turn a repo into something agents can
understand/verify) contributes three formats/processes that fill gaps the toolchain
above does not. Its other pieces (stack-specific lint/hook/CI templates, the
one-time discover→generate playbooks, the generated CLAUDE/AGENTS files) overlap
what adh or the driving skill already own — not adopted.

- [x] **ADR decision format for NFR trade-offs (§10.2, §11, §12).** DONE. The bootstrap
  ADR — *Status / Context / Decision / Consequences split into **Easier** and
  **Harder*** — is the durable, routable home for a team's local NFR
  prioritize/trade-off decisions. The `decision` context-unit kind + `lesson --to
  decision promote` shipped with Loop D; now the ADR structure is a pure owner
  (`internal/adr.Render`/`Valid`, which `lesson` delegates to), and `arc close --as
  decision --proof <adr>` closes on a **well-formed ADR** — `close`'s proof path
  branches so a decision's proof is the ADR itself (structural: the sections present
  and no unfilled `<placeholder>`), not a hash manifest. A skeleton fails
  NO-PROOF-NO-CLOSE (exit 8), so an undocumented decision cannot ship. Verified:
  `adr.Valid` table + a journey (a filled ADR closes; a skeleton exits 8).
- [x] **Harness-integrity self-verification (§10.4).** DONE. `adh doctor` runs a
      deterministic harness-wide self-check (`internal/harnesscheck.Check`, a pure core
      over the loaded store/registries/specs): each registry is structurally valid,
      unit ids are unique, NFR specs are well-formed, and the cross-references resolve
      (every unit's `integrity` names a declared §13 tool). Any problem exits 16 (reason
      `harness_integrity`); `--jsonl` carries the problems. Broader than Loop F's
      content-drift check — "is the whole harness intact and consistent?" — and cheap,
      so the `harness-integrity` §15 loop's sensor is now `adh doctor` (was `tool
      doctor`). Verified: pure `Check` table + a journey (init → doctor clean; a dangling
      integrity ref → exit 16). **Follow-up:** a session-start hook to run it
      automatically; checking agent-facing guidance references real commands.
- [x] **Standing-order accretion triggers as §15 loops.** DONE for the standing
      registry. `loop.StarterRegistry()` declares three standing accretion loops and
      `adh init` seeds `.adh/loops.json` from it (idempotent, single-owned
      `loop.DefaultRegistryFile`): `context-drift` (sensor `adh context verify` —
      Loop F), `harness-integrity` (sensor `adh doctor` — §10.4), and
      `lesson-backlog` (sensor `test ! -s .adh/lesson-candidates.json` — Loop D), each
      with `on_finding: open arc`, so a sensed departure becomes an arc an agent
      drives — accretion as a standing behavior, not a manual step. Verified: the
      starter registry validates, round-trips, and `adh loop list` shows them after
      `init`. The ADR trigger (architectural/NFR decision → `arc close --as decision`)
      now has its proof path. The session-start sweep now ships: `adh loop tick`
      fires *every* standing loop in one call (senses each, opens an arc per
      departure, exit 0 — a finding is queued work), and the `adh-relay` SKILL.md
      instructs the driving agent to run it at session start — for a skill-driven
      harness the agent is the hook. **Follow-up:** a real `.claude/settings.json`
      `SessionStart` hook is the environment-specific equivalent (deferred).

Optional / deferred (useful but larger or partly owned elsewhere): an `Always/Ask/
Never` boundaries format routed as legible authority context (complements the §5.2
gates that *enforce* with a summary that *teaches*); and an `adh init --bootstrap`
that seeds initial context units + §13 tool entries from a repo-profile scan
(discover→analyze), plus routing a hand-authored ARCHITECTURE map as a context unit.

### From `nfrs-guide` (NFR Taxonomy + Planguage Spec Format)

`nfrs-guide` (MIT) is a **knowledge base**, not a tool — so it is a curated *source
to distill via exegesis* into routable NFR units, **not** a tool adh integrates. But
its terminology/taxonomy and one data format are the answer to the *articulate-NFRs*
goal (the schema half; the ADR is the decision half):

- [x] **Adopt an NFR taxonomy + Planguage as the NFR-check spec format (§10.5).**
      DONE. `internal/nfr` (a pure core) defines the Planguage `Spec` — `Tag` (its head
      a FURPS+/ISO-25010 category), `Gist`, `Ambition`, `Scale`, `Meter`, `Direction`
      (higher/lower-is-better), `Baseline`, `Fail`, `Goal`, `Stretch` — with `Valid()`
      (taxonomy + a meter and scale + `Fail`/`Goal`/`Stretch` ordered by direction) and
      `Meets(value)` (the `Fail` acceptance bar). `adh nfr <list|show|lint>` inspects and
      validates `.adh/nfr/*.json` (invalid → exit 17). This binds the four things adh
      scattered into one unit: *category* (Tag), *check* (`Meter` → a §13 tool),
      *gate* (`Fail` → the Evaluation/proof acceptance bar), *rationale* (`Ambition` →
      a `decision`/ADR). Turns "should be fast" into a testable, gateable requirement.
      Verified: `Valid`/`Meets`/ordering table + a journey (a spec whose Meter is the
      `skillsaw-eval` §13 tool lints clean; a mis-tagged spec exits 17). The
      `Meter`/`Fail` wiring into Evaluation **now shipped**: an `nfr` finding whose
      `Ref` names a spec runs the spec's `Meter` §13 tool, parses the measured value,
      and confirms the finding when it breaches `Fail` (`evaluation.adjudicateSpec` +
      a `CheckRunner.Measure` seam) — the declarative threshold gates, not a tool exit
      code; an undeclared/unmeasurable Meter is unrunnable (unconfirmed). **Follow-up:**
      NFR allocation across components (`nfr-architecture-allocation`).
- [x] Terminology alignment (§10.5): DONE. `adh docs` now carries a **VOCABULARY**
      man-page section naming the two consistency questions in the standard NFR sense
      — **validation** (are the requirements right and conflict-free: `context check`/
      `lint`, Loop E) vs **verification** (is it built right: the `Meter`-driven
      Evaluation gate + proof, §19.2/§12) — "validation guards the inputs; verification
      gates the ship." Verified: docscmd renders the section. Optional (deferred): NFR
      **allocation** (split a top-level `Fail` into per-component budgets by routing
      derived units to component labels/paths, `nfr-architecture-allocation`).

### From `cc-thinking-skills/evals` (Replication-Gated Outcome Eval — the "Not an RNG" Backbone)

Its `evals/` is a replication-gated, condition-blind eval pipeline (JS/MIT) with a
pure `verdict()` core, honest negative results (it publishes that *zero* skills yet
ELEVATE), a validated judge, split-leakage checks, and real statistics (McNemar,
effect-size threshold, cluster bootstrap, Holm). It is a **reference methodology**,
not something to vendor (JS; adh implements the `verdict()`/stats in Go or shells
out). It directly answers the *accretive-not-RNG* goal and shows adh's/skillsaw's
single strict-`>` gate is insufficient.

- [x] **Replication-gated outcome-eval verdict for the adoption gate (§18.2, §16,
      Loop C).** DONE as the pure verdict core, layered over the strict-`>` gate.
      `internal/verdict` (pure, unit-testable — adh's FCIS) defines the taxonomy
      `ELEVATE / DIRECTIONAL-NOT-REPLICATED / REPLICATION-MISSING / KILL` via `Decide`
      (an effect-size threshold `DefaultMinEffect` + a paired significance test), plus
      `McNemar` (continuity-corrected, χ²(1,.05)=3.841). `consolidate.Plan` computes it
      from adh's existing two-split structure — **primary = the selection gain the gate
      ratchets on, replication = the held-out test split's paired outcomes** (McNemar) —
      so a staged candidate is ELEVATE only when its selection gain also replicates
      significantly. `sleep` surfaces the verdict in the staged line + manifest;
      staging is unchanged (the verdict labels trust, skillsaw's `gate` is the floor
      **under** this bar). Verified: `Decide`/`McNemar` tables + the cycle reports a
      verdict. The **fresh-replication** verdict now ships: `verdict.Replicate(runs,
      minEffect)` (pure) ELEVATEs only when ≥2 *independent* runs each clear the
      effect-size + significance bar (a fresh replication, not just a held-out split),
      KILLs on any regression, and is REPLICATION-MISSING with <2 runs. It is now
      **fed by a real multi-run rollout**: `consolidate.SplitForSeed` re-partitions the
      mined tasks N independent ways and `replicationVerdict` scores the candidate over
      `DefaultReplicationRuns` (3) independent seeded partitions (each an Outcome with
      McNemar significance), so `Cycle.Replication` is a genuine fresh-replication
      verdict, not the single held-out split — no live model worker needed, because
      adh's deterministic task-check *is* the rollout. `sleep` surfaces it. **Follow-up
      (deferred):** *model-generated* rollouts (a live worker running K model attempts)
      remain a separate capability; the deterministic evaluator is the rollout today.
- [x] **Validate the graders and the splits (§18.2).** DONE. Splits:
      `verdict.ValidateSplits` is a pure leakage guard (a task id in two splits →
      EINVALID). Grader: `harness.GraderSelfTest` extends the negative control from the
      *gate* to the *grader* — it proves the rubric scores a known-strong artifact
      strictly above a known-weak one (a blind grader makes the ratchet measure noise →
      EINTERNAL), and `sleep run` runs it beside `SelfTest` (exit 15) before trusting
      the loop. Judge calibration against labeled fixtures now ships too:
      `harness.CalibrateJudge` (pure) runs the deterministic judge over labeled
      `JudgeCase` fixtures and reports agreement, and `harness calibrate --cases <file>`
      exits non-zero on any disagreement — so the operator's check-sets are validated to
      discriminate. **Follow-up:** calibrate a *model* judge's free-text verdicts once a
      live judge is wired (this calibrates the deterministic rule-judge adh owns).
- [x] **Routing eval for the context lever (§10, Loop A/E).** DONE. `contextstore.
      EvaluateRouting` (a pure core) scores routing fixtures — each `RoutingCase`
      asserts the units that should route for an arc's labels/paths (an empty `Want`
      asserts NONE) — into per-case exact-match plus aggregate precision/recall. `adh
      context eval` runs `.adh/routing-cases.json` against the store; a failing case
      exits 12, so a routing regression on the #1 lever is measured, not assumed.
      Verified: `EvaluateRouting` table (hit/miss/NONE) + CLI pass→0 / fail→12.

### From the Knowledge Layer (OKF Format + Compounding-Wiki Process)

The context store *is* the team's curated knowledge base. Two of these give it a
real, open format and a compounding-maintenance process; the third is a front-end,
not an adh component.

- [x] **Adopt OKF as the context-unit format (§10.4).** DONE for the OKF *dimensions*.
      `contextstore.Unit` gained the three OKF dimensions (additive, backward-compatible
      JSON): **provenance** (`sources`), a **trust tier** (`verified`:
      unverified / machine-confirmed / human-reviewed, a `TrustTier` type with `Valid`
      + `Rank`) that records adh's "agent proposes → human confirms at a gate" on the
      unit, and **lifecycle** (`superseded_by`). Routing now **weights by how trust was
      earned**: a superseded unit never routes, and score ties break by trust rank
      (human-reviewed ▷ machine-confirmed ▷ unverified); `context show` surfaces trust +
      sources + supersession. Verified: `TrustTier`/routing/show tests + a journey.
      **Follow-up (deferred, no new capability):** the single-file markdown +
      YAML-frontmatter *packaging* (the JSON+content split already delivers content
      routing), per-claim footnote citations, and **freshness-by-time/staleness** (needs
      the deferred injected Clock — lifecycle is modeled by deterministic supersession
      now).
- [x] **Compounding-wiki operations (`llm-wiki.md` pattern) for §10/§11/§18.** DONE for
      the concrete new bits. The **read-first routing catalog** ships: `contextstore.
      Index` (pure) + `adh context index` render one line per routable (non-superseded)
      unit — id, kind, trust tier, labels, provenance — the JIT grounding preview a
      worker reads first. The **wiki-lint** ships: pure `Orphans` (no labels/paths → can
      never route), `DanglingSupersessions`, and `InvalidTrust`, wired into `context
      lint` (exit 12) and — for the cross-reference/enum defects — into `harnesscheck`/
      `adh doctor` (exit 16). Verified: helper + `Index` tables + `context index`/`lint`
      journeys. **Follow-up:** **file-answers-back** — an `investigation`/synthesis arc's
      output becoming a routable unit — is served today by `lesson --to context promote`;
      an `arc close --as investigation` → unit writer is a separate proof-path loop. The
      append-only evidence trail already exists (`sleep` evidence, the miss log).
- [ ] **`Unit.Verified` holds a conclusion where OKF holds the evidence for it, and the
      field name makes that invisible.** Found 2026-08-22 while `skillet` re-reviewed its
      OKF trust-fields decision. `contextstore.Unit.Verified` is a `TrustTier` *string* —
      `unverified` / `machine-confirmed` / `human-reviewed` — with `Valid()` checking enum
      membership and `Rank()` ordering it for routing tie-breaks, and its own comment calls
      it "the OKF `verified` dimension". OKF §5.2's `verified` is a **list of verification
      events**, each `{by, at}`, and §5.3 derives the tier by folding over the actor prefix.
      Same field name, same three tier names, same JSON key, different type: adh stores the
      answer where the spec stores what the answer was computed from. Three things go with
      it, and the first two are why §5.2 separates the fields at all. **Who confirmed it and
      when** — the tier says a human reviewed the unit and cannot say which human or on what
      date, an omission that is local to this field rather than a considered position, since
      the autonomy ladder treats actor identity as load-bearing everywhere else. **Two
      independent checks collapse into one** — §5.2's list exists precisely so a human
      sign-off *and* an automated nightly pass are distinct events, and `human-reviewed`
      cannot represent "reviewed by a person **and** re-confirmed by a process", which is
      the state a compounding knowledge base reaches most often. And **`verified`
      independent of `generated.at`** — OKF keeps them separate so *content changed without
      re-confirmation* and *re-confirmed without regeneration* are separately
      representable, where a stored tier makes the first silent: an edited unit keeps
      whatever tier it had. The entry above is honest about this and the honesty is easy to
      miss — it says "DONE for the OKF *dimensions*", not for the format, and its deferred
      list names packaging, per-claim citations, and freshness. It does not name the type
      substitution, which is the one that will bite, because a `verified` field that is a
      string where the spec says list is the kind of divergence that reads as conformance.
      **Not necessarily a bug:** if adh never exchanges units with an OKF consumer, storing
      the tier is cheaper and `Rank()` is a real need OKF does not serve. The decision to
      make explicitly is **whether `Unit` claims OKF conformance or borrows its
      vocabulary**, and to say which in the type comment — and if it is borrowing, to
      rename the field so it stops colliding.
- [ ] **adh is one change away from being `skillet`'s second consumer, and should know
      it.** `skillet` revised its OKF trust-field trigger on 2026-08-22 from *the first repo
      that stores trust metadata* to **the second repo that classifies an actor or derives a
      trust tier**, because gnosis shipped an actor type rejecting two of OKF §7's three
      forms without touching trust metadata at all — so a storage trigger could not have
      fired. adh stores a tier and derives nothing, so it has **not** tripped the new
      trigger. What it has done is implement OKF's tier vocabulary independently,
      identically named and differently typed, which is the second independent
      implementation in the family inside a week. If the deferred `{by, at}` half is ever
      built here — that is, if adh folds a tier out of verification events rather than
      storing it — that is the trigger, and the promotable unit is the **fold** and not the
      record types, because gnosis's `okf` keeps frontmatter verbatim and a struct would be
      decode-only. Keep any such fold a pure function over a list of actor strings so it
      lifts unchanged.

Not adopted: **leafwiki** (Go/MIT, single-binary wiki server, SQLite + markdown on
disk) is a *human front-end* to browse/edit the OKF context store (the Obsidian role
in the llm-wiki pattern) — complementary and optional, but adh is a CLI that routes
files, not a hosted wiki, so it is not an adh component.

### From `virgil` (Patterns Only — a Peer System, No License, Not Vendored)

`virgil` (Go, **no LICENSE**) is a peer harness (a personal assistant) that
independently arrives at adh's design — a five-stage `signal→classify→plan→execute→
output` loop, deterministic-first with AI as a gated fallback, stateless
invocations with context assembled fresh from memory (not a chat buffer),
self-improvement into *readable config, not weights*, and append-only JSONL
evidence. That convergence corroborates adh; it is not a tool adh orchestrates. One
pattern is a genuinely novel add for the context lever:

- [x] **Routing learns from its misses (§10.3).** DONE. Each critic routing gap
      (§19.1, exit 12) is no longer discarded: `run`/`step` append the arc's
      labels/paths to an append-only **miss log** (`.adh/context-misses.jsonl`, kept
      distinct from evidence — a learning signal, not an audit record;
      `contextstore.AppendMiss`, best-effort so a failed append never masks the gap).
      `adh context misses` aggregates them and, past `defaultMissThreshold` (2),
      **proposes a deterministic route** for any label/path the arcs keep missing on
      (`contextstore.ProposeRoutes`, a pure core ranked by recurrence). It only
      proposes — authoring the unit stays **gated at §11** (nothing is auto-routed).
      Verified: pure `ProposeRoutes` table, append/load round-trip, and an E2E test (a
      relayed gap writes a miss; `context misses` proposes the label). **Accretion
      applied to the #1 lever — the router improves the more it is used.**
- [x] **Per-tool / per-unit KPIs → gated improvement proposals (§16, §18).** DONE for
      per-unit (the deterministic slice that reuses data already logged); per-tool is the
      documented follow-up. A `§10` unit declares KPIs (`adh.KPI`: metric + threshold +
      degradation `Direction`, `Breached`/`Valid`); `contextstore.Unit.KPIs`, with a
      malformed one caught by `doctor`/`harnesscheck` (exit 16, `invalid_kpi`) so it never
      silently fails to fire. `adh kpi` measures each unit's `grounded_miss` KPI against
      the failure-record log — how often an arc failed *despite* the unit's scope being
      routed — and proposes a change to any unit whose breach **replicates across ≥2
      strata** (§18.2, the same never-on-one-signal bar as the miss/lesson gates). Advisory
      by design (exit 0, like `context misses`): a human makes the change, never the
      harness. The `internal/kpi` core is pure and **source-agnostic**
      (`Observation`/`Subject`/`Propose`), so per-tool KPIs drop in once a tool-run outcome
      log exists. Verified: `KPI.Breached`/`Valid`, `Propose` (breach × strata gate),
      `ObserveUnits` (label/path scope overlap, ungrounded ignored) tables + `kpi`/`doctor`
      journeys. **Per-tool KPIs DONE:** tools declare KPIs (`toolreg.Tool.KPIs`, malformed
      ones rejected by `Registry.Validate`/`tool doctor`); `adh tool run` appends a
      stratum-stamped outcome to `.adh/tool-runs.json` (`internal/toolrun`); and
      `kpi.ObserveTools` measures each tool's `run_failure` KPI so `adh kpi` proposes a
      change to a tool that keeps failing across ≥2 strata (Subject/Proposal now carry a
      `Kind` so the output names "tool" vs "unit"). Verified: `ObserveTools` table,
      `toolrun` round-trip, `Registry.Validate` KPI case, a `kpi` tool journey + smoke.
      **Residual follow-up DONE:** every §13 tool run is now logged through one owner
      (`toolrun.AppendOutcome`) carrying its duration — `adh tool run`, `context verify`
      (a drift is a failed run), and declared-tool adjudication during `adh eval` (the real
      `RepoAdjudicator` gets a `logPath`; the test constructor stays silent). `ObserveTools`
      adds a `run_duration_ms` KPI (mean latency over started runs), so `adh kpi` proposes a
      change to a tool that keeps failing *or* running slow across ≥2 strata. Verified:
      `AppendOutcome`, `toolDuration`, a `context verify` log journey, an adjudicator log
      test, and a slow-tool `kpi` journey. **Stays out by design:** NFR Meter runs (their
      exit code is not the pass/fail signal) and percentile latency (mean is the cheap,
      deterministic aggregation).
- [x] Effectiveness north-star (§16): DONE as a coarse proxy. `metrics.ClassifyHistory`
      /`StepClass.Ratio` (pure) classify each arc's history into **deterministic-handled**
      steps (evaluation, gate, commit, close) vs **LLM/critic-handled** turns (a
      relayed `strategy:`/`execution:`/`critic:` reply, grounded in the actual history
      format); `adh selfeval` surfaces the deterministic share, the direction accretion
      (routing rules, checks, lessons) should drive *up* over time — a measurable §16
      direction, not a vibe. Verified: `ClassifyHistory`/`Ratio` table. **Follow-up:**
      per-step instrumentation for an exact (not history-proxy) ratio. Optional/larger:
      a **pipe/pipeline** composition model for §13 tools (atomic tools + recursive
      pipelines over the `--jsonl` envelope), so distill→optimize→gate is a declared
      pipeline.

### From a Peer-System Survey (`emulo`, `PolyBrain`, the Hermes family, `darwinian_evolver`)

Seven `~/Documents/git` systems examined for additive techniques (2026-08-01). Most
overlap adh's existing machinery — and adh is **ahead** of both evolution repos
(`darwinian_evolver`, `hermes-agent-self-evolution`) on gating rigor: neither has
McNemar, effect-size thresholds, multi-run seeded replication, or judge calibration,
which adh does. **Overlap to skip** (confirmed across all seven): the five-stage gated
loop, tool registry + `run`, the relay, the full sleep cycle (rubric floor + strict-`>`
ratchet + judge + held-out splits + replication verdict), lesson promotion, standing
loops + `tick`, `doctor`, routing eval, effectiveness north-star, staged-never-auto-
adopt, provenance/trust/lifecycle units, content-hash no-op guards, secret redaction,
PR-not-direct-commit. The genuinely additive, deterministic, adh-aligned ideas:

- [x] **Temporal-stratum gating for promotions (Tier 1, §11/§18.2).** DONE for the
      structured miss log (the `[]string` failures registry is a documented follow-up).
      `contextstore.Miss` gained a `Stratum` (year-month); `run`/`step` `recordMiss`
      stamp `contextstore.Stratum(time.Now())` (the Clock stays in the shell — its first
      real consumer). `ProposeRoutes` now requires **≥2 distinct strata** (`minStrata`)
      *and* the count threshold, so a same-day burst is recorded but not proposed — only
      a pattern sustained across time earns a route. An independence axis orthogonal to
      the seeded-partition replication verdict. Verified: `ProposeRoutes` strata table +
      an E2E test (two same-stratum gaps record but do not propose). **Follow-up DONE:**
      the same gate now covers lesson promotion via the stamped failure-record log —
      see "Multi-signal accretion + root-cause triage" below.
- [x] **Critic coverage / blind-spot history (Tier 1, §19/§10.3).** DONE. A pure
      `internal/critic` coverage log (`AppendCoverage`/`LoadCoverage`/`UnderCovered` +
      `adh.FindingKinds`, `.adh/critic-coverage.jsonl`): `run`/`step` record an arc's
      surfaced finding kinds after a critic turn, and the `Inputs`→`Grounding` seam
      carries the under-covered kinds into `critic.tmpl` ("recent reviews under-covered
      these check kinds — probe them"), steering the next critic to the gaps. The
      routing-learns-from-misses pattern applied to the critic/eval lever. Verified:
      `UnderCovered`/`FindingKinds`/round-trip tables + the template render.
- [x] **Provenance / receipt verification gate (Tier 1, §10.4).** DONE — both halves.
      Path existence: `contextstore.LooksLikePath` + `DanglingSources` (pure, filesystem
      injected) flag a unit `source` that looks like a repo path but does not resolve
      (URLs and prose skipped). Quote-tracing (the receipt half): `Unit.Claims`
      (quote + source) + `UnverifiedClaims` (pure, file read injected) flag a claim whose
      quote is not found in its cited source. Both wired into `context lint` (exit 12) and
      `doctor`/`harnesscheck` (exit 16: `dangling_source`, `unverified_claim`). Serves the
      "provenance weighted by how it was earned" goal. Verified: `LooksLikePath`/
      `DanglingSources`/`UnverifiedClaims` tables + `context lint`/`doctor` journeys.
- [x] **Reflective trace into the optimizer (Tier 2, §18).** DONE. `consolidate.
      ProposePrompt` now mines the tasks and scores the selection-split assertions
      against the current artifact (both pure), rendering a **"currently-failing
      held-out assertions — target these"** section — the failure trace, not just the
      ranked reflection modes, so the relayed agent proposes a targeted edit. Self-
      contained: no signature or shell change. Verified: a prompt-trace test (an artifact
      missing the class surfaces it; a satisfied artifact shows none).
- [x] **Scope-tagged lessons (Tier 2, §10/§11).** DONE. Every disposed finding is
      stamped into `.adh/failure-records.json` (`failures.Record`) with the arc's routing
      scope; on promotion, `failures.ScopeFor` tags the materialized unit with the
      distinct labels/paths the class recurred under, so the lesson routes to where it was
      learned rather than only by its generic class label — a context-specific correction
      cannot over-govern every arc. Composes with the OKF labels + trust tier. Verified:
      `ScopeFor` table + a promote journey asserting the unit routes by its scope label.
- [x] **Multi-signal accretion triggers + root-cause triage (Tier 2, §11/§10.3).** DONE
      for the deterministic core. Accretion trigger: lesson promotion requires a class to
      recur across **≥2 distinct time strata** (`failures.StrataCount` + `lesson.MinStrata`,
      exit 19) — corroboration across independent temporal signals, not a single-stratum
      burst. Root-cause triage: each record is classed **ungrounded** (failed with no
      routed context — fix routing) vs **grounded-miss** (failed despite it — fix content),
      derived deterministically from `arc.Context`; `failures.RootCauseCounts` surfaces the
      breakdown at promotion so a human sees whether the class is a routing or a content
      problem. Verified: `StrataCount`/`RootCauseCounts`/`ClassifyRootCause` tables +
      `Apply`-stamps-record + a strata-gate promote journey. **Follow-up:** richer causes
      (infra/auth/rate-limit) need a live model worker's diagnostics, still deferred.
- [x] **Fixable-vs-structural finding taxonomy (Tier 2, §12/§19.2).** DONE. `Finding`
      gained an optional `Class` (fixable | structural; empty defaults to fixable,
      validated by `ParseFindings`); a confirmed **structural** finding now fails the arc
      terminally at `evaluation.Decide` — escalating to a human — instead of spending
      rework cycles on an edit that cannot close a design change, while fixable findings
      keep the return-to-Execution path. The critic prompt asks for the class. Verified:
      `Decide` structural→Fail case + `HasStructural`/`ParseFindings` class tables.
- [ ] Optional / against-grain / larger (Tier 3): adaptive percentile + novelty
      selection (`darwinian_evolver`'s sigmoid sharpness/midpoint + `1/(1+novelty·
      children)`) — an *alternative* to adh's deliberate strict-`>` gate, not a
      replacement; richer generated-artifact templates (checklist/anti-patterns/
      integration) from `hermes-skill-factory`; schema-enforced plan-audit before
      execution from `PolyBrain` (`schema.json`); and external session-log mining
      (Claude/Codex/Copilot `*.jsonl` → context units / eval examples, secret-redacted —
      adh's `redact` already covers the safety half) from `emulo` + `hermes-agent-self-
      evolution`.

## Ported from Skillsaw (Done)

- [x] `internal/judge` — deterministic rule-judge (6 operators; hard/soft), plus
      `cmd/judge`. Retargeted, adh.Error-ized, tested.
- [x] `internal/edit` — `WithinSizeBudget` + `IsNoOp` (via `identity.Hash`).
- [x] `internal/evidence` — append-only JSONL audit log (validate-on-write,
      corruption-is-a-hard-error), wired into `sleep run`.
- [x] `internal/rubric` — the DET.SCORE floor + NeedsJudge + weighted total +
      Diagnose pattern, with adh's own dimensions (no SKILL.md/markdown/
      neutrality parts).

Remaining wiring for these:

- [x] A `harness eval` command that scores an artifact with `rubric` and its
      behavioral checks with `judge` (the floor + judge-boundary surface).
      `cmd/harnesscmd` via `internal/harness.Eval`; `--checks`/`--output`/
      `--json`/`--min` (below-floor exits 1; `harness --min N eval <artifact>`).
- [x] Use `edit.WithinSizeBudget`/`IsNoOp` and `evidence` inside the real
      `sleep` consolidate loop — now wired through `internal/consolidate.Plan`.

## Rubric Applicability — Deterministic Dimensions a Heading Satisfies (2026-08-09)

`deterministicScore` scores `KeyFailure` (weight 20) as
`len(skilllens.FailureMechanisms(md)) > 0` — "either satisfies the dimension". But
`FailureMechanisms` returns section spans as well as prose spans, and the section match is
on the heading *title*: `"boundary"` is in `skilllens.FailureSectionTitles()`. So **a bare
`## Boundary` heading earns the full 20 points** with nothing written under it.
`KeyBoundary` (weight 15) has the same shape. That is **35 of adh's 100 points**, and
unlike skillsaw neither dimension is `NeedsJudge`, so there is no human backstop to
correct it.

Measured in the skillsaw corpus (233 skills, re-measured 2026-08-14): 154 skills — 66% —
have **zero** inline failure branches and pass this exact check on a heading alone.
(An earlier pass said 144/62%; it parsed frontmatter as body. Figures below are the
corrected ones.)

- [x] **Split `KeyFailure`'s evidence by `Span.Kind` instead of taking `len()`.** DONE
      (2026-08-15). Only `KindProse` — an actual "if X fails, do Y" branch — satisfies the
      dimension now. `KindSection` is a heading whose *title* matched, and a heading with
      nothing under it encodes nothing; counting the two alike is what let a bare
      `## Boundary` earn all 20 points.
- [x] **Gate the resulting deduction on `markdown.Doc.HasCodeBlock`.** DONE (2026-08-15, on
      skillet v0.15.0). A prose branch always satisfies the dimension; its absence is a
      defect only when the artifact runs something. An artifact that executes nothing has no
      runtime failure to encode, so docking it would be a category error.
      **The gate is what let this stay binary.** The obvious split — prose 1.0, section-only
      0.5, neither 0.0 — invents a 0.5 nobody calibrated, inside 20% of the total. Making
      the *deduction* conditional does the work grading would have done, with no
      uncalibrated constant.
      `DimScore.Reason` is always populated, so a suppressed deduction stays visible — the
      "emit the flag even when the penalty is suppressed" requirement needed no new plumbing.
      Read the field; do not write a local fence scan — a start-of-line fence regex misses a
      fence indented inside a list item, which is exactly what made the first measurement
      here wrong.
- [x] **Re-score before and after.** DONE (2026-08-15). **38 of 233 artifacts moved, every
      one by exactly −20** (the `KeyFailure` weight), and **nothing moved that was not
      predicted**: the moved set is exactly "executes something, and has no prose branch".
      Scored through `harness eval --jsonl` with binaries built from before and after the
      change and confirmed to differ. Two artifacts predicted to move did not, which
      surfaced the separate defect below.
- [x] **`Evaluate` scores YAML frontmatter as if it were body prose.** DONE (2026-08-15).
      `Evaluate` now splits with `skillet/frontmatter.Split` and parses only the body; the
      header block is discarded, since none of the five dimensions is about metadata.
      **The split lives inside `Evaluate`, not at the three call sites.** Putting it at the
      callers would make each one know that a header must be stripped first, and a fourth
      added later would silently get it wrong — it defines the failure mode out of existence
      instead of documenting it. Signature is unchanged; `Split` returns the whole document
      as body when there is no leading `---`, so it is inert for header-less artifacts.
      **It was never confined to `failure-handling`.** The re-score moved 5 of 233 artifacts,
      **all downward**: `letsgo-form-validator`, `test-helper-process-subprocess-mock` and
      `lintme` by −20 (the branch regex was matching their *description*), and `crit-story`
      and `unconventional-commits` by −15, where a boundary word in the header was earning
      `boundary-section`. `SofteningPhrases` reads the same `Doc`, so specificity was exposed
      the same way.
      `GraderSelfTest` is untouched and still passes: its fixtures carry no header, so the
      split is inert there.
      One test in the first draft was **vacuous** and worth recording: the CRLF fixture used
      "it fails when the input is bad", which skilllens correctly rejects as prose rather
      than a branch, so it scored 0 whether or not the header was split off. Caught by
      reverting the split and checking *which* tests failed — only one of the two did. Any
      test of this kind must use a phrase the detector actually matches.
- Note: **`KeyBoundary` is not the same shape, and was deliberately left alone.** An earlier
  draft of this section said it was. `skilllens.BlacklistSections` returns **only** headings
  — its doc says so — so there is no `KindProse` alternative and no Kind split to make; its
  analogous laundering is an empty heading, which is a `Span.Units` threshold, a different
  instrument with no calibrated value. It must not be gated either: the gate exists for
  *runtime* failure mechanisms, whereas a boundary is a conceptual claim that a document
  executing nothing should still make. Gating it would excuse exactly the documents that
  most need it.

## Disposition

- [x] Branch `implement/adh-spec` merged to `main` via PR #1 (`6180963`); the whole
  implementation is now on `main`. Remaining tidy-up: delete the merged branch
  (`git branch -d implement/adh-spec`, and the remote copy if still present).

## Fidelity to the Source Implementations

Gaps found comparing adh to `SkillOpt` and `evals-differential-oracle`. The pure
decision cores (gate ratchet, score projection, content hash, defect/lapse,
independent invariant checker, gate self-test) are faithful; the orchestration
around them is partial.

- [x] `sleep` runs the real `harvest → mine → reflect+gate → stage → adopt`
  cycle (`internal/consolidate` + `cmd/sleep`): stable held-out splits, the
  edit guards, a persisted rejected-edit buffer, staged proposals under
  `.adh/sleep/staging/<id>/`, and `sleep adopt` applying with a backup.
  Remaining: the optimizer is the deterministic mock `Propose` (no model
  backend); staged text is not yet secret-redacted (§18.4); no `sleep schedule`.
- [x] Ported from SkillOpt into `internal/consolidate`: `compute_score`
  (`scoreSplit` aggregates mean hard **and** soft), `select_gate_score`
  projection wired into the ratchet (`Config.Metric`/`MixedWeight`),
  success+failure mode extraction (`Reflect`, failure wins a conflict),
  `rank_and_select` (modes ranked by recurrence, budget clips), longitudinal
  improved/regressed/persistent categories (`Categorize`), the slow-update /
  meta guidance (`SlowGuidance`, staged as `longitudinal.md`), and the
  LR-scheduler (`Config.Round` + `effectiveBudget`, linear decay).
  Remaining: multi-rollout contrastive reflection (`rollouts_k>1`) stays
  mock-single — adh has no live worker to run K attempts; and the slow guidance
  is staged as cross-cycle memory, not written into a live protected region (the
  gated candidate stays the single LEARNED region so a score change is
  attributable). With single-check tasks per-task soft equals hard, so the dual
  aggregate is structural until multi-check mining diverges them.
- [x] Oracle is two-dimensional (rows + columns): `internal/oracle` resolves a
  `Board` with independent React/Native enumeration and an independent invariant
  checker; the planted `Buggy` resolver is caught by both nets.
- Deliberately omitted: `select_gate_score`'s optional semantic-density bonus (a
  gameable proxy). Not a defect.

## Command Surface (PLAN Phase 9 Follow-Ups)

- [x] Per-stage direct commands `strategy`/`execute`/`critic`/`ops` (`cmd/stagecmd`):
  strategy/execute/critic run one Mock stage on an arc already at it; `ops` reports
  the ship gate. `eval` shipped earlier (§19.2).
- [x] `gate list` (`cmd/gatecmd`) lists blocked arcs with their gate reason;
  `failures list` (`cmd/failurescmd`) and `selfeval` (`cmd/selfeval`) compose
  `failures.Load` + `lesson.Distill` + `metrics.Summarize` (a new `metrics.Load`
  single-owns the ledger, replacing the inline reader).
- [x] Real ff subcommand nesting for the ratchet regroup: `harness` now has nested
  `eval`/`gate`/`hash` subcommands (each its own flagset); the top-level `gate`
  ratchet moved to `harness gate` and `cmd/gate` was deleted, freeing `gate` for
  the human-gate listing.
- [x] Global flags on `root.Config`: `--config`, `--profile`, `--repo`,
  `--verbose`, `--quiet`, `--no-color` bound once on `root.Flags`. `--quiet` is
  wired (discards stdout in `cmd.Run`); `--config` is wired (`root.ConfigGetenv`
  threaded into every `config.Load` site).
- [x] Global `--jsonl` machine output: a single global flag on `root.Config`
  emitting **JSON Lines** (one compact JSON object per stdout line) — chosen over
  `--json` so the contract is uniform whether a command returns one result or many.
  Replaced the local `--json` flags (version/harness/step) that would have collided
  with a global one.
- [x] Structured outcome envelope: every `--jsonl` line is
  `{status, code, reason, message, data}` (`root.Outcome`, `EmitOK/EmitBlocked/
  EmitError`), so success, a gate stop, and a failure share one shape an agent
  switches on. Wired through every command and the mutation commands
  (`run`/`proof`/`approve`/`reject`/`close`), and the dispatcher envelopes any other
  returned error (reason = the domain code) instead of a usage banner. Stable reason
  tokens (`at_ops`, `ungrounded`, `gate`, `proof`) replace the relay's stderr
  string-matching. Follow-up: structured logging (`log/slog`) on stderr keeps the
  diagnostic stream separate from this data plane.
  - [x] Aligned to climax's generated envelope (climax v0.8.0 `climax init --jsonl`):
    the generic shell (`Status*`/`Outcome`/`Emit*`) moved to `cmd/root/outcome.go`
    matching the template verbatim (`EmitJSONL` now `json.NewEncoder(...).Encode`);
    adh's words stay — specialized `CodeForError` (over the climax stub),
    `ReasonForError`, and the `Reason*` tokens. `// climax:features jsonl` in the
    dispatcher makes `climax lint` drift-check the `outcome` surface (which reports
    clean; the separate base-scaffold check does NOT — see the survey follow-up
    below). This closes skillet's deferred "envelope → climax", which was blocked on
    climax shipping an `outcome` surface.
- [x] Global `--yes`/`--dry-run`: bound once on `root.Config` (the same unification
  pass as `--jsonl`), long-only to avoid subcommand short-flag collisions; `approve`
  dropped its local copies and reads the inherited fields, so its safety invariant is
  unchanged — `GateSatisfied(required, phrase, dryRun)` still returns false under
  `--dry-run`, and neither flag can satisfy a human gate. `--dry-run` now previews the
  other two gate/ship mutations without persisting: `reject` reports "would reject"
  without reverting or saving, `close` runs every gate (ready/proof/`CanClose`) then
  reports "would close as <res>" without shipping, saving, or recording a metric. Every
  other mutation command (`run`/`step`/`eval`/`arc`/`worker`/`proof create`/stage
  relays) refuses `--dry-run` via `root.DryRunUnsupportedError` so the global flag never
  silently mutates state. `--yes` is a documented reserved convention (adh has no
  interactive prompt today) with the hard guarantee that it never satisfies a gate.
  - [ ] Deferred — full `--dry-run` for the `run`/`step`/`sleep` relay loops: previewing
    a whole multi-stage drive (not just the single gate/ship mutations) is a larger
    change; those commands refuse the flag today rather than fake a partial preview.
- [ ] Deferred — `registry audit`: no artifact-registry model exists; auditing only
  proof packets would be a partial interpretation of "orphans/missing-manifests/SHA
  mismatches". Needs the registry concept first.
- [ ] Deferred — broad ff nesting for `arc`/`sleep`/`oracle`/`lesson`/`context`/
  `tool`: positional-verb dispatch works; converting is broad and low-payoff. Only
  the ratchet regroup (above) needed real nesting.

## Config Wiring

- [x] `internal/config` precedence loader (SPEC §3): defaults → user config →
  repo `.adh/config.toml` → `.adh/autonomy` runtime override → `ADH_AUTONOMY`.
  `resolve` is a pure TOML overlay (BurntSushi/toml); `Load` is the file/env
  shell; `getenv` is injected from the composition root so precedence is tested
  without touching the process env. A malformed config is a wrapped error.
  - [x] `run` and `autonomy` resolve the autonomy level from config
    (`AutonomyLevel`); the relay parks earlier below L2.
  - [x] The model-gate judgment set is config-driven (`[models.gate]
    judgment_roles` → `authority.ModelGate`, enforced in `stage.Execute`).
  - [x] The approval phrase is a single-owner policy
    (`authority.RequiredApprovalPhrase` = arc id), deliberately never sourced
    from config/env (`ADH_APPROVAL_PHRASE` ignored) so the gate stays structural
    (§5.2) — a config-settable phrase would be a self-grant route.
  - [x] `worker requalify` binds the per-role baseline from `[models]`
    (`config.BaselineModels`); editing `[models]` opens a new epoch (§14).
  - [x] `adh init` writes a starter `.adh/config.toml` (`config.StarterTOML`,
    idempotent — kept if present) and scaffolds `.adh/context` + `.adh/artifacts`.
  - [x] The `--profile` layer (SPEC §3 tier 3): `--profile <name>` (or
    `ADH_PROFILE`) selects a repo-local overlay `.adh/config.<name>.toml` that
    overlays the repo config and is overridden by env then flags. Wired through the
    `ConfigGetenv` bridge (flag → `ADH_PROFILE`, flag wins) so no call site changed;
    `configDocs` appends the profile layer; a missing profile file is a no-op.
    Follow-up (deferred): a profile overlay at the user (XDG) layer; secrets stay
    env-only.
  - [x] `[evaluation] max_reworks`: the Evaluation rework budget (§4.1) is
    config-driven (`config.MaxReworks()`, default owned by
    `evaluation.DefaultMaxReworks`), threaded into the `eval`/`run`/`step`
    disposition sites.

## Effectful Seams Still Mocked

- [x] `model.Relay`: a third `model.Client` whose completion is supplied out of
  band — `adh step --relay` emits the stage prompt (`internal/prompt` templates,
  cold-critic view) and parks a pending turn; `--response` resumes and advances.
  Drives the LLM from a skill (`.claude/skills/adh-relay`) instead of an API.
- [x] Validated relay replies + relay wired into `run`. Every relayed reply is
  validated against its stage before the arc advances (`critic.ParseReply`): empty
  is rejected for all, a critic reply is findings JSON, and a strategy reply may
  choose the resolution via a leading `resolution: <word>` line (§12). The emit/
  resume orchestration moved to `internal/relay` (a shared engine; the worktree
  grounding to `internal/worktree`), so `step` and `run` are thin shells over it.
  `run --relay [--response]` now drives the relay, chaining a resume through inline
  evaluation to the next emitted prompt in one call.
- [ ] `model.Client`: no real *API* client yet (Anthropic/OpenAI); `Mock` and
  `Relay` are the only backends, selected by the `--relay` flag. (A `[models]
  driver` config to pick the default backend was considered and dropped — the
  driving agent passes `--relay` anyway, so it earned no keep. The API client
  itself stays deprioritized — the relay is the backend.)
- [ ] `device.Validator`: only `Mock`; no adb adapter. Domain-specific (mobile
  port) — not core to a general repo (see the proof-contract note below).
- [x] `VCS`/git adapter: `internal/vcs` (go-git v6) + the `vcs` command do
  status/branch/commit/diff/revert; `internal/vcs.Mock` is the test double. `close`
  commits a `change` arc past the approval+proof gates onto its own branch
  `adh/<arc-id>` (branch-per-arc, base untouched; best-effort: no repo / clean tree
  / no baseline commit → the arc still closes in place), and `reject` reverts the
  arc's working-tree paths to HEAD and returns it to Execution. `vcs.Revert` is
  path-scoped and hermetic (go-git only, no `git` binary), so it never touches the
  `.adh` workspace or unrelated work. Follow-up: merge of the arc branch to base
  (go-git merge is experimental → a `git` shell-out via
  `github.com/ldez/go-git-cmd-wrapper/v2`); returning to the base branch after ship.
- [ ] No injected `Clock` (deterministic timestamps). **Deferred deliberately**:
  the precondition is still unmet — no `time.Time` on `adh.Arc`/state. The `time.Now()`
  sites all isolate time to a one-line shell so the pure cores never see it:
  commit-authorship (go-git, never pinned by a test), `sleep.stamp`, and the
  stratum stamping (`contextstore.Stratum(time.Now())` in `run`/`step`/`eval`) whose
  year-month string the miss log and the failure-record gate consume as an opaque
  token. A `Clock` seam today would be a speculative abstraction; the strata gates
  assert on records seeded with a fixed stratum, not on wall-clock. Revisit when a
  state time field or a test needs deterministic time.
- [x] The oracle's two "implementations" are in-package functions — **generalized to a
  command-level differential oracle.** `oracle diff --reference <cmd> --candidate <cmd>`
  runs two repository commands and confirms divergence in their output (`oracle.DiffOutputs`
  pure; the shell captures and formats), the general form of "two implementations grade
  each other" for any repo — a reference build vs an optimized port, an old vs new
  implementation. It exits the oracle gate code on divergence, so declaring it as a §13
  tool makes `adh eval` confirm an oracle finding against **real** divergence, no
  mobile-port profile and no adb. The in-package board oracle stays the built-in fallback
  (diff with no flags). An **invariant** finding is likewise providable — declare any
  property-check command as a §13 tool; the adjudicator already runs it. Verified:
  `DiffOutputs` table, command journeys, an end-to-end tool-wrapping smoke. `device`/adb
  stays deferred by choice (domain-specific hardware).

## Proof Contract Generalization

- [x] SPEC/SPEC-ADDITIONS decision: the `change` resolution's proof contract is
  generalized and made **configurable** per deployment (§SPEC 3.1
  `[proof.contract]`, §12). The generic default is code-level (tests/review/CI);
  the oracle + invariant + on-device triad is one deployment profile, not a
  built-in requirement. Screenshot sanitization (screen dims, redaction) is now
  documented as domain-specific, not part of proof in general.
- [x] Code follow-up: the contract is config-driven. Validity split from text:
  `adh.Resolution.Valid()` is the domain fact; `ProofKind()` is a generic default
  text (change → code-level, no longer oracle/device). `config.ProofContract(res)`
  returns the `[proof.contract]` override or the default, and the critic's
  acceptance bar is threaded from it (`critic.Ground` takes the bar as data, so it
  never reads config). Remaining: `prompt`'s non-critic `.ProofKind` view field
  still shows the built-in default (not the config override) — a minor follow-up.

## Proof Packet Generation

- [x] `adh proof create <arc-id> <path>...` hashes an arc's artifacts into a
  manifest (`internal/proof.Create` + `Save`, beside `Load`/`Verify`), writes it
  to `.adh/proof/<arc>.json`, records it on `Arc.Proof`, and verifies it — so an
  agent driving adh via a skill can satisfy NO-PROOF-NO-CLOSE without hand-computing
  `identity.Hash` digests. This is the wall that used to dead-end every arc at close.
- [x] `proof` is nested (real ff `create`/`verify` subcommands), so
  `proof create --out <path> <arc> <paths>` parses (`--out` restored); and `close`
  defaults `--proof` to `Arc.Proof` when omitted, closing the create → close loop.
- [x] Provenance: the manifest carries an optional `Provenance{git_sha}` (§SPEC
  5.4/§4). `proof create` records the repo's HEAD SHA (`vcs.HeadSHA`, best-effort:
  no repo / no commit → none); it is informational, not a gate — `Verify` still
  checks only existence + digest, so a proof created at one commit verifies at
  another. Follow-up: screenshot-domain provenance (dimensions, redaction method).

## Cold-Critic Grounding and Finding Disposition (§19)

- [x] §19.1 grounding contract: the critic prompt now carries the repository-owned
  working set — touched paths, acceptance bar, the proof packet's artifacts, and
  the context units routed by the arc's labels/paths (`internal/critic`,
  `Arc.Labels/Paths/Proof`). The builder's transcript stays denied (cold).
- [x] §19.1 routing gap: an arc that declared a footprint (labels/paths) but routed
  no context and left no proof exits 12 on `adh step --relay` at the critic —
  the environment did not teach it; the relay must not guess.
- [x] §19.2 structured findings: a critic turn's reply is findings JSON, parsed
  and validated (`critic.ParseFindings`, validate-and-reject) into `Arc.Findings`
  on `step --relay --response` at the critic; each finding names the artifact
  (oracle/invariant/device/nfr/contract) that would confirm it.
- [x] §19.2 disposition: `adh eval <id>` adjudicates each finding against its
  named artifact (`cmd/evalcmd`, pure `critic.Dispose`). Confirmed (the artifact
  ran and failed) returns the arc to Execution + a `failures` registry entry and
  exits 5–8 by kind; unconfirmed (passed, or unrunnable) becomes a §11 lesson
  candidate (`.adh/lesson-candidates.json`) and the arc advances to ops. The Ops
  human gate is unchanged; no new exit code. `step --relay` refuses evaluation and
  points at `adh eval`.
- [x] §19.3 convergence: recorded as `critic.ConvergenceConstraint`; the critic
  runs once per arc — a bounded critic/author revision loop stays a design
  constraint, not a runtime gate.
- [x] §19.4 config: `[critic]` block parsed with defaults; `unconfirmed` is wired
  into `adh eval` (`config.CriticUnconfirmed`). `ground_from`/`deny` are recorded
  but enforced structurally (the grounding assembly and the cold renderer), not yet
  read as behavior.
- [x] Unified evaluation: the disposition orchestration moved to
  `internal/evaluation` (`Adjudicator`, `RepoAdjudicator`, `Adjudicate`, `Apply`),
  and `run`, non-relay `step`, and `adh eval` all route the evaluation stage
  through it — evaluation is deterministic on every path, never a model step. The
  relay path already refused evaluation and pointed at `adh eval`.
- [x] §19.1 changed-path grounding: resuming a relayed *execution* turn records the
  working tree's changed code paths into `Arc.Paths` from `internal/vcs`
  (`.adh/` state filtered), so the cold critic is grounded on the real change, not
  hand-seeded paths. Best-effort: outside a git repo it is a no-op.
- [x] §19.2 NFR adjudication: an `nfr` finding's `Ref` resolves to a declared
  check in the tool registry (§13, `toolreg.FindByID`) and runs it via a
  `CheckRunner` seam (`evaluation.ShellRunner`, `sh -c` in the repo). A non-zero
  exit confirms the finding (returns the arc to Execution); a Ref the registry does
  not declare is unrunnable → unconfirmed, so the gate still drops an invented
  requirement (§19.2). The command is repo-owned config, never model input — the
  critic supplies only the tool ID. `RepoAdjudicatorFor` wires it for `eval`,
  `run`, and `step`; the contract path's `proof.Verify` is now rooted at the same
  repo dir instead of `.`.
- [x] §19.2 adjudication depth — **generalized (the domain-specific targets stay
  deferred).** An `oracle`/`invariant`/`device` finding now resolves its `ref` to a
  repository-declared §13 tool when one exists (the real domain target the repo provides,
  its exit code the signal — exactly how NFR findings already adjudicate), falling back to
  adh's built-in check when it names none; a declared-but-unstartable tool is
  unrunnable/unconfirmed, never a false confirmation. This makes the per-arc *confirmed*
  path reachable for **any** repo via `evaluation.runDeclaredTool` (shared with the NFR
  branch) without adh hard-coding a mobile target. Verified: a declared-tool table
  (device fails → confirms; oracle passes → clears; no-tool → built-in fallback). What
  stays deferred is providing the actual artifacts — a real differential-oracle target and
  an `adb` device binary — which are domain-specific (mobile port) and belong to the repo,
  now pluggable as §13 tools rather than baked into adh.
- [x] §19.1 unified diff *text*: `vcs.Diff(paths)` renders a unified diff of the
  HEAD-blob vs worktree content via the go-git handle, formatted with
  `github.com/hexops/gotextdiff` (no `git` binary — tests stay hermetic). The critic
  grounding carries it (`Grounding.Diff`, via a `critic.Inputs` bundle so the pure
  core reads no config or vcs), `step` gathers it best-effort for the critic stage
  (size-capped with a truncation marker), and `critic.tmpl` renders it. Follow-up:
  the post-commit tree-to-tree diff (go-git's native `(*object.Patch).String()`)
  for a critic that runs after a branch-per-arc commit — not needed pre-commit.
- [x] Populate `Arc.Labels` from Execution + smooth the exit-12 wall. Resuming a
  relayed execution turn derives area labels from the changed paths
  (`contextstore.AreaLabels`, top-level dir) and unions them into `Arc.Labels`, so
  the critic's context routes. The routing gap now requires an *environment*: a
  gap (exit 12) fires only when a context store exists (has units) yet routes
  nothing for a declared arc — an empty/absent store is ungrounded, not a gap
  (§19.1, updated). `adh init` scaffolds a starter store keyed by top-level dir so
  a typical change grounds out of the box. (`Arc.Proof` is set by `adh proof create`.)
- [x] Richer semantic labels than the top-level directory: `arc --label <l>... new
  <title>` (`cmd/arc`, ff `StringSetVar`) seeds `Arc.Labels` for finer context
  routing (§10); Execution unions its derived area labels on top. Follow-up
  (optional): deriving labels from the arc title.

## End-to-End Lifecycle Wiring

The arc lifecycle now runs end to end:
`arc new → run` (relays the stages, parks blocked at the ops gate) `→ approve →
close` (verifies proof, ships).

- [x] Strategy chooses the resolution (§12): `stage.Execute` defaults an unset
  one to a code change; `adh.ParseResolution` validates it.
- [x] `close` invokes `adh.CanClose` + `proof.Verify` — NO-PROOF-NO-CLOSE is
  enforced at the ship (exit 8 on a missing/mismatched proof).
- [x] `close --as <resolution> --proof <manifest>` ships an approved arc under
  the resolution-matched proof contract.
- [x] The `run` relay parks the arc at `StatusBlocked` at the ops gate (and a
  sub-ops gate below L2), so the `approve`/`reject` loop is exercised end to end.
- [x] Two cores wired into the loop: `authority.ModelGate` (enforced in
  `stage.Execute` — a judgment role on a fast-class model is EUNAUTHORIZED) and
  `metrics` (a `close` records the shipped arc's cost to the ledger the
  `metrics` command summarizes).
- [x] `context` + `tool` folded into the loop (§10, §13): every relayed stage now
  loads its routed context units and the available tools into the prompt (not just
  the critic), via `critic.ForStage` grounding all stages and the emit shells
  reading `toolreg.LoadRepo`. The loaded working set is recorded on `Arc.Context`
  (§10.3). Composes with `arc --label` (labels route context at strategy/execution
  before Execution derives paths).
- [x] `lesson` / `loop` / `worker` folded into the loop: `run`/`step` refuse with
  exit 9 (reason `requalify`) when the worker changed from the recorded epoch (§14,
  `worker.RequalifyNeeded`; a never-requalified workspace is ungated); `loop run`
  senses the invariant and opens an arc under the loop's owner on a departure (§15);
  and `lesson list` surfaces the Evaluation loop's candidate file, not just the
  confirmed registry (§11.1). Deliberately still manual: **lesson promotion** is a
  human-gated action (§11.2), and **loop scheduling** (`schedule = "daily"`) is the
  deferred crontab item — `loop run` is the manual/agent-driven trigger. `harness`/
  consolidate remains wired through `sleep`.
- [x] Evaluation fails an arc back to Execution for real (§4.1): a confirmed
  finding (`evaluation.Decide`) returns the arc to Execution and records a failure,
  bounded by `DefaultMaxReworks` — once the budget is spent the arc is marked
  `StatusFailed` (terminal) and `run`/`step` report a `failed` error outcome (exit
  1) so an autonomous drive stops for a human. Previously `StatusFailed` was
  defined but never set, and a pure mock run never confirmed a finding (its arc has
  no findings), so the edge was untriggered; it is now covered end-to-end.
  Follow-up (deferred): a `[evaluation] max_reworks` config key (a default constant
  today).

## Offload to a Mature Library (Undifferentiated Heavy Lifting)

Necessary but edge-case-heavy plumbing that is not adh's differentiated value.
The effectful interfaces (`model.Client`, `device.Validator`, planned `VCS`,
`Clock`) are the slot points: keep the mock for tests, wrap the library in an
`internal/<dependency>` adapter. Keep the policy cores (gate, oracle+invariant,
defect/lapse, autonomy ladder, NO-PROOF-NO-CLOSE, effectiveness) hand-rolled.

- [x] State writes are atomic and fsync-durable via the locally vendored
      `internal/atomicfile` (Tailscale, BSD-3-Clause; temp file + fsync + rename).
      Still to offload: `gofrs/flock` (cross-process locking) once the parallel
      manager writes one workspace from many arcs.
- [ ] `model.Client` (LLM): retries/backoff, streaming, tool-calls, token
      counting → the official SDKs (`anthropic-sdk-go`, `openai-go`). Never a
      hand-rolled HTTP/retry layer. **Deferred**: needs API keys + network (no
      deterministic test here), and `model.Relay` is the skill-driven backend the
      tool actually uses today.
- [x] `VCS` (branch/commit/status/diff/revert) → `internal/vcs` over `go-git/v6` +
      the `vcs` command; `Mock` for tests. `close` commits a `change` arc onto its
      own branch `adh/<arc-id>` past the approval+proof gates, and `reject` reverts
      the arc's paths to HEAD and returns it to Execution. `revert` is path-scoped
      and hermetic (go-git only) — the safety over a repo-wide reset+clean, which
      would discard the untracked `.adh` workspace. Only merge stays a `git`
      shell-out via `github.com/ldez/go-git-cmd-wrapper/v2` (go-git merge is
      experimental); branch-per-arc merge to base is the follow-up.
- [ ] `device.Validator` (adb) → the `adb` CLI or `electricbubble/gadb`.
      **Deferred**: needs a device; a shell-out adapter is untestable in CI.
- [x] `.adh/config.toml` + precedence → `internal/config` (SPEC §3 loader),
      decoding with `BurntSushi/toml` behind an explicit, pure precedence overlay
      (no config-framework globals — Viper avoided per go-advice §1/§3). See the
      Config wiring section.
- [x] Structured logging (§14) → stdlib `log/slog` on stderr (`root.Config.Log`,
      built in `cmd.Run` post-parse). `--verbose`/`--quiet` set the level
      (`root.LogLevel`: Debug/Error, else Warn) and `--jsonl` selects the JSON
      handler, so diagnostics stay on stderr separate from the stdout data plane
      (SPEC §8). The incidental warnings (close/reject) and a `run` stage-advance
      trace log through it with `op`/`arc` attrs. Follow-up: broaden Info/Debug
      tracing across the other stages/commands as needed, and redact secrets in log
      attrs alongside the sleep-evidence work.
- [x] Secret redaction in `sleep` evidence and staged text (§18.4): `internal/redact`
      wraps `github.com/betterleaks/betterleaks` (MIT) — `detect.NewDetectorDefaultConfig()`
      + `DetectString()` with the embedded ruleset — and replaces each detected
      secret with `[REDACTED]`, preserving context. `sleep run` redacts the proposed
      artifact, longitudinal guidance, and every evidence note once after `Plan`, so
      the top-level `evidence.jsonl` and all staged files (proposal, report,
      evidence, longitudinal) are scrubbed. Always on — the safe default (§18.5).
      Deferred follow-ups: a `[sleep] redact = false` opt-out (needs unset-vs-false
      handling), slog log-attribute redaction, and a repo-tunable ruleset/allowlist.
- [x] Scheduling (`sleep schedule`, loops §15): `adh sleep schedule
      add|list|remove|tick` persists cron jobs that fire an adh command on a
      cadence. `internal/schedule` holds the pure cron layer (`ParseCron`/`NextFire`
      over `robfig/cron/v3`, time as a parameter so it is deterministic), a SQLite
      job store (cgo-free `ncruces/go-sqlite3`), and a `Tick` executor behind a
      point-of-use `Runner` seam (the command's runner execs the adh binary; tests
      inject a fake). `tick` fires every due job, records its outcome, and advances
      its next fire — driven by one system-cron line or the agent, so adh stays
      stateless-per-invocation. The store code is adapted from a private scheduler
      (design reference only). Deferred follow-ups: a blocking `run` daemon (the
      store's `SoonestDeadline` + `Due` are the seam; signals/run-lock/launchd
      stay out), one-shot `at` jobs, per-job enable/disable, captured run history +
      GC, and auto-installing the `[sleep] schedule`/loop `schedule` config cadence
      (today `schedule add` is explicit).
- [x] `sleep schedule run` daemon: a blocking loop that ticks due jobs (reusing
      `Tick`), then sleeps `schedule.NextSleep` (the earlier of the next deadline
      and a 1-minute poll cap, floored) until the next tick, until the context is
      canceled. Graceful shutdown is free from `main`'s `signal.NotifyContext`
      (SIGINT/SIGTERM → the command ctx); a cancellation interrupting an in-flight
      query mid-tick returns cleanly (at-least-once on shutdown). Single-instance by
      convention — a cross-process run-lock is the same open gap as the arc store's
      missing flock, tracked there.
- [x] `sleep schedule` repo-root awareness: the store now opens under
      `cfg.repoDir()` (the `--repo` global, else cwd), so a daemon launched from
      elsewhere (e.g. launchd) finds the repo's jobs. `sleep`'s other paths and the
      global arc store stay cwd-relative — a broader repo-relative-state sweep is
      separate.

Keep hand-rolled (not offload candidates): the typed manifest/registry decoders
(the "parse at the boundary" idiom), JSON via `encoding/json`, hashing via
`crypto/sha256`, and CLI/flags via ff/v4 + climax (already offloaded).

## Housekeeping

- [x] CI: `.github/workflows/test.yml` runs build, vet, `go test -race`, golangci-lint,
  and a `go mod tidy` drift check on push/PR to `main`. (A boilerplate
  `gotmplfumpt` self-format step copied from another project was dropped — adh has
  no such binary.)
- [x] Release: `RELEASE_PROCESS.md` documents the GoReleaser flow (`.goreleaser.yaml`,
  already tracked) and the `gowheels` PyPI-wheel distribution. Follow-up: the
  `.github/workflows/postrelease.yaml` that doc references (auto-publish wheels on
  release) is not present yet.
- [x] All tracked markdown is formatted with `rumdl fmt .` (per `.rumdl.toml`:
  aligned tables, title-case headings, canonical rules/fences) and checked with
  `vale`. Only the vendored `internal/atomicfile/README.md` is left as upstream.
  Follow-up: `mdformat`/prettier are not installed here, and vale's prose
  suggestions (E-Prime, passive voice, em-dash/semicolon house style) are not
  enforced — treat them as advisory.
- [x] Man pages: `adh docs` renders roff man pages from the live `ff.Command` tree
  via `mango-ff`, so they never drift from the flags — the root page to stdout, or
  one page per command into `--dir` (`adh.1`, `adh-<command>.1`). The root page is
  enriched with the machine vocabulary ff cannot derive from flags: EXIT STATUS,
  REASON TOKENS (built from the `root.Reason*` constants), and ENVIRONMENT (the
  `AGENTIC_DEV_HARNESS_` override rule). Follow-up (deferred): richer per-command
  `LongHelp` prose (only root's placeholder was fixed) and a `--man` global flag /
  markdown reference are out of scope.

## Cross-repo alignment (2026-08-05 survey)

Surfaced by a survey across the skillet-family repos (skillet, exegesis, skillsaw,
canonizer, toerr). adh is the only CLI in the family that fails the base-scaffold
check its siblings pass.

- [x] **`climax lint` base-scaffold: add the unmatched-subcommand guard.** DONE (2026-08-05):
  `Run` now checks, after Parse, whether the selected command is a group parent
  (`Exec == nil`) with a leftover positional; if so it prints usage and returns
  `"<cmd>: unknown subcommand %q"` (exit 1). A bare invocation leaves no leftover arg and
  still returns `ff.ErrNoExec` (exit 0). This is the exact canonical shape exegesis and
  skillsaw use — `climax lint` reports **no structural drift**, and it also guards nested
  ff-group parents (e.g. `adh proof bogus`). A dispatcher test (`cmd/dispatch_test.go`)
  asserts a typo fails and is not `ErrNoExec`, a nested-group typo fails, and a bare
  invocation stays the exit-0 no-op. Note: the canonical guard prints the usage banner
  unconditionally, so under `--jsonl` a typo yields a banner on stderr plus exit 1 (the
  machine signal) rather than a JSON outcome — acceptable, since a mistyped subcommand is
  an operator error, not a machine-consumed result; conforming to the shared scaffold was
  chosen over an adh-only `--jsonl` refinement that `climax lint` rejects.
- [x] **Bump skillet v0.3.0 → v0.5.0.** DONE (2026-08-05): go.mod/go.sum only — no code
  change, 43 packages test green, `golangci-lint` clean. adh imports none of the packages
  that changed between v0.3.0 and v0.5.0 (`errs`/`identity`/`proof`/`ratchet`/`stats`/
  `atomicfile` were stable), and `go mod tidy` did not pull in `toerr` because adh imports
  nothing that reaches `ruleset/synthesize`. Because adh re-exports `skillet/errs.Error`
  as `adh.Error`, a later skillet errs→toerr consolidation would still reach adh's 157
  `adh.Error` sites for free. Restores the single-kernel guarantee skillet exists to provide.

## SkillLens dimensions: adh reimplemented what skillsaw already had (2026-08-08)

Source: `~/Documents/agent-orange/skillopt_changes_findings.md`. `internal/rubric` was
built "adapting skillsaw's rubric pattern" — the DET.SCORE floor, NeedsJudge, Diagnose —
"with adh's own dimensions." Three of those five dimensions turn out **not** to be adh's
own: `failure-handling`, `actionable-specificity` and `boundary-section` are the three
SkillLens quality dimensions (`microsoft/SkillLens`, arXiv:2605.23899), the same three
skillsaw scores as its dims 3/5/9. adh weights them at **60 of 100**, the heaviest
concentration anywhere in the family.

- [x] **Consume `skillet/skilllens` for `failure-handling` and `boundary-section`; delete
      the local detectors.** DONE (skillet bumped v0.7.0 → v0.14.0 first). `deterministicScore`
      now reads `skilllens.FailureMechanisms` (KindProse inline branch or KindSection
      failure-mode section) and `skilllens.BlacklistSections`; the private `hasFailureHandling`
      and `hasHeading` and their inline vocab lists are deleted, and `Evaluate` parses the doc
      once with `skillet/markdown`. Only the detectors moved — adh's weights, its 0..1
      `Deterministic` factor, and its five-dimension set stay local.
      **This was a convergence, not a 0-movement port** (adh's hand-rolled detectors were
      narrower than skilllens's, unlike skillsaw's, which skilllens was derived from). Measured
      on the repo's own docs: two moved 85 → 80, and reading the per-dimension reasons both
      changes were accuracy gains — `failure-handling` dropped a false positive (a prose line
      "a stage fails when …", which the old loose line-scan matched and skilllens's bounded
      regex correctly rejects) while `boundary-section` gained a true section the old narrow
      heading set missed. The shared-vocab fixtures (`## Boundary`, "if the build fails") still
      score 100/65 unchanged; new tests pin the de-false-positive and the richer-vocab
      recognition. `actionable-specificity` is NeedsJudge, so it has no deterministic detector
      to move.
- [x] **Give the three dimensions their provenance in the package doc.** DONE. `rubric.go`'s
      package doc now names failure-handling, actionable-specificity, and boundary-section as
      the microsoft/SkillLens dimensions (arXiv:2605.23899) with published tests/anti-examples,
      and states that `actionable-specificity` is NeedsJudge, so its base is supplied against
      SkillLens's stated definition.
- [x] **`internal/rubric` never states its own dimension count.** DONE. `rubric.go`'s package
      doc now calls it "adh's own five-dimension rubric ... distinct from the external
      skillsaw 9-dimension rubric adh consumes at its gate," so a reader moving between the two
      instruments has the cue. (`SPEC-ADDITIONS.md` and the harness/harnesscmd comments still
      say "judge-only dimensions" without a count; the authoritative statement now lives in the
      package doc where the rubric is defined.)

## Reasoning-toolkit survey (unified-thinking, 2026-08-05)

Source: a survey of `~/Documents/git/unified-thinking` (a deterministic Go reasoning
toolkit) for reusable techniques. adh is the biggest beneficiary — it emits confidences
everywhere and mines arc history, both of which have deterministic rigor to gain.

- [x] **Confidence calibration — scoped to the sleep optimizer.** DONE (2026-08-05), rescoped
  after verifying the premise was false: the *literal* item (critic/judge/evaluation confidence)
  has **no data** — `confidence` appears once in the whole codebase (a test fixture), Findings are
  categorical, and the judge/evaluation are deterministic pass/fail. adh emits no probabilistic
  confidence *by design* (it relays to a model and applies deterministic gates), so calibrating
  those stages would be an inert feature. The one genuine predicted→actual signal adh has is the
  **sleep optimizer**: `consolidate.Calibration(records)` runs `skillet/calibration` over the
  evidence log — each proposed-candidate cycle's projected score `NewScore` (already [0,1] from
  `gate.SelectScore`, so no invented mapping) is the prediction and `Status==keep` the outcome;
  no-candidate baselines (`NewScore==OldScore`) are skipped. `sleep status` now prints
  `calibration: <n> cycles  ECE … MCE … Brier …`, the cycle count shown so sparse (low-N)
  calibration is visible. Bumped adh to skillet **v0.7.0** for the package. This is the reliability
  axis complementing the significance `verdict` gets from `stats`. (The critic/judge instrumentation
  path — self-reported confidences persisted per stage — was considered and rejected as a large
  subsystem against adh's deterministic-relay grain.)
- [x] **FPR-based suppression for critic precision.** DONE (2026-08-05): built as the precision
  mirror of the coverage subsystem. The false-positive signal is **deterministic**, not a human-gate
  reject: `evaluation` adjudicates every finding (runs its named artifact) and `Dispose` splits them
  into `Confirmed` (ran and failed) vs `Unconfirmed` (surfaced but not reproduced = a false positive).
  New `critic/precision.go`: a `.adh/critic-precision.jsonl` log (`PrecisionEntry{Arc, Confirmed,
  Unconfirmed}` kinds, with multiplicity), `AppendPrecision`/`LoadPrecision`, and pure
  `NoisyKinds(entries, minSamples, maxFPR)` (a kind adjudicated ≥`DefaultMinAdjudications` with an
  unconfirmed share > `DefaultMaxFalsePositiveRate` is noisy). `evaluation.Apply` records a precision
  entry next to `recordStrata` (one shared site, both entry points); `cmd/run` reads it and feeds the
  noisy kinds into the critic prompt (`Inputs.Noisy` → `critic.tmpl`), holding over-flagged kinds to a
  higher bar — the relay-driven analog of "suppress." **Safety:** it is a prompt hint, never a hard
  gate — a confirmed finding (ran and failed) is a real defect regardless of its kind's history, so it
  is never skipped. Refactored the coverage/precision JSONL boilerplate into shared `appendJSONL`/
  `loadJSONL` helpers. (Severity-gating and a config knob for the thresholds are deferred — the hint is
  safe for all kinds, so gating isn't needed for correctness.)
- [x] **Deterministic trajectory mining in the `sleep` consolidation stage.** DONE (2026-08-05),
  scoped after verifying overlap: `consolidate.Reflect` **already** mines recurring failure/success
  modes (via `lesson.Distill`), and `toolrun.Record` has no `Arc`, so per-arc *tool* sequences don't
  exist. The genuine, non-redundant increment shipped is **arc-coverage**: `consolidate.CommonClasses`
  counts, per lesson class, the fraction of harvested arcs whose failure/success signals distill to it,
  and returns those in >`DefaultCommonCoverage` (50%) of arcs — "what most arcs consistently do,"
  distinct from `Reflect`'s raw `Mode.Count` (which one noisy arc can inflate). `sleep status` surfaces
  them (`common <kind>: <class> (<pct>% of arcs)`) as evidence for the operator's lessons, mirroring the
  calibration line. Pure miner; model-free. Deferred (no data / redundant): tool-sequence hashing + RRF
  (no per-arc tool log), and richer root-cause heuristics (the grounded/ungrounded `failures.RootCause`
  and failure-mode mining already exist). This closes the reasoning-toolkit survey items for adh.
- Lower-value / deferred: a *seeded* Thompson-Sampling strategy selector (gated on a
  `skillet/bandit` — see skillet's TODO; weigh the seeded stochasticity against adh's
  determinism preference); weighted-sum MCDA for multi-criteria evaluation scoring; and
  inequality-SAT conflict detection (`validation/symbolic.go`) for NFR constraints.
- Deliberately NOT adopted: unified-thinking's keyword bias/fallacy/blind-spot detectors —
  brittle, uncalibrated heuristics that adh would either enforce deterministically or relay
  to a model, never split the difference.

## Agent-Red Survey (2026-08-15)

Source: a survey of `~/Documents/agent-red` (26 agent-tooling projects). Most of what looked
promising from READMEs turned out to be weaker than what adh already has; the two items
below survived checking against the code, and the third is a retraction.

- [ ] **Evidence records have no staleness state.** `internal/evidence` is an append-only
  log with `Timestamp`, validate-on-write, malformed-line-is-a-hard-error — the right shape.
  But a grep for `stale` across `internal/` returns nothing: when the artifact an evidence
  record measured changes underneath it, the record stays exactly as authoritative as the
  day it was written, and `NO-PROOF-NO-CLOSE` is satisfied by a proof of something that no
  longer exists. `proof.Verify` catches this for *declared artifacts* by re-hashing; nothing
  catches it for the evidence log. `goalx/cli/freshness_state.go` carries the vocabulary
  worth taking: a four-state enum — `fresh`, `stale`, `unknown`, `not_applicable` — with
  `LatestRevision` vs `CurrentRevision` and a `Reason` per item, split across cognition
  facts and evidence items. **The four states are the point.** `unknown` (never evaluated)
  and `not_applicable` (no revision to compare) are distinct from `stale`, exactly as
  `skillet/timeseries` keeps `Verdict.Compared` distinct from a zero baseline — absence of
  a comparison is not a passing comparison. Deterministic and model-free: it is a revision
  comparison, not a judgment.
  src: `agent-red/goalx` `cli/freshness_state.go`.
- [ ] **Capability routing is a missing axis, not a missing feature.** The autonomy ladder
  governs *how much* an agent may do; nothing expresses *which* agent should take a given
  arc. `clu` declares agent capabilities in config and routes unassigned work by `cap:*`
  label, with a well-defined shared pool (`assignee IS NULL` and no `cap:*` label) so
  unlabelled work is never stranded, and it refuses at creation time to attach a `cap:` a
  no declared agent could match — the failure mode that makes label routing rot. Relevant
  because the harness is meant to serve a team with genuinely different skills, and today
  arc assignment carries none of that. Not urgent; recorded because the shape is right and
  the refuse-unmatchable-label detail is the part that is easy to omit and expensive to
  retrofit.
  src: `agent-red/clu` `internal/cli/{create,batch}.go`, `internal/config/config.go:247`.
- **Retraction — adh's redaction is already the better design; do not adopt `pantry`'s.**
  An earlier pass suggested lifting pantry's "3-layer redaction". Checked: `internal/redact`
  offloads detection to `betterleaks`' maintained ruleset and is a thin wrapper, which is
  exactly what skillet's *Preserve Mature Libraries* hard constraint asks for.
  `pantry/internal/redaction/redaction.go` hardcodes ten regexes maintained by nobody,
  including `password\s*[:=]\s*["']?.+`, which is greedy to end of line and will swallow
  surrounding context — the opposite of adh's tested property that redaction preserves the
  context around a secret. **One idea does survive:** pantry's first layer honours explicit
  `<redacted>…</redacted>` tags, letting an author mark a secret no pattern would catch.
  That is a genuine complement to pattern detection rather than a competitor to it, and it
  is a few lines. Cheap to add to the wrapper; no ruleset change.
- Note, for the same reason: `goalx`'s `objective-contract` / `obligation-model` /
  `assurance-plan` looked like a richer NFR model from its README, and is not.
  `internal/nfr` is **Planguage-quantified** — `Scale`, `Meter`, `Direction`, and ordered
  `Fail`/`Goal`/`Stretch` against a validated taxonomy tag, rejecting a goal ordered worse
  than its own fail threshold — which is strictly more rigorous than typed obligations with
  prose bodies. goalx's contribution to adh is freshness and nothing else. Its
  refuse-under-resource-pressure behaviour (never silently shrink fan-out to hide pressure)
  is philosophically aligned with the no-self-granted-gates rule but has no adh consumer:
  adh does not own a fan-out to shrink.
- Deliberately NOT adopted: `beadwork`'s git-orphan-branch state store with intent replay
  (genuinely good for multi-writer state, but adh's `contextstore` + worktrees already have
  an owner and no observed contention); `4x`'s role isolation (adh's cold critic is the same
  invariant, reached independently, and adh's is stronger — it withholds the builder's
  transcript *and* requires a fresh sub-agent).

## Agent-Blue Survey (2026-08-15)

Source: a survey of `~/Documents/agent-blue` (22 projects — the sources the practice came
from). adh is the tool most of it lands on. Checked against the code.

- [ ] **`consolidate.Harvest` only learns from work adh itself drove.** `Harvest(arcs
  []adh.Arc)` reduces *closed arcs* to signals — so a correction only accretes if the work
  became an arc. The team's actual corrective interactions happen in Claude Code, Gemini,
  and Qwen sessions that never entered the loop, and those are exactly the interactions the
  practice claims to make accretive. `agent-blue/SkillOpt/skillopt_sleep` is the reference:
  `harvest_codex.py`, `harvest_copilot.py`, `harvest_copilot_cli.py`, `harvest_cursor.py`,
  `harvest_pi.py` normalize foreign session stores into one `SessionDigest`, and
  **`mine.heuristic_mine` extracts training units from them deterministically** — its own
  docstring: "detects retry chains (a prompt re-asked after negative feedback => the early
  attempt failed), extracts the user's recurring intents, and labels outcomes from feedback
  signals… what makes the whole cycle runnable offline and is the basis of the deterministic
  experiment." An optional `llm_mine` produces richer records and **falls back to heuristic
  on error**. That is precisely the split adh already enforces: mechanism in code, judgment
  relayed — with the model as enrichment over a deterministic floor rather than the floor
  itself. Retry-chain detection needs no model and no cooperation from one.
  Scope note: harvesting foreign session stores is a read of someone else's on-disk format
  and will rot; keep each harvester behind one normalizing seam so a format change is one
  file, as SkillOpt does.
- [ ] **Log why the deterministic path did not fire, every time judgment is relayed.**
  `agent-blue/virgil` runs a layered router — exact match → keyword index → category
  narrowing → AI fallback — and claims 80%+ of queries never reach a model, with the AI
  surface shrinking over time. The mechanism is one small file,
  `internal/router/misslog.go`: an append-only JSONL of
  `MissEntry{Signal, KeywordsFound, KeywordsNotFound, FallbackPipe, AIPlan, AIConfidence,
  Timestamp}`. **`KeywordsNotFound` is the load-bearing field** — it records *why* the
  deterministic layers missed, so the miss is directly actionable rather than merely
  counted. adh relays each stage's judgment by design, but nothing distinguishes "no
  deterministic check could decide this" from "a check exists and did not fire". Without
  that distinction, "minimize the model" is unmeasurable. Cheap: a record per relay,
  alongside `evidence`.
- [ ] **A critic that declares what it did *not* examine.**
  `agent-blue/super-hermes/skills/prism-scan/SKILL.md:57` appends a **constraint footer** to
  every findings table — "This analysis maximized X. It did not examine: [1-2 specific
  alternative angles]" — and `prism-reflect` writes a fuller constraint report to
  `.prism-history.md`, which **later `prism-scan` runs read to steer their lens away from
  angles already exhausted**. Today a cold-critic reply with no finding in an area is
  indistinguishable from that area not having been looked at, and `evaluation` disposes of
  the arc on that silence.
  **This does not compromise cold-context independence**, and that is the point worth
  recording: a coverage history says *what was looked at*, never *what was concluded* or how
  it was built. It is categorically different from the builder's transcript, which `adh`
  withholds. Feeding prior coverage into a fresh critic biases it toward unexamined ground —
  the opposite of the contamination the cold split exists to prevent.
- [ ] **A gate before Strategy: make the arc earn the right to start.**
  `agent-blue/mycellium-harness` opens with four questions — what is the problem, who has
  it, what is the riskiest assumption, what is the smallest move that tests it — before an
  editor opens, and its stated invariant is that "what the agent won't do is silently skip
  past missing evidence and call the work done." Crucially, **depth is negotiable and
  skipping is not**: a weekend hack draws lighter prompts than a team product, and a user
  may decline depth at any step, but never silently. adh's arc begins at Strategy, which
  plans *how*, and takes *whether* as given. This is the validation half of the
  validation/verification pair (`agent-blue/nfrs-guide`: validation is "are we building the
  right thing — requirements examined for conflicts and assurance they meet need";
  verification is "are we building it right"). adh is currently all verification.
- [ ] **"Every instruction the agent reads must be backed by a working command."** Principle
  1 of `agent-blue/agentic-harness-bootstrap`, enforced by
  `templates/verify-harness.sh.tmpl`, whose first check **parses the module table out of
  `ARCHITECTURE.md` and verifies each path exists** — the document's claims about the repo
  checked against the repo, on every CI run. `doctor` checks harness integrity and
  `context verify` checks context drift; neither checks that an instruction's *commands*
  resolve. Same class as the §13 tool registry, pointed at prose instead of at findings.
- [ ] **An NFR evaluation set, and a taxonomy to reconcile against.** `internal/nfr` is
  Planguage-quantified and grounded (`agent-blue/nfrs-guide` is the source of that
  discipline), but we have no ground truth for whether a *document* was correctly mined for
  NFRs. `agent-blue/NFRLocator` carries labeled corpora — iTrust, CCHIT ambulatory criteria,
  OpenEMR, PromiseData, RFPs, CFRs, DUAs — plus ARFF exports of labeled sentences and a
  category listing, alongside a `SPEC.md` for a Go reimplementation.
  **Take the corpora and the taxonomy; do not take the classifier.** The pipeline is
  Stanford dependencies + WordNet + Weka SVM/Naive Bayes over lemmatized sentences (~19K LOC
  Java, a WordNet dictionary, a 4GB heap) — a statistical classifier with no per-decision
  provenance, which is the uncalibrated shape this family rejects. Its one transferable
  design detail is `classification_attributes.json` (§6.5): the classification config
  externalized as data, the same move recorded for skillsaw's rubric.
- [ ] **Invariants as a second, independent net beside the differential oracle.**
  `agent-blue/evals-differential-oracle` is the source of `oracle`'s differential self-test,
  and adh took the differential half. It has a second, separate net: `src/invariants.py` —
  "the rules any correct implementation must obey, **checked against a board independently
  of how the result was produced**". A differential comparison needs two implementations and
  proves only that they agree; an invariant needs none and holds even when both are wrong
  the same way. `oracle diff` covers the first; nothing covers the second.
- Deliberately NOT adopted: `mycellium-io`'s peer-agent coordination layer (its
  "fail to recognise disagreement" framing is apt, but it reports coordination paying off at
  3+ concurrent agents and adh drives one arc at a time); `hermes-dojo`'s Telegram/report
  delivery and learning-curve dashboards (the *signal* it mines is the valuable half and is
  covered by the session-mining item above; the reporting surface is not adh's job);
  `PolyBrain`'s multi-model synthesis (adh's model-gate is a convention, and cross-model
  verification would need a model-invocation seam adh deliberately does not have).

## Agent-Fuschia Survey (2026-08-18)

Source: a survey of `~/Documents/agent-fuschia` (26 repositories). Three items, two of
which sharpen invariants adh already holds rather than adding new ones.

- [ ] **"Cannot verify" must not read as "verified" — and `eval` has the same seam.**
  `agent-fuschia/vac-gate` enforces two rules worth copying verbatim, both about refusing to
  let an absence pass as a success: `binding-unrecorded` ("unrecorded is not matching" — a
  declared key the contract never recorded *fails*, rather than being skipped), and the
  sharper one, **"'cannot regrade' is not 'regraded'"** — the issuer regrader's honest
  stale-code refusal fails the gate.
  Where this lands: `eval` disposes of an arc by running the artifact each critic finding
  names — a proof manifest, a built-in check, or a §13 repo-declared tool. A tool that
  cannot run (missing, unbuilt, wrong commit) is a third outcome, distinct from "ran and
  passed" and "ran and failed", and the disposition rule for it should be stated rather
  than emergent. This is the same not-applicable state now required of `quotecheck` and of
  canonizer's `anchor-absent` — three consumers, which is why it is worth naming once.
- [ ] **Grade the artifacts, not the account.** `agent-fuschia/agent-certlab` runs an agent
  against tasks with **seeded, known defects** and grades "only the artifacts it leaves on
  disk… **never the agent's own account of its success.**" adh's cold critic withholds the
  builder's transcript, which is most of the way there — but the critic still reads a
  *reply*, and `evaluation` adjudicates findings the critic asserted. The artifacts-only
  rule is stricter: the ground truth is the working tree and the proof packet, never a
  narration of either. Worth auditing which parts of the arc already meet it (proof
  verification does; finding disposition partly does) and stating the rule where they do.
  The seeded-known-defect technique is also the natural extension of `oracle selftest`
  from "does the control discriminate" to "does the whole arc catch a planted defect".
- [ ] **Name the refusals in `SPEC.md`.** `vac-protocol` §7 is titled "Explicitly refused
  in v0.1" and opens: **"Named refusals, so their absence reads as a decision rather than
  an oversight."** Its entries earn it — signatures are refused because "a signature proves
  who spoke, not that they spoke the truth… signing an unreplayable bundle would launder
  it"; Docker images because "shipping opaque filesystem images as 'reproducibility' hides
  exactly the drift this protocol exists to surface."
  adh has more of these than any tool in the family — no model invocation, no self-granted
  gates, no `--yes` bypass, no environment-variable override, no envelope of its own — and
  they are currently distributed through prose and inferable from tests. Collecting them
  into one refusals section costs nothing and stops a future contributor from "fixing" an
  absence that was a decision.
- Corroboration, no work implied: `engineering-notebook` ingests **both** Claude Code and
  Codex transcripts into daily summaries — a second reference implementation for the
  session-mining item above, after `SkillOpt/skillopt_sleep`. Read both before writing the
  adapters; two independent readers of the same on-disk formats are worth more than one.
- Deliberately NOT adopted: `agent-graph`'s swappable `MockPolicy`/`LLMPolicy` seam (a good
  answer to testing a nondeterministic policy, but adh's relay posture already achieves it
  by never invoking a model — the mock is the human); `vac-protocol`'s bundle format (adh's
  `proof` packet is the same idea scoped to one arc, and VAC's claims are about system
  capability rather than about a change).

## Deep Reads — `oh-my-agent`, `ruflo`, `superpowers` (2026-08-22)

Three repositories from `~/Documents/agent-green` that the first survey pass had filed as
read-shallowly, opened. Written up in gnosis's `manifesto.md`; these are adh's. They were
recorded against gnosis first, which was the wrong home for a backlog belonging to this
tool.

Two of the three are about the arc loop's honesty — what it records when a stage runs in a
degraded form, and what it may call an improvement. The third is about the seam between a
skill's wording and a harness's tools, which `toolreg` already half-owns.

### The Critic, and What It Records About Itself

- [ ] **A critic that could not run cold must say so, and there is no state for that
  today.** §19.1's grounding contract denies the builder's transcript, which is the
  right mechanism and is the whole basis for calling the critic independent.
  `oh-my-agent`'s judge protocol reaches the same design — a spawned subagent with fresh
  context, briefed on the criteria and never on what the implementer claims it fixed,
  and it says outright that *"independence is structural, not a prompt-level
  role-play."*

  What it has that adh does not is the degraded path. When the runtime cannot spawn a
  fresh context it runs the protocol inline **and emits an event recording the
  downgrade** (`ralph.judge-inline-fallback`). adh's critic is either cold or the arc
  does not advance, which is stricter and therefore fine — until the first environment
  where the cold path is unavailable, at which point the pressure is to relax the
  contract silently. **Neither `checked` nor `unchecked` covers *checked under reduced
  independence*.** Add the third disposition before it is needed, so the answer to that
  pressure is a recorded downgrade rather than an edit to §19.1.
  **CLOSED 2026-08-22: the state cannot occur, and the entry described the wrong risk.**
  `oh-my-agent`'s downgrade event exists because its judge is a **spawned subagent** — a
  runtime operation that can be unavailable, so it falls back to inline and records that it
  did. adh spawns nothing. `critic.Ground` assembles labels, paths, proof and acceptance bar
  and the package doc states it *"never carries the builder's transcript"*; there is no
  branch where it could, and adh never invokes a model at all. So *the cold path is
  unavailable* has no code path to reach, and a field for it could only ever be empty —
  worse than absent, because an empty downgrade field reads as evidence of an isolation
  nobody checked.
  **The real limit is different and belongs where §19.1 states the guarantee:** the critic
  is cold **by construction of the prompt, not by isolation of the reader.** adh emits a
  prompt and something external runs it; if the session that built the code also runs the
  critic prompt it holds the transcript in its own context even though the prompt does not
  contain it, and adh cannot see that. `model.Relay` is `{Response, Class}` — the capability
  tier and nothing about *who* answered or in what session, so there is no identity to
  compare against the builder's.
  The guarantee is that the transcript is not **supplied**. It is not that the critic did
  not **have** it. Same narrowing this family already accepted for content-addressing
  detecting accidents rather than tampering, and for checksums not being authentication: a
  guarantee stated more broadly than it holds is worse than a narrow one, because a reader
  stops looking.
  **Rejected — a self-declared "this was a fresh context" field.** Testimony from the one
  party that benefits from misreporting it, and unlike `finding.Unexamined`, where a critic
  gains nothing by lying, the incentive here runs the wrong way. It would let a reader treat
  an unverifiable claim as a check.
  **The only real fix is session identity on the relay**, refusing a critic reply whose
  session matches the builder's. That is a feature rather than a field — the relay has no
  session concept, and the model-gate is convention-only, which is where it belongs.
  Recorded against that item so this has a trigger that can fire instead of one that cannot.

- [ ] **`Dispose` throws away a distinction its own input type already carries.**
  `Adjudicated` has three fields — `Finding`, `Ran`, `Failed` — and
  `disposition.go:79` collapses them to two buckets: `if r.Ran && r.Failed` is
  confirmed, *"every other case — it passed, or no artifact ran — is unconfirmed."*
  So **`Ran: false` and `Ran: true, Failed: false` land in the same bucket**, and both
  flow to `LessonNotes()` and become §11 lesson candidates.

  An artifact that could not run is not evidence the finding was wrong. It is the
  `Unchecked` case, the family's most-derived discipline, and here it reads as *the
  critic was mistaken* — which is the flattering direction, since it advances the arc.
  A finding whose artifact was unrunnable is also the one most worth surfacing: it
  means the critic named something the repository cannot check.

  The information is already in hand, so this is a third `Verdict` slice and a decision
  about what it gates — not a modelling change. It does change which findings become
  lesson candidates, and `LessonNotes()` is currently drawing on a mixed population.

### What the Arc May Call an Improvement

- [ ] **Guard invariants belong in the acceptance bar, declared before the run.** `ruflo`'s
  nightly loop is the negative control here and the positive one, four weeks apart. Its
  earlier optimisation loop hill-climbed a single objective and produced two documented
  failures: it raised a harness score 40 → 55 by adding files a presence-counting metric
  rewarded, with no capability change, and it doubled a promotion rate by relaxing the
  promotion predicate from `AND` to `OR` and logged it as `"BIG WIN"`.

  Its later loop rejected two changes carrying large favourable primary metrics —
  *"max-load −46.1%, CoV −44.4% **but density −13.7% breaches ±10% invariant**"* and
  *"latency −55.9%/−57.5% **but recall@10 breaches 0.90 floor at N=8000**"* — because a
  **named guard** moved, under a hypothesis marked *"Frozen before evaluation began; not
  modified after seeing results."*

  adh's arc carries an acceptance bar and `internal/nfr` is already Planguage-quantified
  with Fail/Goal/Stretch, which is most of the machinery. What is missing is that the
  bar names only what the change is *for*; nothing names what it must not cost. **An
  objective without guards is hill-climbed by trading away everything unmeasured.**

- [ ] **`ACCEPT-scoped` is a fourth verdict and the concern travels as a number.** The same
  loop's admitted-with-reservations rows carry the reservation as a signed measurement
  rather than prose — *"low-bucket recovery −17.6% (t=7.00, held); med-bucket null (no
  generalization)"*, *"overall recall@10 +0.267; category B (pure-paraphrase) regresses
  −0.133"*. `haft`'s `review_ready` is the same verdict from a different project. adh
  advances or returns to Execution; there is no disposition for *advanced, with the
  regression recorded*, and the honest cases exist.

- [ ] **Freeze the hypothesis before the evaluation, mechanically.** The discipline above
  only works if the bar cannot move after the numbers arrive. adh writes the arc's
  acceptance criteria at Strategy and evaluates at Evaluation, with nothing preventing
  an edit in between. Hash the bar at Strategy and record the hash with the verdict; a
  changed bar is then visible rather than deniable. Small, and it is the difference
  between a pre-registration and a note.

### Actions, Tools, and What Degrades

- [ ] **`toolreg` describes tools; nothing describes the *action* a stage needs.**
  `superpowers` runs one skill corpus across nine harnesses on a rule worth taking
  whole: **"Skills name actions, not tools."** A skill body says *dispatch a subagent*,
  *create a todo*, *read a file* — never a tool name — and a per-harness mapping
  resolves each action to the real tool. That is `lexicon`'s Keyword/Role split applied
  to tool invocation, with the stable layer (actions) and the volatile layer (tool
  names) the right way round, and it is why their skill bodies never fork per harness.

  `toolreg.StarterRegistry` is the mapping half, already built — `Run`, `Verifies`,
  `RepairHint` per capability. The missing half is the vocabulary the prompts are
  written against: adh's emitted prompts name commands, so a stage's requirement and
  the way it is satisfied are the same string, and porting to a harness with different
  tooling means editing prompts. Name the actions, keep `toolreg` as the resolver.

- [ ] **A capability that is absent needs authored fallback wording, not a repair hint.**
  `RepairHint` carries the install line, which answers *how do I get this*. It does not
  answer *what do I do without it*, and those are different questions — the second one
  matters in an environment where the tool cannot be installed. `superpowers`' porting
  checklist gives every capability an `If absent` column and defines **degradable** as
  the skill already carrying fallback wording for the missing tool, with one prohibition
  that is the whole point: *"never to invent a `Task` call."* An agent that cannot
  dispatch a subagent must do the work inline or report the missing capability. It must
  not synthesise a call to something that is not there.

  Better than probing for capabilities in the runner, because the fallback is authored
  by whoever wrote the stage and knows what the stage actually needs.

- [ ] **`adh init` seeds `.adh/tools.json` — check it against the never-edit-the-user's-files
  rule.** `superpowers` states it flatly: *"Everything ships through the harness's own
  install mechanism. Never edit the user's files… The harness owns what it loads; your
  install artifact is the only thing you get to write."* `init` writing into the
  repository it was invoked in is fine by that rule, and the rule is worth writing down
  before something reaches for a global config. It also contradicts `mdm`'s `rules link`,
  which symlinks forty-five agent filenames into the user's tree; recorded as a contested
  pair, with superpowers holding the better argument — an installer that edits user
  config cannot be cleanly uninstalled or reasoned about.

### Testing the Seam `adh` Does Not Test

- [ ] **The relay posture makes most testing easy and one case impossible, and there are
  three methods, not two.** The Agent-Fuschia entry above concludes that adh's relay
  *"already achieves"* deterministic agent testing *"by never invoking a model — the mock
  is the human."* That is right about determinism and it leaves the same gap gnosis has:
  nothing establishes that an agent handed a **real emitted prompt** produces a reply the
  next stage accepts. gnosis §18.6 now enumerates the three options and adh has the same
  choice:

  | Method                           | Runtime                  | Reasoning                  | Assertion                          | In CI                                    |
  | -------------------------------- | ------------------------ | -------------------------- | ---------------------------------- | ---------------------------------------- |
  | Hand-written replies (today)     | none                     | authored                   | direct                             | yes, and proves nothing about the prompt |
  | Scripted model                   | real binary, real prompt | local server from a script | on the request *and* the reply     | yes                                      |
  | Real model, mechanical predicate | real                     | real                       | pure predicate over the transcript | no                                       |

  Two disciplines carry over. From the second: **assert on what the agent sent** — a
  fixture that only dictates replies is a playback, and it must fail when the request
  arrives without the fields the prompt was supposed to carry. From the third:
  `superpowers/tests/explicit-skill-requests/` runs the real agent under an **isolated
  `HOME`** and greps machine-readable transcript events, and its second assertion is the
  instructive one — not only that the required step happened, but that **nothing else
  happened first**, against an explicit allowlist of actions that do not count. Ordering
  is not a property a prose reply can be trusted to report about itself.

- [ ] **No map says what each suite covers that the others do not.** adh runs the
  differential oracle, the invariant checker, and `gate.SelfTest`.
  `superpowers/docs/testing.md` annotates every test with its coverage delta against the
  other harness — *"drill covers the YAGNI subset; bash adds commit-count, task-tracking,
  and token telemetry assertions"*, *"tests description-recall, not behavior"* — and
  states plainly which suite is **not** in CI and why. Cheap, and it is what makes a
  redundant-looking test defensible rather than deletable.

- Note, not a work item: `superpowers` publishes *"This repo has a 94% PR rejection rate"*
  and requires every contribution to disclose model, harness, harness version, and installed
  plugins, on the reasoning that *"agent-generated content reasoned from documentation is
  held to a different bar than work grounded in a real session."* That is a testimony
  distinction — whether the witness was present — rather than an authorship one, and it is
  the basis the family's still-missing `AI_POLICY.md` should use. Recorded against gnosis,
  where the policy item lives.

## Applicability Is a Rule, Not a Type (2026-08-22)

`skillet` closed its open note about naming a general `Applicability` mechanism, and adh's
`HasCodeBlock` use was one of the five sites counted. The answer is **no type**: the five
sites each suppress a different thing — a deduction, a whole check, or nothing — because
their output shapes differ, and each already carries a reason. Full reasoning in
`skillet/TODO.md`.

- Nothing to build. Recorded because adh's handling is cited as one of the conforming
  cases and should not be flattened by someone who has not read the rule. `internal/rubric`
  returns **full credit plus a reason** when the dimension cannot apply — *"no failure
  branch, and the artifact executes nothing to fail"* — rather than docking or skipping
  silently. That is the correct shape for a 0..1 factor, and it is a different correct
  shape from skillsaw's flag and gnosis's skip record. The rule: *applicability is derived,
  not declared, and a run states what it skipped. What gets suppressed is the consumer's
  choice; the reason is never optional.*
- [ ] **Worth one pass: is any other adh gate suppressed without a reason?** The rule is
  cheap to check and adh has several places that decline to act — `toolreg` entries whose
  capability is absent, `critic` when an arc routed no context, `eval` when a named
  artifact could not run. The last of those is already filed above as `Dispose` discarding
  `Adjudicated.Ran`, which is the same defect in a different wrapper: an artifact that
  could not run is not evidence the finding was wrong. Sweep for the rest rather than
  waiting to trip over them.

## Adopt `finding.Unexamined` (2026-08-22)

`skillet/finding` gained `Unexamined{Aspect, Reason}` and a `Result.Unexamined` field,
promoted on adh's *"name the refusals"* entry plus canonizer's matching cold-critic one —
two consumers with present defects. adh's defect is concrete: `critic.ParseFindings`
documents that *"an empty or absent list is a clean review"*, and `evaluation` disposes of
the arc on that silence, so a critic that looked at nothing and a critic that found nothing
reach Ops identically. Design record in `skillet/TODO.md`.

- [ ] **Parse `unexamined` in `critic.ParseFindings` and carry it on the arc.** The parse
  is already the right shape — it validates each finding and rejects a malformed reply with
  `EINVALID` rather than advancing on trust — so extend it: an entry failing `Valid()`
  (either field empty or whitespace-only) rejects the reply the same way a finding with no
  summary does. Do not drop invalid entries silently; that is how a reply saying nothing
  passes for a reply that found nothing.
  Advisory by construction: `Result.HasBlocking` iterates `Diagnostics` only, so a declared
  gap cannot change a verdict however `Dispose` is written.
- [ ] **`critic.Ground` should offer the prior run's gaps, and this is the one place it
  helps rather than contaminates.** §19.1 withholds exactly one input, the builder's
  transcript, and that is what makes the critic cold. A previous critic's *coverage* is not
  the builder's reasoning and not a conclusion — it is a record of angles already taken —
  so feeding it forward steers a fresh critic toward unexamined ground, which is the
  opposite of the contamination the cold split prevents. Both source entries argued this
  independently; worth stating in §19.1 so the exception is deliberate rather than
  discovered.
- Related, and **not** the same item: `vac-protocol` §7's *named refusals* belongs in
  `SPEC.md` as prose — a section saying what adh deliberately does not do, so an absence
  reads as a decision. It shares the discipline and shares no code, and it was previously
  counted alongside the coverage record as though four entries described one mechanism.
  Three of the four did; this one is a document.

## Commissioned Gap Report, Round Two — One Idea, Not From the Report (2026-08-22)

Source: `~/Documents/agent-green/FPF/agentic-dev-harness_todo.md`. Its verdict is *"no
genuinely unimplemented, valuable, and relevant gaps found"*, and that verdict is correct.
**Full reasoning for the family is in `skillet/TODO.md` under "Round Two, and What Asking
for Code-Reality Verification Actually Bought"**; the short version is that round two's
findings across all seven repositories are restatements of the backlogs its verification
step read, and the four *no-gaps* verdicts are the honest output of that process.

One thing in its addendum is genuinely absent here and is recorded on its own merits rather
than as a finding, because the report supplies no evidence for it beyond a corpus reference:

- [ ] **A Critic finding says who can close it and not what to run.** `finding.Action`
  carries `automatic` / `guided` / `human`, which is *who*; `coherence` emits a
  `recommended_next_command` alongside, which is *what*. For a five-stage harness whose
  whole output is consumed by an agent deciding its next move, the difference is real: an
  agent handed "guided" has to infer the command, and inferring it is the step most likely
  to go wrong quietly.
  **Two objections have to be answered before this is built, and the second is the hard
  one.**
  - `skillet`'s backlog already refused `FixClass` as a duplicate axis of `Action`. This is
    not that — a command string is not a classification — but it must not become a third
    way of saying `automatic`.
  - **A generated command an agent then executes is an execution surface.** gnosis §15's
    rule is that anything an agent can name which later selects a file, a command, or a
    check must be validated against a closed set rather than used. A
    `recommended_next_command` assembled from a finding's own fields — a path, a rule id —
    is exactly the shape that rule exists for. The defensible version emits a **closed
    enumeration of remediation kinds** plus their arguments, and lets the caller render the
    command, rather than emitting a string for something else to run.
  Filed as a design question, not as a gap. If it is built, the enumeration is the design.

## `Critic.Deny` Is a Dead Knob That Looks Like the Guarantee (2026-08-22)

Found while verifying that adh's critic has no code path carrying the builder's transcript.
It does not — and the config says otherwise.

- [ ] **`Critic.Deny: []string{"transcript"}` is declared in `config.go:147` and read by
      nothing.** Zero non-test references anywhere in the repository. The actual guarantee
      is structural and lives elsewhere: `prompt.view` omits `History` for the critic, and
      the type comment states it plainly — *"the guarantee is enforced here in the data, so
      a hand-edited critic template still cannot leak the transcript."*
      **The defect is not the unused field, it is what a reader concludes from it.** A
      deny-list named `transcript` sitting beside `GroundFrom` reads as the mechanism that
      excludes it, and as configurable. Neither is true: removing `"transcript"` from
      `Deny` changes nothing, and adding another name to it protects nothing. Someone
      hardening the critic would edit the wrong thing and believe they had succeeded.
      This repo already has the principle. §4.2's *"a finding nobody can act on is noise"*
      and the `in_degree_cut` entry — *"declining to build a reader is a decision worth
      writing down"* — are the same shape one level up: a value nobody reads is a claim
      nobody checks.
      **Three ways out, and the choice is a real one.** Delete the field, since the
      guarantee does not need it. Or read it — have the renderer assert that every name in
      `Deny` is absent from the view it builds, which turns a decorative list into a
      belt-and-braces check and makes the config honest. Or keep it and document it as
      declarative-only, naming where the enforcement actually is. The middle option is the
      only one that makes the field earn its place, and it is small; the first is the
      cheapest and loses nothing real.
      Related: gnosis's `doctor` grew an unread-value check for exactly this class, and
      distinguishes *consumed* / *pinned* / *unread* because two states were not enough.
      `Deny` is `unread` today and reads as `consumed`.
