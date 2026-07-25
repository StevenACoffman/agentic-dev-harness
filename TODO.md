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
  - [ ] Deferred: the `--profile` layer (SPEC §3 tier 3) and the global
    `--config`/`--profile`/`--repo` flags (tracked under Command surface); no
    `adh init` writing a starter `.adh/config.toml`; secrets stay env-only.

## Effectful seams still mocked

- [ ] `model.Client`: only `Mock`; no real LLM client.
- [ ] `device.Validator`: only `Mock`; no adb adapter.
- [ ] No `VCS`/git adapter — `ops`/close do not branch, commit, or merge.
- [ ] No injected `Clock` (deterministic timestamps) once state grows time
  fields.
- [ ] The oracle's two "implementations" are in-package functions, not a real
  reference build vs native port.

## End-to-end lifecycle wiring

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
- [ ] Still standalone (not force-fit into the single-arc ship path):
  `lesson`, `context`, `tool`, `loop`, `worker`. `harness`/consolidate is wired
  through `sleep`. A real evaluation stage that fails an arc back to execution
  is modeled (`StatusFailed`) but the mock never triggers it.

## Offload to a mature library (undifferentiated heavy lifting)

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
