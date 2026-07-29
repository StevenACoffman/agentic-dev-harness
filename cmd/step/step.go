// Package step implements the "step" CLI command: run exactly one stage
// transition on an arc through the model seam, then stop (SPEC §2.1).
//
// With --relay the transition splits across two invocations so an operator
// (Claude driving adh via a skill) can supply the model's reasoning: the first
// invocation emits the stage's prompt and parks a pending turn on the arc; the
// second, given --response, feeds the operator's reply back and advances the arc.
// Without --relay it runs the deterministic mock in one shot, as before.
package step

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

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

// Turn outcomes reported to the caller. "awaiting" means the prompt was emitted
// and the arc is parked for a reply; "advanced" means the arc moved on a stage.
const (
	statusAwaiting = "awaiting"
	statusAdvanced = "advanced"
	// opsGateCode is the exit code when an arc has reached the ops ship gate
	// (SPEC §7 code 4: a pending human gate blocks advancement); routingGapCode is
	// the §10 routing gap (exit 12).
	opsGateCode    = 4
	routingGapCode = 12
	// requalifyCode is the worker-change refusal (§14); codeFailed is a terminal
	// evaluation fail past the rework budget (§4.1).
	requalifyCode = 9
	codeFailed    = 1
)

// Config holds the configuration for the step command.
type Config struct {
	*root.Config
	Relay    bool
	Response string
	Flags    *ff.FlagSet
	Command  *ff.Command
}

// result is the machine-readable outcome emitted under --jsonl.
type result struct {
	Arc    string `json:"arc"`
	Stage  string `json:"stage"`
	Status string `json:"status"`
	Prompt string `json:"prompt,omitempty"`
}

// New creates and registers the step command with the given parent config.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("step").SetParent(parent.Flags)
	cfg.Flags.BoolVar(&cfg.Relay, 0, "relay",
		"emit the stage prompt for an operator to answer instead of calling a model")
	cfg.Flags.StringVar(&cfg.Response, 0, "response", "",
		"resume a relayed turn with the reply in this file (- for stdin)")
	cfg.Command = &ff.Command{
		Name:      "step",
		Usage:     "agentic-dev-harness step [--relay [--response <file>]] [--json] <arc-id>",
		ShortHelp: "run exactly one stage transition on an arc",
		LongHelp:  "Run the arc's current stage through the model, then stop (SPEC §2.1).",
		Flags:     cfg.Flags,
		Exec:      cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

func (cfg *Config) exec(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("step: requires an arc id")
	}
	store := state.Default()
	arc, err := store.Get(args[0])
	if err != nil {
		return fmt.Errorf("step: %w", err)
	}
	if arc.Status != adh.StatusOpen {
		return fmt.Errorf("step: arc %s is not open (status %s)", arc.ID, arc.Status)
	}
	if arc.Stage == adh.StageOps {
		return cfg.reportOpsGate(&arc)
	}
	// Evaluation is deterministic on the relay path (§19.2): it adjudicates the
	// critic's findings against repository artifacts, not by relaying another
	// prompt. Point the operator at the command that does it.
	if cfg.Relay && arc.Stage == adh.StageEvaluation {
		return fmt.Errorf(
			"step: arc %s is at evaluation; adjudicate its findings with `adh eval %s`",
			arc.ID,
			arc.ID,
		)
	}
	conf, err := config.Load(cfg.ConfigGetenv())
	if err != nil {
		return fmt.Errorf("step: %w", err)
	}
	if err := cfg.requalifyGate(&conf); err != nil {
		return err
	}
	renderer, err := prompt.Default()
	if err != nil {
		return fmt.Errorf("step: %w", err)
	}
	judgment := conf.JudgmentRoles()

	switch {
	case !cfg.Relay && arc.Stage == adh.StageEvaluation:
		// Evaluation is the deterministic disposition (§19.2), not a model step —
		// the same path `adh eval` and the relay take.
		return cfg.disposeEval(ctx, store, &conf, &arc)
	case !cfg.Relay:
		return cfg.advance(ctx, store, model.Mock{}, renderer, &arc, judgment)
	case cfg.Response != "":
		return cfg.resume(ctx, store, renderer, &arc, judgment)
	default:
		return cfg.emit(store, &conf, renderer, &arc, judgment)
	}
}

// disposeEval adjudicates the critic's findings and disposes of the arc (§19.2),
// the same deterministic evaluation as `adh eval`.
func (cfg *Config) disposeEval(
	ctx context.Context,
	store *state.Store,
	conf *config.Config,
	arc *adh.Arc,
) error {
	adjudicator, err := evaluation.RepoAdjudicatorFor(cfg.repoDir())
	if err != nil {
		return fmt.Errorf("step: %w", err)
	}
	verdict, err := evaluation.Adjudicate(ctx, adjudicator, arc.Findings)
	if err != nil {
		return fmt.Errorf("step: %w", err)
	}
	recordLessons := conf.CriticUnconfirmed() == config.UnconfirmedLesson
	if err := evaluation.Apply(arc, &verdict, recordLessons, conf.MaxReworks()); err != nil {
		return fmt.Errorf("step: %w", err)
	}
	if err := store.Save(arc); err != nil {
		return fmt.Errorf("step: %w", err)
	}
	if arc.Status == adh.StatusFailed {
		return cfg.reportFailed(arc)
	}
	return cfg.report(arc, statusAdvanced, "")
}

// reportFailed reports an arc that failed evaluation past its rework budget (§4.1):
// an error outcome (reason failed, exit 1) so the drive stops for a human. The
// per-finding kinds are already in the failure registry from the reworks.
func (cfg *Config) reportFailed(arc *adh.Arc) error {
	msg := fmt.Sprintf("arc %s failed evaluation after %d rework(s); escalate to a human",
		arc.ID, arc.Reworks)
	if cfg.JSONL {
		if err := cfg.EmitError(codeFailed, root.ReasonFailed, msg); err != nil {
			return fmt.Errorf("step: %w", err)
		}
	} else {
		_, _ = fmt.Fprintf(cfg.Stderr, "step: %s\n", msg)
	}
	return root.ExitError(codeFailed)
}

// advance runs one stage synchronously through client (the mock) and saves it.
func (cfg *Config) advance(
	ctx context.Context,
	store *state.Store,
	client stage.Client,
	renderer stage.Prompter,
	arc *adh.Arc,
	judgment authority.JudgmentRoles,
) error {
	if err := stage.Execute(ctx, client, renderer, arc, judgment); err != nil {
		return fmt.Errorf("step: %w", err)
	}
	if err := store.Save(arc); err != nil {
		return fmt.Errorf("step: %w", err)
	}
	return cfg.report(arc, statusAdvanced, "")
}

// emit renders the stage's prompt, parks it as a pending turn on the arc, and
// prints it for the operator. Re-emitting an already-open turn for the same
// stage is idempotent: it reprints the parked prompt without opening a new one.
func (cfg *Config) emit(
	store *state.Store,
	conf *config.Config,
	renderer stage.Prompter,
	arc *adh.Arc,
	judgment authority.JudgmentRoles,
) error {
	// Ground the stage from repository state (§10, §19.1): the acceptance bar (§19.4),
	// the available tools (§13), and — for the critic — the diff of the change under
	// review (the shell reads them; the engine routes context, renders, and parks).
	reg, err := toolreg.LoadRepo(cfg.repoDir())
	if err != nil {
		return fmt.Errorf("step: %w", err)
	}
	in := critic.Inputs{AcceptanceBar: conf.ProofContract(arc.Resolution), Tools: reg.Tools}
	if arc.Stage == adh.StageCritic {
		in.Diff = worktree.Diff(cfg.repoDir(), arc.Paths)
	}
	outcome, err := relay.Emit(
		arc, contextstore.DefaultStoreDir, in, renderer, model.Relay{}.ModelClass(), judgment,
	)
	if err != nil {
		return fmt.Errorf("step: %w", err)
	}
	if outcome.Kind == relay.Gap {
		return cfg.reportGap(arc)
	}
	if err := store.Save(arc); err != nil {
		return fmt.Errorf("step: %w", err)
	}
	return cfg.report(arc, statusAwaiting, outcome.Prompt)
}

// reportOpsGate reports an arc that has reached the ops ship gate (§5.2): a
// blocked outcome under --jsonl, else a stderr line, and exit 4.
func (cfg *Config) reportOpsGate(arc *adh.Arc) error {
	msg := fmt.Sprintf("arc %s is at the ops gate; ship it with `close`", arc.ID)
	if cfg.JSONL {
		if err := cfg.EmitBlocked(opsGateCode, root.ReasonAtOps, msg); err != nil {
			return fmt.Errorf("step: %w", err)
		}
	} else {
		_, _ = fmt.Fprintf(cfg.Stderr, "step: %s\n", msg)
	}
	return root.ExitError(opsGateCode)
}

// reportGap reports a critic routing gap (§19.1): the environment did not teach
// the critic, so no prompt was emitted. It is a blocked outcome under --jsonl,
// else a stderr line, and exits 12.
func (cfg *Config) reportGap(arc *adh.Arc) error {
	msg := fmt.Sprintf(
		"critic ungrounded for arc %s: no context or proof routed for its labels/paths (§19.1); teach the repo (adh context) or record proof, do not guess",
		arc.ID,
	)
	if cfg.JSONL {
		if err := cfg.EmitBlocked(routingGapCode, root.ReasonUngrounded, msg); err != nil {
			return fmt.Errorf("step: %w", err)
		}
	} else {
		_, _ = fmt.Fprintf(cfg.Stderr, "step: %s\n", msg)
	}
	return root.ExitError(routingGapCode)
}

// resume feeds the operator's reply back through the relay, advancing the arc
// and clearing the pending turn. It refuses a reply that does not match an open
// turn for the arc's current stage, so a stale reply cannot advance the wrong
// stage.
func (cfg *Config) resume(
	ctx context.Context,
	store *state.Store,
	renderer stage.Prompter,
	arc *adh.Arc,
	judgment authority.JudgmentRoles,
) error {
	switch {
	case arc.Pending == nil:
		return fmt.Errorf("step: arc %s has no pending turn to resume", arc.ID)
	case arc.Pending.Stage != arc.Stage:
		return fmt.Errorf("step: pending turn is for %s but arc %s is at %s",
			arc.Pending.Stage, arc.ID, arc.Stage)
	}
	text, err := cfg.readResponse()
	if err != nil {
		return fmt.Errorf("step: %w", err)
	}
	// The engine validates the reply against the stage's contract before any state
	// changes (§19.2, §12); the shell captures the execution footprint afterward
	// because that needs the worktree.
	wasExecution := arc.Stage == adh.StageExecution
	if _, err := relay.Resume(ctx, arc, text, renderer, judgment); err != nil {
		return fmt.Errorf("step: %w", err)
	}
	if wasExecution {
		worktree.CaptureFootprint(cfg.repoDir(), arc)
	}
	if err := store.Save(arc); err != nil {
		return fmt.Errorf("step: %w", err)
	}
	return cfg.report(arc, statusAdvanced, "")
}

// repoDir is the repository root — the --repo global, or the current directory.
func (cfg *Config) repoDir() string {
	if cfg.Repo != "" {
		return cfg.Repo
	}
	return "."
}

// requalifyGate refuses a step when the worker changed from the recorded epoch
// (§14): the fixed worker must be requalified before normal runs resume. Exit 9,
// a blocked outcome with the requalify reason. A never-requalified workspace is
// not gated (RequalifyNeeded requires a recorded epoch).
func (cfg *Config) requalifyGate(conf *config.Config) error {
	needed, err := worker.RequalifyNeeded(
		worker.DefaultStateFile,
		worker.EpochFor(conf.BaselineModels()),
	)
	if err != nil {
		return fmt.Errorf("step: %w", err)
	}
	if !needed {
		return nil
	}
	const msg = "worker changed; run `adh worker requalify` before continuing (§14)"
	if cfg.JSONL {
		if emitErr := cfg.EmitBlocked(requalifyCode, root.ReasonRequalify, msg); emitErr != nil {
			return fmt.Errorf("step: %w", emitErr)
		}
	} else {
		_, _ = fmt.Fprintf(cfg.Stderr, "step: %s\n", msg)
	}
	return root.ExitError(requalifyCode)
}

// readResponse reads the operator's reply from the --response file, or from
// stdin when it is "-".
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

// report prints the outcome: one JSON object under --jsonl, otherwise a
// human-readable line (and, when awaiting, the prompt itself on the following lines).
func (cfg *Config) report(arc *adh.Arc, status, promptText string) error {
	if cfg.JSONL {
		rec := result{Arc: arc.ID, Stage: string(arc.Stage), Status: status, Prompt: promptText}
		if err := cfg.EmitOK(rec); err != nil {
			return fmt.Errorf("step: %w", err)
		}
		return nil
	}
	if status == statusAwaiting {
		_, _ = fmt.Fprintf(
			cfg.Stdout,
			"%s awaiting response at %s:\n%s\n",
			arc.ID,
			arc.Stage,
			promptText,
		)
		return nil
	}
	_, _ = fmt.Fprintf(cfg.Stdout, "%s now at %s (%s)\n", arc.ID, arc.Stage, arc.Status)
	return nil
}
