# TODO — outstanding work on the adh CLI

Tracks what remains after Phases 0–9 (see [`PLAN.md`](./PLAN.md)). Everything
committed passes the phase gate: `golangci-lint run ./...` clean and
`go test ./...` green.

## Ported from skillsaw (done)

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

## Fidelity to the source implementations

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

## Offload to a mature library (undifferentiated heavy lifting)

Necessary but edge-case-heavy plumbing that is not adh's differentiated value.
The effectful interfaces (`model.Client`, `device.Validator`, planned `VCS`,
`Clock`) are the slot points: keep the mock for tests, wrap the library in an
`internal/<dependency>` adapter. Keep the policy cores (gate, oracle+invariant,
defect/lapse, autonomy ladder, NO-PROOF-NO-CLOSE, effectiveness) hand-rolled.

- [x] State writes are now atomic (temp file + rename). Still to offload:
      `google/renameio` (fsync durability) and `gofrs/flock` (cross-process
      locking) once the parallel manager writes one workspace from many arcs.
- [ ] `model.Client` (LLM): retries/backoff, streaming, tool-calls, token
      counting → the official SDKs (`anthropic-sdk-go`, `openai-go`). Never a
      hand-rolled HTTP/retry layer.
- [ ] `VCS` (git branch/commit/merge/revert, protected path) → shell out to the
      `git` binary for mutations; `go-git` for read-only inspection.
- [ ] `device.Validator` (adb) → the `adb` CLI or `electricbubble/gadb`.
- [ ] `.adh/config.toml` + precedence → ff/v4's own config providers
      (`fftoml`/`ffyaml`), or `knadh/koanf`. Avoid Viper (package globals fight
      go-advice §1/§3).
- [ ] Structured logging (§14) → stdlib `log/slog`.
- [ ] Secret redaction in `sleep` evidence (§18.4-6) → a gitleaks-style ruleset,
      not hand-grown regexes.
- [ ] Scheduling (`sleep schedule`, loops §15) → system crontab, or
      `robfig/cron` in-process.

Keep hand-rolled (not offload candidates): the typed manifest/registry decoders
(the "parse at the boundary" idiom), JSON via `encoding/json`, hashing via
`crypto/sha256`, and CLI/flags via ff/v4 + climax (already offloaded).

## Housekeeping

- [ ] No CI workflow to run `golangci-lint` + `go test` on push.
- [ ] `PLAN.md`, `SPEC-ADDITIONS.md`, `README.md`, and this file have not been
  run through the repo's `mdformat`/`rumdl`/prettier config (outside the
  golangci gate; may drift).
- [ ] No per-command docs or man pages.
