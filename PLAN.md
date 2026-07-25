# PLAN — implementing adh from SPEC.md + SPEC-ADDITIONS.md

This plan turns the existing climax scaffold (`main.go`, `cmd/{cmd,root,version}`,
ff/v4) into the `adh` CLI specified by [`SPEC.md`](./SPEC.md) (§1–9: arcs, the
five stages, gates, oracle, proof, autonomy ladder) and
[`SPEC-ADDITIONS.md`](./SPEC-ADDITIONS.md) (§10–18: context, lessons, resolution
types, tool registry, worker requalification, maintenance loops, effectiveness,
credentials, harness self-optimization).

It follows the local climax convention (as in `skillsaw`) and the guidelines in
`go-advice/summary_rules.md`. A "Revisions applied from go-advice" section
records how the advice shaped the plan.

## 1. Reality and scope

adh orchestrates agents, an oracle, a device, and git. Only part of that is
deterministic and CLI-ownable; the rest needs a model, `adb`, or the network.
The plan front-loads the deterministic core — which matches the SPEC's own
thesis ("do everything measurable in code; reserve the model for irreducible
judgment") and keeps every early phase fully testable without an API key.

Two tiers:

- **Deterministic core** — gate ratchet, content identity/hash, proof
  verification, rubric + diagnose, arc state machine, config precedence, exit
  codes, authority decisions. Pure functions; unit-tested with real values.
- **Effectful seams** — model stages, oracle, `adb` device validation, git,
  the sleep cycle. Expressed as interfaces with a deterministic **mock** backend
  (mirroring SkillOpt-Sleep's `mock` that runs the whole cycle with no API), so
  behavior is testable and the real backend is swapped in at `main`.

## 2. Architecture (functional core / imperative shell)

adh is a pure CLI tool, so `main.go` stays at the repo root and `cmd/` holds
command sub-packages (go-advice §1 "pure CLI tool" branch; §18 Pattern B).

- **`internal/adh` — the foundational domain package (the shared language).**
  Holds the `Error` type + codes (§3), the core domain types (`Arc`,
  `Resolution`, `Stage`, `Score`, config structs), and the effectful interfaces
  (`ModelClient`, `Oracle`, `Device`, `VCS`, `Clock`). It imports no sibling
  package; every other package imports it. This gives Ben Johnson's "root
  package is the shared language" while keeping `main.go` at the root, and it
  prevents sibling cross-imports (§1 ban).
- **`internal/<concern>` — pure cores**, each value-in/value-out and unit-tested:
  `gate`, `identity`, `proof`, `rubric`, `state`, `config`, `authority`,
  `context`, `lesson`, `tool`, `worker`, `loop`, `metrics`, `harness`, `sleep`.
- **`internal/<dependency>` — adapters named after what they wrap** (§1):
  `git`, `model` (LLM interface + `model/mock`), `device` (adb + `device/mock`),
  `oracle`. These are the only packages with I/O beyond the command shell.
- **`cmd/<name>/` — thin imperative shells** (ff/v4): bind flags in `New`, read
  values in `exec`, call a pure core, write to `cfg.Stdout`, return
  `root.ExitError(n)` for controlled exits. No business logic.

Every `internal/<concern>` depends only on `internal/adh` (and stdlib), never on
a sibling concern. When two concerns need to compose (e.g. `harness` uses
`gate`), the composition happens in the command shell or `harness` imports
`gate` as a leaf — we keep the import graph a DAG rooted at `internal/adh` and
document any concern→concern edge in the package doc comment.

## 3. Command surface (target)

Grouped as in the SPECs; each is a `cmd/<name>/` package registered in
`cmd/cmd.go`. Group-parent commands (`arc`, `context`, `tool`, `worker`, `loop`,
`harness`, `sleep`) set `Exec: nil`.

| Group | Commands |
|-------|----------|
| Lifecycle | `init`, `arc {new,list,show}`, `run`, `step`, `status` |
| Stages | `strategy`, `execute`, `critic`, `eval`, `ops` |
| Gates | `gate list`, `approve`, `reject` |
| Oracle/validation | `oracle diff`, `invariants`, `device validate` |
| Evidence/audit | `proof verify`, `registry audit`, `selfeval`, `failures list` |
| Autonomy | `autonomy {show,set}` |
| Additions §10–17 | `context {list,show,route,lint}`, `lesson {list,show,promote,gc}`, `tool {list,show,run,doctor}`, `worker {show,requalify}`, `loop {list,run,retire}`, `metrics` |
| Self-optimization §18 | `harness {eval,gate,hash}`, `sleep {run,adopt,status,schedule}` |

Global flags on `root.Config` (§SPEC 2, bound once): `--config`, `--profile`,
`--repo`, `-v/--verbose`, `-q/--quiet`, `--json`, `--no-color`, `--yes`,
`--dry-run`.

## 4. Phased plan

Each phase is a step. **Do not proceed to the next phase until**
`golangci-lint run --fix ./...` is clean without relaxing any rule, `go test
./...` passes, and the phase's checklist items (§6) are satisfied. Pause after
each phase, re-read the relevant go-advice section, and improve before moving on.

- **Phase 0 — Foundations.** `internal/adh`: `error.go` (Error type + five
  codes + `ErrorCode`/`ErrorMessage`), `domain.go` (Arc, Resolution, Stage,
  Score), `interfaces.go` (ModelClient, Oracle, Device, VCS, Clock with godoc
  naming error codes). `internal/config` precedence (flags > env `ADH_*` >
  `.adh/config.toml` > defaults). Exit-code constants (§SPEC 7 + additions
  9,10,12,13,14,15). Tests: Error wrap/unwrap, config precedence.
- **Phase 1 — Deterministic decisions.** `internal/gate` (comparative ratchet,
  strict `>`, `SelectScore` hard/soft/mixed — port of SkillOpt `evaluate_gate`),
  `internal/identity` (`sha256[:16]` content hash + no-op guard),
  `internal/proof` (NO-PROOF-NO-CLOSE: declared artifacts exist + manifest hash
  match, §SPEC 5.4), `internal/rubric` (deterministic floor + `NEEDS-JUDGE`
  dims + diagnose). Shells: `gate`, `harness hash`, `proof verify`, `selfeval`
  skeleton. Tests for every pure function.
- **Phase 2 — State & lifecycle.** `internal/state`: arc state machine
  (`created→strategy→…→ops→closed`, reject/fail edges) and **resolution types**
  (`change|investigation|experiment|decision`, §ADD 12) with a resolution-matched
  proof contract; `state.json` read/write. Shells: `init`, `arc {new,list,show}`,
  `run`, `step`, `status`. Close gate enforces NO-PROOF-NO-CLOSE.
- **Phase 3 — Authority.** `internal/authority`: capability vs authority
  contract, the human safety gate (approval phrase, no self-grant; `--yes`/env/
  `--dry-run` never satisfy it, §SPEC 5.2), the model-gate (§SPEC 5.1), the
  autonomy ladder L0–L4 (§SPEC 6), and the credential-broker interface (§ADD 17).
  Shells: `approve`, `reject`, `gate list`, `autonomy {show,set}`.
- **Phase 4 — Proof engines (seams).** `internal/oracle` (differential oracle +
  planted-bug negative-control self-test), `invariants`, `internal/device` (adb
  interface + mock). Deterministic comparison/logic real; execution behind
  interfaces. Shells: `oracle diff`, `invariants`, `device validate`,
  `proof verify` (full), `registry audit`.
- **Phase 5 — Context & tools.** `internal/context` (store, routing by
  labels/paths, `lint`, §ADD 10), `internal/tool` (registry, `doctor`, §ADD 13).
  Shells: `context {list,show,route,lint}`, `tool {list,show,run,doctor}`.
- **Phase 6 — Stages (model seam).** `internal/model` (ModelClient interface +
  `model/mock`), stage orchestration `internal/stage` (strategy, execute,
  **cold-context critic**, eval, ops), the `run` relay honoring the autonomy
  level and stopping at gates. Shells: `strategy`, `execute`, `critic`, `eval`,
  `ops`.
- **Phase 7 — Feedback & self-optimization.** `internal/lesson` (candidate
  distillation, promote-to-owner with the executable-owner approval gate, gc,
  §ADD 11), `internal/metrics` (effectiveness accounting, §ADD 16),
  `internal/harness` (eval/gate/hash ratchet on a held-out split, §ADD 18.2–18.3
  with defect-vs-lapse and rejected-edit memory), `internal/sleep` (cycle:
  harvest→mine→gate→stage→adopt with the mock backend, evidence log, and the
  negative-control gate self-test, §ADD 18.4–18.6). Shells: `lesson *`,
  `metrics`, `harness *`, `sleep *`, `failures list`.
- **Phase 8 — Maintenance & worker.** `internal/loop` (maintenance loops with
  retirement, §ADD 15), `internal/worker` (requalification epoch + gate, §ADD 14).
  Shells: `loop {list,run,retire}`, `worker {show,requalify}`.
- **Phase 9 — Integration.** Wire the full dispatcher, `--json` on every command,
  end-to-end tests through `cmd.Run`, and update `README.md`.

## 5. Effectful seams — interface contracts

Defined in `internal/adh/interfaces.go`, each with a `mock` implementation:

- `ModelClient` — `Complete(ctx, req) (Response, error)`; the mock returns fixed,
  seeded output so stage/critic/sleep tests are deterministic.
- `Oracle` — `Diff(ctx, board) (Divergence, error)`; the differential compare is
  pure, the two implementations are the seam.
- `Device` — `Validate(ctx) (Report, error)`; the mock models a healthy/failing
  device without `adb`.
- `VCS` — branch/commit/revert/hash; the mock records calls for assertion.
- `Clock` — `Now()`; injected so timestamps and epochs are deterministic
  (go-advice §5: never call `time.Now()` inside logic).

## 6. Cross-cutting invariants (checklist, gates each phase)

Applied from go-advice §19 (Architecture, Domain, Errors, Testing, CLI):

- `main` wires only; no business logic in `main` or `cmd/cmd.go` beyond dispatch.
- One `Error` type in `internal/adh`; leaf = Code+Message, wrapper = Op+Err,
  never both; external errors translated at the boundary; `ErrorCode`/
  `ErrorMessage` at call sites.
- Pure cores accept values and return values; no `time.Now`, fs, net, or model
  call inside a function that also holds logic — the shell passes results in.
- ff/v4 Pattern B: flags bound in `New`, read in `exec`; `SetParent` on every
  subcommand; write to `cfg.Stdout`/`cfg.Stderr`; `root.ExitError(n)` for exits;
  `Exec: nil` on group parents; never `os.Exit`/`os.Getenv` in a command;
  preserve the `climax:` markers.
- Table-driven tests, each case named under `t.Run`; hand-written mocks; pure
  functions tested with real values; no `init()`, no global state (only
  `version.Version`).
- Error strings lowercase, no trailing punctuation, `"<command>: <reason>"`.
- One concept per file; ≤1000 SLOC; interface comment written before the body.

## 7. Revisions applied from go-advice

The first draft placed domain types in whatever `internal` package used them and
let siblings import each other. The advice changed the plan as follows:

1. **§1 layout / no sibling cross-imports** → introduced the single
   `internal/adh` foundational domain package; the import graph is a DAG rooted
   there. Adapters are named after the dependency they wrap (`git`, `model`,
   `device`), not by role.
2. **§3 error handling** → one `Error` type with Op/Err wrapping and five codes
   up front in Phase 0, instead of ad-hoc `fmt.Errorf` per package; external
   errors (fs, git, adb, model) translated at each adapter boundary.
3. **§5 functional core / imperative shell** → every decision (gate, resolution,
   proof check, rubric, diagnose, ladder) is a pure function; `Clock`/`VCS`/
   `ModelClient`/`Device` are injected so no logic function touches wall-clock or
   I/O. The two-commit "extract then swap" rhythm is the per-feature cadence.
4. **§4 interface design** → interfaces kept minimal and defined at point of use;
   the mock backends implement only the methods a test needs; the SPEC's rubric
   "deterministic floor + judge dims" is the interface-hiding boundary.
5. **§18 Pattern B + §19 CLI checklist** → the cadence gate for each phase is a
   clean `golangci-lint run --fix ./...` (no rule relaxation) plus the CLI
   checklist; group parents use `Exec: nil`; `climax:` markers preserved.
6. **§15 design philosophy** → deterministic core before effectful seams; delete
   shell scaffolding once a pure core replaces it (the large-deletion commit).

## 8. Cadence

Per phase: implement one package + its shell, run `golangci-lint run --fix
./...` and `go test ./...`, re-read the governing go-advice section, fix without
relaxing rules, commit, then pause before the next phase. Scaffold new command
packages with `climax add` when available so the `climax:` markers and Pattern B
wiring stay consistent; otherwise mirror `skillsaw/cmd/gate` exactly.
