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
      **Residual follow-up:** auto-log tool runs from `context verify`/NFR adjudication too
      (today `adh tool run` — the relay's actual tool path — is the source), and a
      `run_duration_ms` metric once timing is recorded.
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
- [ ] Deferred — global `--yes`/`--dry-run`: local today (approve owns them for the
  safety gate). Binding globally needs the same unification pass; lower priority
  than machine output.
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
- [ ] The oracle's two "implementations" are in-package functions, not a real
  reference build vs native port. Domain-specific (a mobile-port profile) — see
  the proof-contract note below.

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
