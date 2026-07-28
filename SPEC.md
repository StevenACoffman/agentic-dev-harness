# Agentic Development Harness — CLI Specification

A command-line application that runs the five-stage agent loop described in the
[README](./README.md): agents **plan, build, review, and validate** their own
changes to a real codebase, with a human on every irreversible step.

This document specifies the CLI surface, configuration, state model, and the
gates that make the loop trustworthy. It is a specification, not an
implementation guide — it defines *what* the tool does and *how it behaves*, not
how any given stage is implemented internally.

The lineage of these ideas — Erik Hill's architecture case study and Ryan
Lopopolo's *Harness Engineering* practice — is set out in the
[README](./README.md#lineage). [`SPEC-ADDITIONS.md`](./SPEC-ADDITIONS.md) §10–18
extends this base with the practice's context, tool, feedback, and effectiveness
levers.

______________________________________________________________________

## 1. Overview

The harness is invoked as a single binary, `adh` (Agentic Development Harness).
It drives units of work called **arcs** through five stages:

```text
Strategy → Execution → Critic → Evaluation → Ops
```

Each stage hands off to the next automatically via hooks, but every
**irreversible or outward-facing** action (device deploy, git commit/push,
release) stops at a **human gate** that requires an explicit approval phrase.

### Design Principles (From the README)

1. **NO-PROOF-NO-CLOSE** — an arc cannot close until an automated check confirms
   its proof artifacts exist on disk.
2. **Cold critic** — review runs on a fresh model with no memory of building the
   change.
3. **Gates sit on irreversibility, not competence** — higher autonomy removes
   *clicks*, never *gates*.
4. **The agent can't self-grant** — approval always comes from a human operator.
5. **Model-gate** — judgment roles refuse to run on an under-powered model.

______________________________________________________________________

## 2. Command Surface

```text
adh <command> [subcommand] [flags]
```

Global flags (available on every command):

| Flag               | Description                                                               |
| ------------------ | ------------------------------------------------------------------------- |
| `--config <path>`  | Override config file location.                                            |
| `--profile <name>` | Select a named config profile (e.g. `dev`, `ci`).                         |
| `--repo <path>`    | Target repository root (default: git repo containing cwd).                |
| `-v, --verbose`    | Increase log verbosity (repeatable: `-vv`, `-vvv`).                       |
| `-q, --quiet`      | Suppress non-error output.                                                |
| `--json`           | Emit machine-readable JSON on stdout instead of formatted text.           |
| `--no-color`       | Disable ANSI color.                                                       |
| `--yes`            | Pre-answer *non-gate* confirmations. Never satisfies a human safety gate. |
| `--dry-run`        | Plan and print actions without mutating state, repos, or devices.         |
| `--version`        | Print version and exit.                                                   |
| `-h, --help`       | Print help for the command and exit.                                      |

### 2.1 Lifecycle Commands

| Command               | Purpose                                                                  |
| --------------------- | ------------------------------------------------------------------------ |
| `adh init`            | Scaffold `.adh/` config, state store, and artifact registry in the repo. |
| `adh arc new <title>` | Create a new work arc and print its ID.                                  |
| `adh arc list`        | List arcs with current stage and status.                                 |
| `adh arc show <id>`   | Show an arc's full detail: stage history, artifacts, approvals.          |
| `adh run [<id>]`      | Advance an arc through the loop until it hits a gate or completes.       |
| `adh step <id>`       | Run exactly one stage transition, then stop.                             |
| `adh status`          | Show the harness state: active arcs, pending gates, autonomy level.      |

### 2.2 Stage Commands

Normal operation uses `adh run`, which drives the stages via the relay (and hooks),
or `adh step` to run one transition at a time. Individual stages can also be invoked
directly for debugging or manual re-runs.

| Command                   | Stage                                                                               | Default model class      |
| ------------------------- | ----------------------------------------------------------------------------------- | ------------------------ |
| `adh stage strategy <id>` | Plan the change (manual single stage).                                              | Reasoning (strong, cold) |
| `adh stage execute <id>`  | Build it (manual single stage).                                                     | Fast                     |
| `adh step <id>`           | Run the arc's current stage, then stop. Under `--relay` the cold critic runs here.  | per stage                |
| `adh eval <id>`           | Adjudicate the critic's findings and dispose of the arc (deterministic — no model). | —                        |
| `adh ops <id>`            | Report the ship gate for an arc at ops.                                             | Fast                     |

### 2.3 Gate & Approval Commands

| Command                                         | Purpose                                                                                  |
| ----------------------------------------------- | ---------------------------------------------------------------------------------------- |
| `adh gate list`                                 | List pending human gates across all arcs.                                                |
| `adh approve <id> --phrase "<approval-phrase>"` | Approve a pending gate.                                                                  |
| `adh reject <id> [--reason "<text>"]`           | Reject a pending gate; revert the arc's working-tree changes and return it to Execution. |

### 2.4 Oracle & Validation Commands

| Command                 | Purpose                                                               |
| ----------------------- | --------------------------------------------------------------------- |
| `adh oracle diff`       | Run the differential oracle: compare reference build vs. native port. |
| `adh oracle invariants` | Run property-based invariant tests over the native engine.            |
| `adh oracle selftest`   | Prove the oracle gate catches a planted bug (negative control).       |
| `adh device validate`   | Run adb-driven on-device tests; capture sanitized screenshots.        |

### 2.5 Evidence & Audit Commands

| Command                           | Purpose                                                                                   |
| --------------------------------- | ----------------------------------------------------------------------------------------- |
| `adh proof create <id> <path>...` | Build an arc's proof packet: hash the artifacts into a manifest and record it on the arc. |
| `adh proof verify <manifest>`     | Confirm proof artifacts exist and match the manifest (exit 8 on failure).                 |
| `adh selfeval`                    | Run periodic self-evaluation: score health, delta vs. prior, failure taxonomy.            |
| `adh failures list`               | Show the failure registry with root-cause status and lesson candidates.                   |
| `adh metrics`                     | Report effectiveness accounting (proven outcomes beside their cost).                      |

### 2.6 Autonomy Commands

| Command                     | Purpose                                                     |
| --------------------------- | ----------------------------------------------------------- |
| `adh autonomy show`         | Print current autonomy level (L0–L4).                       |
| `adh autonomy set <L0..L4>` | Set autonomy level. Raising a level is itself a human gate. |

______________________________________________________________________

## 3. Configuration

Configuration follows a strict precedence hierarchy (highest wins):

1. Command-line flags
2. Environment variables (`ADH_*`)
3. Profile-specific config (`--profile`)
4. Repo config: `.adh/config.toml`
5. User config: `$XDG_CONFIG_HOME/adh/config.toml`
6. Built-in defaults

### 3.1 Example `.adh/config.toml`

```toml
autonomy = "L2"                 # current autonomy level (L0–L4)

[models]
# The model-gate enforces that "judgment" roles run on a "reasoning" class.
reasoning = "strong-cold-model" # Strategy, Critic, Evaluation
fast      = "fast-model"        # Execution, Ops

[models.gate]
# Roles that MUST run on a reasoning-class model. The harness refuses to
# start one of these on a fast/under-powered model.
judgment_roles = ["strategy", "critic", "eval"]

[gates]
# Irreversible actions require an explicit approval phrase.
approval_phrase_required = true
irreversible = ["device_deploy", "git_commit", "git_push", "release"]

[oracle]
reference_cmd = "make ref-build"
native_cmd    = "make native-build"
tolerance     = 0             # exact-match; any divergence fails the gate

[proof]
archive_dir       = ".adh/artifacts"
require_manifest  = true       # NO-PROOF-NO-CLOSE
redaction_method  = "blackout" # domain-specific: sanitize sensitive screen regions (screenshot artifacts only)

# Proof contract: the acceptance bar each resolution must satisfy to close. The
# harness enforces that *a* matching proof exists (NO-PROOF-NO-CLOSE); the *content*
# of the bar is the deployment's to define. These generic defaults suit any repo;
# a domain overrides them — a mobile port might set
# change = "oracle, invariant, and on-device proof".
[proof.contract]
change        = "the change's tests pass and its review/CI checks are green"
investigation = "the sources inspected and the reproducible finding"
experiment    = "the instrumentation and the readout that answers the product question"
decision      = "the evidence and the rationale behind the call"
```

### 3.2 Environment Variables

| Variable              | Meaning                                                                                                                                |
| --------------------- | -------------------------------------------------------------------------------------------------------------------------------------- |
| `ADH_CONFIG`          | Config file path (same as `--config`).                                                                                                 |
| `ADH_PROFILE`         | Active profile.                                                                                                                        |
| `ADH_AUTONOMY`        | Override autonomy level.                                                                                                               |
| `ADH_APPROVAL_PHRASE` | **Not honored for gates.** The agent cannot self-grant; gate approval must be typed interactively or passed via `--phrase` by a human. |

Secrets (model API keys, signing keys) are read from the environment or a
referenced secret store — never written to config, state, or logs. Log output
redacts anything matching known secret patterns.

______________________________________________________________________

## 4. State Model

State lives under `.adh/` in the target repo:

```text
.adh/
  config.toml            # committed
  state.json             # arc state machine (committed or gitignored per policy)
  artifacts/             # sanitized proof archive
    <arc-id>/
      screenshot-*.png
      manifest-*.json    # provenance: git SHA, dimensions, redaction method
  failures.json          # failure registry
  selfeval/              # periodic self-eval reports
```

### 4.1 Arc Lifecycle

An arc is a state machine:

```text
created → strategy → execution → critic → evaluation → ops → closed
                        ↑            |          |         |
                        └──── reject ┘          |         │  (human gate)
                                                └── fail ──┘
```

- A **reject** at a gate returns the arc to Execution with the notes and reverts
  its working-tree changes to HEAD, so the rework starts from the base, not the
  rejected attempt. The revert is path-scoped to the arc's footprint.
- On **close**, a `change` arc's commit lands on its own branch `adh/<arc-id>`
  (branch-per-arc), leaving the base untouched and the change ready to open as a PR.
- A **fail** at Evaluation (oracle divergence, invariant break, device failure)
  returns the arc to Execution and records an entry in the failure registry.
- An arc reaches `closed` only after `adh proof verify` passes — **the loop
  physically cannot skip proof.**

______________________________________________________________________

## 5. The Gates (Behavioral Spec)

### 5.1 Model-Gate

Before any stage runs, the harness resolves the role's required model class. If
a `judgment_role` is about to run on a non-reasoning model, the command **exits
non-zero** with a clear error and does not run the stage.

### 5.2 Human Safety Gate

When an arc reaches an irreversible action, `adh run` stops and prints the
pending gate. Advancement requires `adh approve <id> --phrase "<phrase>"` typed
by a human. `--yes`, `ADH_APPROVAL_PHRASE`, and `--dry-run` **never** satisfy a
safety gate. This is structural: the agent path has no code route to self-grant.

### 5.3 Cold-Critic Gate

The Critic stage always runs in a fresh context with no transcript from
Execution. The harness enforces this by spawning the critic with a clean
message history and a distinct system role.

### 5.4 Proof Gate (NO-PROOF-NO-CLOSE)

`adh proof verify <id>` must pass before an arc closes. It checks:

- Every declared artifact exists on disk.
- Manifest hashes match the files on disk — content identity binds the packet to
  the exact bytes it covers.

*What* proof a resolution must carry — its **proof contract** — is not fixed by
the harness. It is configurable per resolution (§3.1 `[proof.contract]`), so each
deployment declares its own acceptance bar: a generic code change might require
"tests pass and CI is green", while a mobile port sets "oracle, invariant, and
on-device proof". Optional provenance fields (a git SHA) and domain-specific
sanitization (screen dimensions, a redaction method) apply only to the artifact
kinds that need them (e.g. screenshots), not to proof in general.

______________________________________________________________________

## 6. The Autonomy Ladder

```text
L0 Manual → L1 Assisted → L2 Hands-off relay → L3 Auto-advance launches → L4 Lights-out
```

Higher levels **auto-launch safe steps** (remove clicks) but **never remove a
gate**. At every level a human still approves anything irreversible or
outward-facing. Raising the autonomy level is itself a human-gated action.

| Level | Behavior                                                  |
| ----- | --------------------------------------------------------- |
| L0    | Every stage transition is manual (`adh step`).            |
| L1    | Harness suggests the next stage; human confirms each.     |
| L2    | Stages relay automatically up to the next gate (default). |
| L3    | Safe launches auto-advance; gates still stop the loop.    |
| L4    | Fully hands-off between gates; gates remain.              |

______________________________________________________________________

## 7. Exit Codes

| Code | Meaning                                                      |
| ---- | ------------------------------------------------------------ |
| `0`  | Success.                                                     |
| `1`  | Generic error.                                               |
| `2`  | Usage / invalid arguments.                                   |
| `3`  | Model-gate refused (judgment role on under-powered model).   |
| `4`  | Pending human gate — advancement blocked, approval required. |
| `5`  | Oracle divergence detected.                                  |
| `6`  | Invariant violation.                                         |
| `7`  | On-device validation failed.                                 |
| `8`  | Proof verification failed (NO-PROOF-NO-CLOSE).               |

`--json` mode returns the same information as a structured object with a
`status`, `code`, and command-specific payload, so the harness can be scripted
and chained by hooks.

______________________________________________________________________

## 8. Output & Logging

- Human mode: concise, colorized stage banners; a pending gate is visually
  distinct and prints the exact approval command to run.
- `--json`: one JSON object per command invocation on stdout; logs go to stderr.
- All logs redact secrets and sensitive screen regions.
- Every stage transition and gate decision is appended to an append-only audit
  log for the periodic self-eval and registry audit.

______________________________________________________________________

## 9. Non-Goals

- The harness does not implement the game or its engine; it orchestrates agents
  that do.
- It does not grade agent output with an LLM-as-judge where a deterministic
  oracle or invariant test can decide correctness instead.
- It does not remove human authority over irreversible actions at any autonomy
  level.
