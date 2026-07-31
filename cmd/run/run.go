// Package run implements the "run" CLI command: relay an arc through stages
// until it reaches a human gate or closes (SPEC §2.1), honoring the autonomy
// level and rework budget resolved from config. --relay drives via the relay;
// otherwise the deterministic mock advances the stages.
package run

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/agentic-dev-harness/cmd/root"
	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/authority"
	"github.com/StevenACoffman/agentic-dev-harness/internal/config"
	"github.com/StevenACoffman/agentic-dev-harness/internal/contextstore"
	"github.com/StevenACoffman/agentic-dev-harness/internal/critic"
	"github.com/StevenACoffman/agentic-dev-harness/internal/evaluation"
	"github.com/StevenACoffman/agentic-dev-harness/internal/model"
	"github.com/StevenACoffman/agentic-dev-harness/internal/prompt"
	"github.com/StevenACoffman/agentic-dev-harness/internal/relay"
	"github.com/StevenACoffman/agentic-dev-harness/internal/stage"
	"github.com/StevenACoffman/agentic-dev-harness/internal/state"
	"github.com/StevenACoffman/agentic-dev-harness/internal/toolreg"
	"github.com/StevenACoffman/agentic-dev-harness/internal/worker"
	"github.com/StevenACoffman/agentic-dev-harness/internal/worktree"
)

// routingGapCode is the exit code for a critic routing gap (§10, §19.1): the
// environment did not teach the critic, so the relay refuses to emit a prompt.
// requalifyCode is the worker-change refusal (§14); codeFailed is a terminal
// evaluation fail past the rework budget (§4.1), which stops the drive.
const (
	routingGapCode = 12
	requalifyCode  = 9
	codeFailed     = 1
)

// Config holds the configuration for the run command.
type Config struct {
	*root.Config
	Relay    bool
	Response string
	Flags    *ff.FlagSet
	Command  *ff.Command
}

// relayResult is the awaiting outcome `run --relay` carries in its data payload —
// the same shape `step --relay` uses, so an agent parses either identically.
type relayResult struct {
	Arc    string `json:"arc"`
	Stage  string `json:"stage"`
	Status string `json:"status"`
	Prompt string `json:"prompt,omitempty"`
}

// New creates and registers the run command with the given parent config.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("run").SetParent(parent.Flags)
	cfg.Flags.BoolVar(&cfg.Relay, 0, "relay",
		"drive the arc by relaying prompts to an operator instead of the mock model")
	cfg.Flags.StringVar(&cfg.Response, 0, "response", "",
		"resume the pending relayed turn with the reply in this file (- for stdin)")
	cfg.Command = &ff.Command{
		Name:      "run",
		Usage:     "agentic-dev-harness run [--relay [--response <file>]] <arc-id>",
		ShortHelp: "advance an arc through the loop until a gate or completion",
		LongHelp:  "Relay the arc through stages until a human gate or closure (SPEC §2.1).",
		Flags:     cfg.Flags,
		Exec:      cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

func (cfg *Config) exec(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("run: requires an arc id")
	}
	conf, err := config.Load(cfg.ConfigGetenv())
	if err != nil {
		return fmt.Errorf("run: %w", err)
	}
	if err := cfg.requalifyGate(&conf); err != nil {
		return err
	}
	renderer, err := prompt.Default()
	if err != nil {
		return fmt.Errorf("run: %w", err)
	}
	store := state.Default()
	arc, err := store.Get(args[0])
	if err != nil {
		return fmt.Errorf("run: %w", err)
	}
	if cfg.Relay {
		return cfg.driveRelay(ctx, store, &arc, &conf, renderer)
	}
	return cfg.drive(ctx, store, &arc, &conf, renderer)
}

// driveRelay drives the arc by relaying (SPEC §1-2): if a reply is supplied it
// resumes the pending turn, then advances — running deterministic evaluation
// inline (§19.2) and emitting the next model stage's prompt to park for a reply,
// or stopping at the ops gate. So one `run --relay --response <file>` applies the
// reply, runs any evaluation, and emits the next prompt, collapsing the relay's
// emit → resume → eval cycle into a single call. It shares the relay engine and
// worktree grounding with `step`; the awaiting outcome uses step's data shape.
func (cfg *Config) driveRelay(
	ctx context.Context,
	store *state.Store,
	arc *adh.Arc,
	conf *config.Config,
	renderer stage.Prompter,
) error {
	judgment := conf.JudgmentRoles()
	recordLessons := conf.CriticUnconfirmed() == config.UnconfirmedLesson
	maxReworks := conf.MaxReworks()
	if cfg.Response != "" {
		if err := cfg.resumeRelay(ctx, store, arc, renderer, judgment); err != nil {
			return err
		}
	}
	for arc.Status == adh.StatusOpen {
		switch arc.Stage {
		case adh.StageOps:
			return cfg.block(
				store,
				arc,
				root.ReasonAtOps,
				"ops is the ship gate; approve then `close`",
			)
		case adh.StageEvaluation:
			if err := cfg.adjudicate(ctx, arc, recordLessons, maxReworks); err != nil {
				return err
			}
			if err := store.Save(arc); err != nil {
				return fmt.Errorf("run: %w", err)
			}
		default:
			return cfg.emitRelay(ctx, store, conf, renderer, arc, judgment)
		}
	}
	return cfg.reportDone(arc)
}

// resumeRelay applies the operator's reply to the pending turn and captures an
// execution turn's footprint. It refuses a reply with no matching pending turn.
func (cfg *Config) resumeRelay(
	ctx context.Context,
	store *state.Store,
	arc *adh.Arc,
	renderer stage.Prompter,
	judgment authority.JudgmentRoles,
) error {
	if arc.Pending == nil || arc.Pending.Stage != arc.Stage {
		return fmt.Errorf("run: arc %s has no pending %s turn to resume", arc.ID, arc.Stage)
	}
	text, err := cfg.readResponse()
	if err != nil {
		return fmt.Errorf("run: %w", err)
	}
	wasExecution := arc.Stage == adh.StageExecution
	if _, err := relay.Resume(ctx, arc, text, renderer, judgment); err != nil {
		return fmt.Errorf("run: %w", err)
	}
	if wasExecution {
		worktree.CaptureFootprint(cfg.repoDir(), arc)
	}
	if err := store.Save(arc); err != nil {
		return fmt.Errorf("run: %w", err)
	}
	return nil
}

// emitRelay grounds and emits the current model stage's prompt, parks it, and
// reports the awaiting outcome — or a routing gap (§19.1).
func (cfg *Config) emitRelay(
	ctx context.Context,
	store *state.Store,
	conf *config.Config,
	renderer stage.Prompter,
	arc *adh.Arc,
	judgment authority.JudgmentRoles,
) error {
	reg, err := toolreg.LoadRepo(cfg.repoDir())
	if err != nil {
		return fmt.Errorf("run: %w", err)
	}
	in := critic.Inputs{AcceptanceBar: conf.ProofContract(arc.Resolution), Tools: reg.Tools}
	if arc.Stage == adh.StageCritic {
		in.Diff = worktree.Diff(cfg.repoDir(), arc.Paths)
	}
	out, err := relay.Emit(
		arc, contextstore.DefaultStoreDir, in, renderer, model.Relay{}.ModelClass(), judgment,
	)
	if err != nil {
		return fmt.Errorf("run: %w", err)
	}
	if out.Kind == relay.Gap {
		return cfg.reportGap(ctx, arc)
	}
	if err := store.Save(arc); err != nil {
		return fmt.Errorf("run: %w", err)
	}
	return cfg.reportAwaiting(arc, out.Prompt)
}

// reportAwaiting reports a parked relay turn: a success outcome carrying the arc,
// stage, and prompt under --jsonl (step's shape), else the human prompt.
func (cfg *Config) reportAwaiting(arc *adh.Arc, promptText string) error {
	if cfg.JSONL {
		rec := relayResult{
			Arc:    arc.ID,
			Stage:  string(arc.Stage),
			Status: "awaiting",
			Prompt: promptText,
		}
		if err := cfg.EmitOK(rec); err != nil {
			return fmt.Errorf("run: %w", err)
		}
		return nil
	}
	_, _ = fmt.Fprintf(
		cfg.Stdout,
		"%s awaiting response at %s:\n%s\n",
		arc.ID,
		arc.Stage,
		promptText,
	)
	return nil
}

// reportGap reports a critic routing gap (§19.1, exit 12): the environment did not
// teach the critic, so no prompt was emitted.
func (cfg *Config) reportGap(ctx context.Context, arc *adh.Arc) error {
	cfg.recordMiss(ctx, arc)
	msg := fmt.Sprintf(
		"critic ungrounded for arc %s: no context or proof routed for its labels/paths (§19.1); teach the repo (adh context) or record proof",
		arc.ID,
	)
	if cfg.JSONL {
		if err := cfg.EmitBlocked(routingGapCode, root.ReasonUngrounded, msg); err != nil {
			return fmt.Errorf("run: %w", err)
		}
	} else {
		_, _ = fmt.Fprintf(cfg.Stderr, "run: %s\n", msg)
	}
	return root.ExitError(routingGapCode)
}

// readResponse reads the operator's reply from the --response file, or from stdin
// when it is "-".
func (cfg *Config) readResponse() (string, error) {
	if cfg.Response == "-" {
		data, err := io.ReadAll(cfg.Stdin)
		if err != nil {
			return "", fmt.Errorf("reading response from stdin: %w", err)
		}
		return string(data), nil
	}
	data, err := os.ReadFile(cfg.Response)
	if err != nil {
		return "", fmt.Errorf("reading response file %s: %w", cfg.Response, err)
	}
	return string(data), nil
}

// drive relays the arc through stages until it parks at a gate or closes, honoring
// the autonomy level. It emits the terminal outcome (blocked or completed).
func (cfg *Config) drive(
	ctx context.Context,
	store *state.Store,
	arc *adh.Arc,
	conf *config.Config,
	renderer stage.Prompter,
) error {
	level := conf.AutonomyLevel()
	judgment := conf.JudgmentRoles()
	recordLessons := conf.CriticUnconfirmed() == config.UnconfirmedLesson
	maxReworks := conf.MaxReworks()
	for arc.Status == adh.StatusOpen {
		if arc.Stage == adh.StageOps {
			return cfg.block(
				store,
				arc,
				root.ReasonAtOps,
				"ops is the ship gate; approve then `close`",
			)
		}
		from := arc.Stage
		if err := cfg.advanceStage(
			ctx,
			renderer,
			arc,
			judgment,
			recordLessons,
			maxReworks,
		); err != nil {
			return err
		}
		if err := store.Save(arc); err != nil {
			return fmt.Errorf("run: %w", err)
		}
		if arc.Status != adh.StatusOpen {
			// Terminal state (evaluation failed past its rework budget, §4.1):
			// reportDone reports it, ahead of the autonomy gate below.
			break
		}
		// Per-stage progress on the diagnostic stream (Info): visible under
		// --verbose even in --jsonl mode, where stdout carries only the terminal
		// outcome.
		cfg.Log.InfoContext(ctx, "stage advanced",
			"op", "run", "arc", arc.ID, "from", string(from), "to", string(arc.Stage))
		if !cfg.JSONL {
			_, _ = fmt.Fprintf(cfg.Stdout, "ran %s\n", from)
		}
		if !stage.AutoAdvances(from, level) {
			return cfg.block(
				store,
				arc,
				root.ReasonGate,
				string(arc.Stage)+" requires a human gate",
			)
		}
	}
	return cfg.reportDone(arc)
}

// reportDone emits the terminal outcome when the loop completes (the arc left the
// open state). An arc that failed evaluation past its rework budget (§4.1) is an
// error outcome (reason failed, exit 1) so the drive stops for a human — the
// per-finding kinds are already in the failure registry from the reworks. Any
// other terminal state is a success outcome.
func (cfg *Config) reportDone(arc *adh.Arc) error {
	if arc.Status == adh.StatusFailed {
		msg := fmt.Sprintf("arc %s failed evaluation after %d rework(s); escalate to a human",
			arc.ID, arc.Reworks)
		if cfg.JSONL {
			if err := cfg.EmitError(codeFailed, root.ReasonFailed, msg); err != nil {
				return fmt.Errorf("run: %w", err)
			}
		} else {
			_, _ = fmt.Fprintf(cfg.Stderr, "run: %s\n", msg)
		}
		return root.ExitError(codeFailed)
	}
	if cfg.JSONL {
		if err := cfg.EmitOK(
			map[string]string{"arc": arc.ID, "status": string(arc.Status)},
		); err != nil {
			return fmt.Errorf("run: %w", err)
		}
		return nil
	}
	_, _ = fmt.Fprintf(cfg.Stdout, "arc %s %s\n", arc.ID, arc.Status)
	return nil
}

// advanceStage runs one stage in place. Evaluation is the deterministic
// disposition (§19.2) — it adjudicates the critic's findings, never a model step;
// every other stage runs through the mock model. This keeps evaluation
// deterministic on the run path too, matching `adh eval` and the relay.
func (cfg *Config) advanceStage(
	ctx context.Context,
	renderer stage.Prompter,
	arc *adh.Arc,
	judgment authority.JudgmentRoles,
	recordLessons bool,
	maxReworks int,
) error {
	if arc.Stage == adh.StageEvaluation {
		return cfg.adjudicate(ctx, arc, recordLessons, maxReworks)
	}
	if err := stage.Execute(ctx, model.Mock{}, renderer, arc, judgment); err != nil {
		return fmt.Errorf("run: %w", err)
	}
	return nil
}

// adjudicate runs the deterministic Evaluation disposition (§19.2) on the arc's
// findings and applies the verdict, mutating the arc. It is shared by the mock
// drive and the relay drive; the caller persists the arc.
func (cfg *Config) adjudicate(
	ctx context.Context,
	arc *adh.Arc,
	recordLessons bool,
	maxReworks int,
) error {
	adjudicator, err := evaluation.RepoAdjudicatorFor(cfg.repoDir())
	if err != nil {
		return fmt.Errorf("run: %w", err)
	}
	verdict, err := evaluation.Adjudicate(ctx, &adjudicator, arc.Findings)
	if err != nil {
		return fmt.Errorf("run: %w", err)
	}
	if err := evaluation.Apply(arc, &verdict, recordLessons, maxReworks); err != nil {
		return fmt.Errorf("run: %w", err)
	}
	return nil
}

// recordMiss logs the routing gap to the miss log so the router can learn from it
// (§10.3): accumulated misses earn a route proposal (`adh context misses`). It is
// best-effort — a failed append must not mask the gap the arc actually hit.
func (cfg *Config) recordMiss(ctx context.Context, arc *adh.Arc) {
	miss := contextstore.Miss{Arc: arc.ID, Labels: arc.Labels, Paths: arc.Paths}
	if err := contextstore.AppendMiss(
		filepath.Join(cfg.repoDir(), contextstore.MissFile), miss,
	); err != nil {
		cfg.Log.WarnContext(ctx, "record routing miss", "arc", arc.ID, "err", err)
	}
}

// repoDir is the repository root — the --repo global, or the current directory.
func (cfg *Config) repoDir() string {
	if cfg.Repo != "" {
		return cfg.Repo
	}
	return "."
}

// requalifyGate refuses a run when the worker changed from the recorded epoch
// (§14): the fixed worker must be requalified before normal runs resume. Exit 9,
// a blocked outcome with the requalify reason. A never-requalified workspace is
// not gated (RequalifyNeeded requires a recorded epoch).
func (cfg *Config) requalifyGate(conf *config.Config) error {
	needed, err := worker.RequalifyNeeded(
		worker.DefaultStateFile,
		worker.EpochFor(conf.BaselineModels()),
	)
	if err != nil {
		return fmt.Errorf("run: %w", err)
	}
	if !needed {
		return nil
	}
	const msg = "worker changed; run `adh worker requalify` before continuing (§14)"
	if cfg.JSONL {
		if emitErr := cfg.EmitBlocked(requalifyCode, root.ReasonRequalify, msg); emitErr != nil {
			return fmt.Errorf("run: %w", emitErr)
		}
	} else {
		_, _ = fmt.Fprintf(cfg.Stderr, "run: %s\n", msg)
	}
	return root.ExitError(requalifyCode)
}

// block parks the arc at a human gate (StatusBlocked) and records why, so the
// approve/reject loop can act on it. Blocking at a gate is not an error, so run
// exits 0; the outcome's status is blocked with the machine reason token.
func (cfg *Config) block(store *state.Store, arc *adh.Arc, reason, message string) error {
	arc.Status = adh.StatusBlocked
	arc.History = append(arc.History, "blocked: "+message)
	if err := store.Save(arc); err != nil {
		return fmt.Errorf("run: %w", err)
	}
	if cfg.JSONL {
		if err := cfg.EmitBlocked(0, reason, message); err != nil {
			return fmt.Errorf("run: %w", err)
		}
		return nil
	}
	_, _ = fmt.Fprintf(cfg.Stdout, "blocked at %s: %s\n", arc.Stage, message)
	return nil
}
