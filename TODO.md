# TODO — outstanding work on the adh CLI

Tracks what remains after Phases 0–9 (see [`PLAN.md`](./PLAN.md)). Everything
committed passes the phase gate: `golangci-lint run ./...` clean and
`go test ./...` green.

## Disposition

- [ ] Branch `implement/adh-spec` is unpushed; upstream PRs are owner-only.
  Decide whether to push to a fork or hand off.

## Fidelity to the source implementations

Gaps found comparing adh to `SkillOpt` and `evals-differential-oracle`. The pure
decision cores (gate ratchet, score projection, content hash, defect/lapse,
independent invariant checker, gate self-test) are faithful; the orchestration
around them is partial.

- [ ] `sleep` is a bounded mock: no real `harvest → mine → reflect →
  consolidate → stage → adopt`, no train/val split in code, no persisted
  rejected-edit buffer; `sleep adopt` is a stub.
- [ ] Not ported from SkillOpt: `compute_score` (mean hard/soft over rollouts),
  success+failure mode extraction, longitudinal improved/regressed/persistent
  pairs, slow-update/meta-skill, and the LR-scheduler / edit-budget /
  `rank_and_select` machinery.
- [ ] Oracle is one-dimensional (rows only); the original is two-dimensional
  (rows + columns). Disclosed reduction — port the column pass for full parity.
- Deliberately omitted: `select_gate_score`'s optional semantic-density bonus (a
  gameable proxy). Not a defect.

## Command surface (PLAN Phase 9 follow-ups)

- [ ] Per-stage direct commands `strategy`/`execute`/`critic`/`eval`/`ops`
  (currently only via `step`/`run`).
- [ ] `gate list` (pending human gates), `registry audit`, `failures list`,
  `selfeval` as their own commands.
- [ ] Real ff subcommand nesting (today `arc new`, `sleep run`, etc. use
  positional-verb dispatch); regroup the ratchet under `harness gate`/`hash`.
- [ ] Bind the global flags declared in `PLAN.md` on `root.Config`: `--config`,
  `--profile`, `--repo`, `--json`, `--quiet`, `--no-color`, `--yes`,
  `--dry-run`. Today `--json` exists only on `gate`.

## Config wiring

- [ ] No `.adh/config.toml` precedence loader (flags > env `ADH_*` > repo config
  > defaults), the Phase 0 item that was deferred. Consequences:
  - [ ] `run` hard-codes autonomy `L2` instead of reading `.adh/autonomy`.
  - [ ] Per-role model classes are not config-driven (the model-gate has no real
    routing to enforce).
  - [ ] The approval phrase is the arc ID (placeholder); make it configurable.
  - [ ] `worker requalify`'s baseline models are a placeholder map.

## Effectful seams still mocked

- [ ] `model.Client`: only `Mock`; no real LLM client.
- [ ] `device.Validator`: only `Mock`; no adb adapter.
- [ ] No `VCS`/git adapter — `ops`/close do not branch, commit, or merge.
- [ ] No injected `Clock` (deterministic timestamps) once state grows time
  fields.
- [ ] The oracle's two "implementations" are in-package functions, not a real
  reference build vs native port.

## End-to-end lifecycle wiring

The tested cores are not yet composed into a working loop.

- [ ] Strategy never sets an arc's `Resolution` (§12); set it so `CanClose` has
  something to check.
- [ ] `CanClose`/NO-PROOF-NO-CLOSE is implemented and tested but not invoked at
  `ops`/close — arcs currently close without a proof-verify gate.
- [ ] Add `arc close --as <resolution>` enforcing the resolution-matched proof
  contract.
- [ ] Nothing sets an arc to `StatusBlocked`, so the `approve`/`reject`
  human-gate loop is never exercised end to end.
- [ ] The cores `harness`, `lesson`, `context`, `tool`, `loop`, `worker`,
  `metrics` are standalone commands/logic — not yet called by the stage or
  sleep loop.

## Housekeeping

- [ ] No CI workflow to run `golangci-lint` + `go test` on push.
- [ ] `PLAN.md`, `SPEC-ADDITIONS.md`, `README.md`, and this file have not been
  run through the repo's `mdformat`/`rumdl`/prettier config (outside the
  golangci gate; may drift).
- [ ] No per-command docs or man pages.
