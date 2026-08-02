# Agentic Development Harness (`adh`)

`adh` is a five-stage harness for letting an AI agent **plan, build, review, and
validate its own changes** to a real codebase — with a human on every step that
can't be undone. It is distributed as a single Go binary and as a Claude skill,
[`adh-relay`](./.claude/skills/adh-relay/SKILL.md).

The point of difference is where the reasoning comes from. `adh` does not call an
LLM API. Instead the skill **relays** each stage: `adh` emits a stage's prompt and
suspends, the calling agent (you, Claude) supplies the reasoning, and `adh` takes
the reply and advances. The agent is the worker; `adh` is the harness around it.
That means the whole tool builds, tests, and runs with **no API key and no
hardware** — the model is whoever is driving the skill.

What `adh` owns is everything that should be deterministic or gated: the arc state
machine, the cold-critic context split, `NO-PROOF-NO-CLOSE`, the differential
oracle self-test, the human approval gate, the model-gate, the autonomy ladder,
context routing and its integrity checks, distilled lessons, the harness-artifact
self-optimization ratchet, and effectiveness accounting. Judgment is relayed to the
agent; control stays in code.

______________________________________________________________________

## The Loop

Work moves through five stages, each handing off to the next:

```text
Strategy → Execution → Critic → Evaluation → Ops
```

- **Strategy** plans the change.
- **Execution** builds it, and records the footprint it touched (paths, and the
  area labels those paths route to).
- **Critic** reviews it in *cold context* — a fresh agent with no memory of
  building it — so it catches what the builder rationalized away.
- **Evaluation** adjudicates the critic's findings deterministically: it runs the
  artifact each finding names — a proof manifest, a built-in check, or a repository-
  declared §13 tool (the repo's own oracle/device/invariant target) — and disposes of
  the arc (confirmed → back to Execution; nothing confirmed → advance, with
  unconfirmed findings kept as lesson candidates).
- **Ops** ships it, behind a human gate.

A unit of work is an **arc**. `adh run <id>` drives an arc through the stages until
it reaches a human gate or closes; `adh step <id>` runs one transition at a time.

## Driving It as a Skill

The [`adh-relay`](./.claude/skills/adh-relay/SKILL.md) skill is the relay's `run`
loop. Given an arc id, it repeats: **emit** (`adh step --relay --jsonl <id>`),
**reason** over the emitted prompt, **resume** (`adh step --relay --response <file>
--jsonl <id>`), until the arc reaches the ops gate. Evaluation is not relayed — it
is deterministic (`adh eval <id>` runs the artifact each critic finding names and
disposes of the arc). `adh run --relay` chains the whole thing — emit, resume, inline
eval, next prompt — to cut round-trips. At session start the skill sweeps the standing
maintenance loops once with `adh loop tick`, which fires `context-drift`
(`adh context verify`), `harness-integrity` (`adh doctor`), and `lesson-backlog`, and
opens an arc for each sensed departure — so accretion happens without being asked.
Two invariants hold the design together:

- **The critic runs cold.** When the emitted stage is `critic`, the skill answers
  from a *fresh sub-agent* that has not seen the build conversation — it receives
  only the critic prompt, and replies in strict findings JSON. `adh` already
  withholds the builder's transcript from that prompt; the fresh sub-agent is the
  other half of the guarantee.
- **No self-granted gates.** Approval requires a human typing `adh approve <id>
  --phrase "<phrase>"`, where the phrase is the arc's own id. `--yes`, `--dry-run`,
  and any environment variable are refused. The agent has no code path to approve
  its own irreversible action.

## What It Can Do

The capabilities group along the two levers the practice treats as primary —
**context** and **tools** — plus **proof**, **authority**, and **feedback**.

**Context & tools — make the repository teach the agent.** A context store
(`.adh/context`) holds navigable domain units; each arc pulls a small working set
selected by its labels and target paths, previewed just-in-time and pulled on demand.
`adh context {list,show,route,lint,verify,check,misses,eval,index}` inspects and
guards that routing: `lint` checks structure and provenance (a cited source path must
resolve, and a unit's quoted claim must actually be found in that source); `verify`
proves a routed unit has not drifted from its canonical source; `check` reviews a
routed set for cross-unit contradictions; `misses` reports the routes the accumulated
routing gaps have earned — proposed only once a gap recurs across **≥2 distinct time
strata**, so a one-day burst never mints a rule. Units carry a trust tier (unverified
→ machine-confirmed → human-reviewed) that weights routing. `adh init` scaffolds a
starter store keyed by the repo's top-level directories, so a change under `internal/`
routes to the `internal` unit out of the box. A tool registry
(`adh tool {list,doctor,run}`) makes external capabilities legible, checkable, and
runnable; `adh doctor` self-checks the whole harness (registries valid, unit ids
unique, cross-references and provenance resolve); `adh nfr {list,show,lint}` holds
nonfunctional requirements as testable Planguage specs.

**Proof — every close leaves evidence.**

- **`NO-PROOF-NO-CLOSE`.** An arc cannot close until an automated check confirms
  its proof artifacts exist on disk and match their manifest hashes. The loop
  cannot skip it. `adh proof create <id> <artifact>…` builds the packet; `adh proof
  verify <manifest>` checks it (exit 8 on failure).
- **Differential oracle.** When a domain has no objective grader, two independent
  implementations of the same rules grade each other — any disagreement is a defect
  in one of them. `adh oracle selftest` proves the gate catches a *planted* bug (a
  negative control), so the oracle is trustworthy before it's trusted. The technique
  generalizes to any repository: `adh oracle diff --reference <cmd> --candidate <cmd>`
  runs two commands (a reference build vs an optimized port, an old vs new
  implementation) and confirms divergence in their output — declare it as a §13 tool
  and an oracle finding confirms real divergence, no domain-specific harness required.
- **Provenance.** Proof packets bind each artifact to its content by `sha256[:16]`
  identity, computed when `adh proof create` writes the manifest, so an artifact
  traces back to exactly the bytes it proves.

**Authority — autonomy inside explicit limits.** Capability (how to cause an
effect) is kept separate from authority (which effect an identity may cause). A
**model-gate** refuses to let a judgment role (Strategy, Critic, Evaluation) run on
an under-powered model, and a **worker epoch** (`adh worker {show,requalify}`) forces
re-qualification when the driving model changes. The **autonomy ladder**
(`adh autonomy {show,set}`) names trust in levels **L0–L4**; higher levels remove
*clicks* (auto-launching safe steps), never *gates*. Every irreversible or
outward-facing action stops at a human gate (`adh gate list`, `adh approve`,
`adh reject`).

```text
L0 Manual → L1 Assisted → L2 Hands-off relay → L3 Auto-advance launches → L4 Lights-out
```

**Feedback — judgment compounds.** Every correction is designed to be *accretive*
and to survive more than one signal. Findings that no artifact confirmed become
**lesson** candidates (`adh lesson {list,promote}`); a class promotes to a durable
owner only after it recurs across **≥2 distinct time strata**, and the promoted unit
is tagged with the scope it was learned in and a root-cause triage (routing gap vs.
insufficient guidance). A confirmed **structural** finding — one needing a design
change — escalates to a human instead of burning rework cycles, while a fixable one
loops back to Execution. The cold critic is steered toward the check kinds recent
reviews under-covered, so its blind spots close over time. `adh kpi` proposes a change
to any context unit *or declared tool* whose performance indicator degrades and the
degradation **replicates** across strata — a unit that keeps failing in its scope, a
tool that keeps failing or running slow (its outcome and duration logged from every
path it runs: `adh tool run`, `context verify`, and adjudication) — never
auto-adopted. A periodic **self-evaluation**
(`adh selfeval`) scores health with a delta versus the prior period and a failure
taxonomy that feeds a registry (`adh failures list`); effectiveness accounting
(`adh metrics`) keeps proven outcomes beside their cost. The harness's own guiding
artifact is optimized under a comparative ratchet with a calibrated judge
(`adh harness {eval,gate,hash,calibrate}`). Settled work runs as maintenance **loops**
(`adh loop {list,run,tick,retire}`), and the offline consolidation cycle
(`adh sleep run`) harvests and gates candidate harness improvements against a held-out
split with a replication verdict — and its own planted-regression self-test.

> Scope note: the arc loop wires Strategy, Execution, the cold Critic, deterministic
> Evaluation, the model-gate, effectiveness, and the human ship gate end to end, and
> the standing accretion loops run via `adh loop tick`. Evaluation resolves an
> oracle/invariant/device finding to a repository-declared §13 tool when the repo
> provides one, and `adh oracle diff` is a ready-made command-level differential oracle,
> so a repo supplies only its own two commands or a check script; what remains is the
> device/`adb` hardware path and an optional live/batch model worker for unattended
> self-optimization runs — see
> [`TODO.md`](./TODO.md).

## Command reference

Every command carries `--help`; global flags include `--jsonl` (one machine-readable
outcome envelope per line), `--repo`, `--quiet`, and `--verbose`.

- **Arcs & loop** — `arc {new,list,show}`, `run`, `step`, `eval`, the manual
  single-stage commands `{strategy,execute,critic,ops}`, `status`, `close`.
- **Gates & authority** — `gate list`, `approve`, `reject`, `autonomy {show,set}`,
  `worker {show,requalify}`.
- **Context & tools** — `context {list,show,route,lint,verify,check,misses,eval,index}`,
  `tool {list,doctor,run}`, `nfr {list,show,lint}`, `doctor`, `init`.
- **Proof & oracle** — `proof {create,verify}`, `oracle {diff,invariants,selftest}`,
  `device validate`.
- **Feedback & accretion** — `lesson {list,promote}`, `failures list`, `kpi`,
  `metrics`, `selfeval`, `loop {list,run,tick,retire}`,
  `sleep {run,adopt,status,schedule}`.
- **Harness self-optimization** — `harness {eval,gate,hash,calibrate}`,
  `judge` (score an output against deterministic rule checks).
- **Utility** — `vcs {status,branch,commit}`, `version`, `docs` (generate man pages).

## Quickstart

```sh
go build -o adh .          # build the binary
go test ./...              # unit + dispatcher integration tests
golangci-lint run ./...    # the project's strict linters, unrelaxed

adh init                          # scaffold the .adh workspace + starter context
adh doctor                        # confirm the harness is intact and consistent
adh loop tick                     # sweep the standing accretion loops (opens arcs)
adh arc new "fix the crash"       # create an arc, prints its id (e.g. arc-0001)
adh run arc-0001                  # relay through the stages to a gate or closure
adh oracle selftest               # prove the differential oracle catches a planted bug

# at the ops gate, a human closes it (the agent cannot):
adh proof create arc-0001 path/to/artifact
adh approve arc-0001 --phrase arc-0001
adh close arc-0001 --proof .adh/proof/arc-0001.json
```

To drive an arc as an agent, invoke the skill — e.g. *"run arc-0001"* in Claude
Code — and it walks the emit → reason → resume loop for you, spawning the cold
critic sub-agent and stopping at the human gate.

On **close**, a `change` arc's commit is created on its own branch `adh/<arc-id>`
(branch-per-arc), leaving the base untouched and the change ready to open as a PR.
A **reject** at a gate reverts the arc's working-tree changes to HEAD and returns
it to Execution to be reworked.

## Architecture

`adh` follows a functional core / imperative shell split. Pure decision logic lives
in `internal/` — the gate ratchet, arc state machine, differential oracle, proof
verification, authority gates, effectiveness metrics, lessons — behind thin `cmd/`
shells over [`ff/v4`](https://github.com/peterbourgon/ff). Effectful work (model,
`adb` device, git) sits behind interfaces with deterministic mock backends, so the
whole tool builds and tests without an API key or hardware. Errors carry a domain
code (`adh.Error{Code,Op,Err}`) translated at each boundary. The design is
specified in [`SPEC.md`](./SPEC.md) and [`SPEC-ADDITIONS.md`](./SPEC-ADDITIONS.md)
and was built to the plan in [`PLAN.md`](./PLAN.md).

______________________________________________________________________

## Lineage

`adh` stands on two bodies of work, and its own contribution is the running
implementation that joins them.

### The Architecture Case Study — Erik Hill

The five-stage loop, the cold critic, the differential oracle, the autonomy ladder,
`NO-PROOF-NO-CLOSE`, and the operating record below come from a harness **Erik
Hill** built solo, from first principles, over ~4 months, to let agents ship
changes to a real (pre-launch) Android game with a human on every irreversible
step. That system is private; what follows is the architecture it proved out.

> Case study by **Erik Hill** — agentic systems engineer.
> Portfolio → <https://egnaro9.github.io> · LinkedIn → <https://linkedin.com/in/erik-hill-98895575>

- **Differential oracle** — the game's core logic exists twice, as a reference
  build and a performance-critical native port authoritative on device; two
  independent implementations make each the other's oracle. The technique is
  runnable in [evals-differential-oracle](https://github.com/egnaro9/evals-differential-oracle).
- **Logic invariants** — core rules (e.g. *"a reward fires only from a direct user
  action, never as a downstream side effect"*) enforced as property-based tests over
  the engine that's authoritative on device.
- **On-device validation** — adb-driven tests confirm behavior on real hardware.
- **Operating record (spring–summer 2026)** — from the on-disk archive: ~200
  completed work-arcs · ~190 human-gated checkpoints · 74 independent cold-critic
  reviews · 13 periodic self-evaluations · a ~200-file sanitized proof archive with
  ~90 provenance manifests · a failure registry with per-item root-cause fixes.
- **Verifiable outcomes (all public)** — an arcade game,
  [Tap Dodge Rush](https://play.google.com/store/apps/details?id=com.seraphlight.tapdodgerush),
  shipped to Google Play under SeraphLight Studios; a one-character fix merged
  upstream into TeaVM; a live public model-drift board grading 16 LLMs daily on a
  frozen, deterministically-graded suite.

### The Practice — Ryan Lopopolo's *Harness Engineering*

`adh` is a concrete instantiation of **harness engineering**: the practice of
improving agent output by shaping the environment around a *fixed* worker (a chosen
model and coding agent held constant as a black box) rather than swapping the
worker. You improve the two external levers — **context** and **tools** — and
curate the environment so the worker can recover intent, operate the real system,
respect authority, prove the outcome, and leave the next run better equipped.

The practice is published by **Ryan Lopopolo** as a three-layer corpus:

- an **anthology** (`sources/`) — his essays, talks, interviews, and public-post
  corpus;
- a **field guide** (`docs/`) — twelve synthesized theses (hold the worker
  constant, give one agent the whole job, route context just-in-time, make the
  repository teach the agent, autonomy inside explicit authority, prove the outcome,
  turn feedback into infrastructure, measure effectiveness, and more);
- an **agent context bundle** — the whole retrieval-optimized repository, meant to
  be pointed at a coding agent alongside the system it should improve.

The `adh` additions map directly onto the practice: the context store and tool
registry are the two levers; `NO-PROOF-NO-CLOSE` and the differential oracle are
*prove the outcome*; the human gate, model-gate, and autonomy ladder are *autonomy
inside explicit authority*; lessons, self-eval, effectiveness, and the sleep cycle
are *turn feedback into infrastructure* so judgment compounds. The mapping is drawn
out in [`SPEC-ADDITIONS.md`](./SPEC-ADDITIONS.md) §10–18.

> Ryan Lopopolo, *Harness Engineering*, CC BY 4.0,
> <https://github.com/lopopolo/harness-engineering>.

### The Implementation — This Repository

The `adh` CLI and the `adh-relay` skill, along with `SPEC.md`, `SPEC-ADDITIONS.md`,
and `PLAN.md`, are by **Steven A. Coffman**. The contribution is the relay design —
inverting "the harness calls a model" into "the harness relays to the agent," so
harness engineering's fixed worker becomes whichever agent drives the skill — and a
deterministic, mock-backed Go implementation of the gates, oracle, proof, and
compounding loop that runs with no key and no hardware.
