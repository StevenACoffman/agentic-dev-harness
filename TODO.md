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

- [ ] **Loop A — context units carry content + provenance (§10.4).** The primary
      lever is half-wired: `contextstore.Unit` is `{id,kind,labels,paths,owner}`,
      `Load` reads only metadata, and the grounding renders `- <id> (<kind>)` — the
      rule/skill *text* is never delivered or pointed at. Add a content route
      (`Path`) + `Provenance` (mirror modelith's origin/ref/commit/**digest** model,
      not a bare string); `context show <id>` returns text + provenance; the
      grounding previews and the worker pulls the slice JIT. An exegesis skill pack
      or a modelith-rendered `*.modelith.md` then drops straight in.
- [ ] **Loop B — register exegesis/skillsaw/modelith as §13 tools.** External
      `Run` commands with `--json` where available (`modelith lint --format json`,
      `modelith render --check`, `skillsaw eval --json`, `exegesis verify`), so the
      worker invokes them via `adh tool run <id>` and interprets output in-loop.
- [ ] **Loop D — lesson promotion materializes the durable owner (§11.2).**
      `lesson promote` gates + prints "promoted" but creates nothing. Make
      `--to context|skill|check` write the artifact (a routable §10 unit with
      provenance; a skill via exegesis; a §13 tool entry) behind the §11.2 approval
      gate — so a correction becomes accretive, not a printed intent.
- [ ] **Loop E — cross-unit consistency (§10.4).** `context lint` checks id/kind
      only. Split: deterministic reference-integrity is owned by `modelith lint`
      (reciprocal-cardinality, mutual-ownership, duplicate ids, cycles, dangling
      refs); irreducibly-semantic conflict (skill vs base rule vs domain invariant)
      is adjudicated by the cold critic via `context lint`/`check`, gated.
- [ ] **Loop F — context-integrity / anti-drift gate (§10.4, proof).** Nothing
      today guarantees a routed unit still matches its source. Register
      `modelith render --check` (and `modelith lint`, `exegesis verify`) as §13
      NFR-checks an arc runs over its routed context, so a stage fails when the
      routed Markdown drifted from its canonical YAML/source. A capability adh
      lacked entirely.
- [ ] **Loop C — skillsaw + relay improvement loop (§18).** Replace the mock
      `consolidate.Propose`: drive skillsaw `eval → diagnose → gate` with the relay
      supplying the `needs_judge` scores and one-dimension edits, so Claude drives
      real, gated skill improvement through adh. skillsaw's strict-`>` `gate` feeds
      Evaluation but never replaces human approval or NO-PROOF-NO-CLOSE; the loop
      stages (auto_adopt = false). Do last — it composes B and the gate.

### From `agentic-harness-bootstrap` (Data Formats / Processes Worth Folding In)

The bootstrap system (MIT, prose+templates: turn a repo into something agents can
understand/verify) contributes three formats/processes that fill gaps the toolchain
above does not. Its other pieces (stack-specific lint/hook/CI templates, the
one-time discover→generate playbooks, the generated CLAUDE/AGENTS files) overlap
what adh or the driving skill already own — not adopted.

- [ ] **ADR decision format for NFR trade-offs (§10.2, §11, §12).** The bootstrap
      ADR — *Status / Context / Decision / Consequences split into **Easier** and
      **Harder*** — is the durable, routable home for a team's local NFR
      prioritize/trade-off decisions, the one thing modelith deliberately omits. Add
      a `decision` context-unit kind (routes past ADRs so a later arc inherits, not
      re-litigates), a `lesson promote --to decision` owner, and make an ADR the
      proof artifact of a `decision`-resolution arc (§12). **Highest-value add for
      the NFR-governance goal.**
- [ ] **Harness-integrity self-verification (§10.4).** Adopt the `verify-harness.sh`
      pattern as an `adh` self-check (a §13 check + a session-start/CI gate): every
      routed unit resolves to a real artifact, every named tool exists, agent-facing
      guidance references only real modules/commands, no dangling references, pieces
      don't contradict. Broader than Loop F's content-drift check — "is the whole
      harness intact and consistent?" Cheap, high-trust.
- [ ] **Standing-order accretion triggers as §15 loops.** Encode the bootstrap
      "Harness Evolution" maintenance triggers as adh loops/hooks so accretion is
      automatic, not prompted: agent mistake → `lesson promote` (Loop D);
      architectural/NFR decision → ADR (the format above); session-start/pre-run →
      harness-integrity + context-drift check (Loop F). Makes "every correction
      accretive" a standing behavior, not a manual step.

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

- [ ] **Adopt an NFR taxonomy + Planguage as the NFR-check spec format (§10.5).**
      An NFR-check unit stops being free prose: name it by an agreed taxonomy
      (**ISO/IEC 25010** or **FURPS+**) and quantify it in **Planguage** — `Tag`
      (`Performance.Latency`), `Scale`, `Meter`, `Baseline`, `Fail`, `Goal`,
      `Stretch`, `Ambition`. This binds the four things adh scatters into one unit:
      *category* (taxonomy), *check* (`Meter` → §13 tool → an SLI), *gate* (`Fail` →
      the Evaluation threshold and proof-contract acceptance bar, §SPEC 3.1), and
      *rationale* (`Ambition` → a `decision`/ADR, §12). Turns "should be fast" into a
      testable, gateable requirement — **the NFR schema that was missing.** Pairs
      with the ADR decision format above: Planguage specifies the requirement, the
      ADR records the trade-off decision about it.
- [ ] Terminology alignment (§10.5): frame the cross-unit consistency check (Loop E)
      as **validation** (are the requirements right and conflict-free) and proof/the
      `Meter`-driven Evaluation gate as **verification** (is it built right) — the
      standard NFR vocabulary, so adh's stages name what they already do. Optional:
      NFR **allocation** (split a top-level `Fail` into per-component budgets by
      routing derived units to component labels/paths).

### From `cc-thinking-skills/evals` (Replication-Gated Outcome Eval — the "Not an RNG" Backbone)

Its `evals/` is a replication-gated, condition-blind eval pipeline (JS/MIT) with a
pure `verdict()` core, honest negative results (it publishes that *zero* skills yet
ELEVATE), a validated judge, split-leakage checks, and real statistics (McNemar,
effect-size threshold, cluster bootstrap, Holm). It is a **reference methodology**,
not something to vendor (JS; adh implements the `verdict()`/stats in Go or shells
out). It directly answers the *accretive-not-RNG* goal and shows adh's/skillsaw's
single strict-`>` gate is insufficient.

- [ ] **Replication-gated outcome-eval verdict for the adoption gate (§18.2, §16,
      Loop C).** A strict-`>` on a rubric is a *structural* screen, necessary but
      not sufficient — one comparison can't separate signal from noise. Gate
      adoption on an **outcome** eval (condition-blind, paired control=no-change vs
      treatment=change, token-budget-balanced) over the held-out split, with an
      effect-size threshold + a paired significance test, and a **verdict taxonomy**:
      `ELEVATE` only on a primary pass **and** an independent fresh replication;
      `DIRECTIONAL-NOT-REPLICATED`/`REPLICATION-MISSING` otherwise (refuse to
      elevate); record `KILL`/not-replicated honestly (§18.6). The `verdict()` is a
      pure, unit-testable core (adh's FCIS). skillsaw's `gate` is the cheap floor
      **under** this bar, not a substitute. **The trust backbone for every accretion
      loop above.**
- [ ] **Validate the graders and the splits (§18.2).** Calibrate the judge before
      trusting its verdicts (extends the §18 negative-control self-test to the judge
      itself), and check selection/test splits for leakage (a `validate-splits`
      equivalent) — a weakenable grader or a leaky split makes the ratchet a proxy.
- [ ] **Routing eval for the context lever (§10, Loop A/E).** Adopt the pipeline's
      routing eval: does `context route` fire the *right* units for an arc's
      labels/paths (and correctly return NONE when nothing applies)? An outcome eval
      for routing quality, so the context lever is measured, not assumed.

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

- [ ] Branch `implement/adh-spec` is unpushed; upstream PRs are owner-only.
  Decide whether to push to a fork or hand off.

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
  the precondition is unmet — no `time.Time` on `adh.Arc`/state, and the only
  `time.Now()` sites are commit-authorship (go-git, never pinned by a test) and
  `sleep.stamp`, which already isolates time to a one-line shell so the consolidate
  core stays pure. A `Clock` seam today would be a speculative abstraction with no
  consumer. Revisit when a state time field or a test needs deterministic time.
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
- [ ] §19.2 remaining adjudication depth: `oracle`/`invariant` findings run the
  in-package React/Native equivalence + invariant checks, which pass (the pair is
  the correct oracle), so a per-arc confirmed path waits on a **real oracle
  target**; `device` findings run `device.Mock{Healthy:true}`, pending an **adb
  adapter**. Both are domain-specific (mobile port) and deferred.
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
