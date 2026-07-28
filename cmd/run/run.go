// Package run implements the "run" CLI command: relay an arc through stages
// until it reaches a human gate or closes (SPEC §2.1), honoring the autonomy
// level. Until config wiring lands it relays at the default level L2.
package run

import (
	"context"
	"errors"
	"fmt"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/agentic-dev-harness/cmd/root"
	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/authority"
	"github.com/StevenACoffman/agentic-dev-harness/internal/config"
	"github.com/StevenACoffman/agentic-dev-harness/internal/evaluation"
	"github.com/StevenACoffman/agentic-dev-harness/internal/model"
	"github.com/StevenACoffman/agentic-dev-harness/internal/prompt"
	"github.com/StevenACoffman/agentic-dev-harness/internal/stage"
	"github.com/StevenACoffman/agentic-dev-harness/internal/state"
)

// Config holds the configuration for the run command.
type Config struct {
	*root.Config
	Flags   *ff.FlagSet
	Command *ff.Command
}

// New creates and registers the run command with the given parent config.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("run").SetParent(parent.Flags)
	cfg.Command = &ff.Command{
		Name:      "run",
		Usage:     "agentic-dev-harness run <arc-id>",
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
	renderer, err := prompt.Default()
	if err != nil {
		return fmt.Errorf("run: %w", err)
	}
	store := state.Default()
	arc, err := store.Get(args[0])
	if err != nil {
		return fmt.Errorf("run: %w", err)
	}
	return cfg.drive(ctx, store, &arc, &conf, renderer)
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
		if err := cfg.advanceStage(ctx, renderer, arc, judgment, recordLessons); err != nil {
			return err
		}
		if err := store.Save(arc); err != nil {
			return fmt.Errorf("run: %w", err)
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
// open state): a success outcome under --jsonl, else a human line.
func (cfg *Config) reportDone(arc *adh.Arc) error {
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
) error {
	if arc.Stage == adh.StageEvaluation {
		adjudicator, err := evaluation.RepoAdjudicatorFor(cfg.repoDir())
		if err != nil {
			return fmt.Errorf("run: %w", err)
		}
		verdict, err := evaluation.Adjudicate(ctx, adjudicator, arc.Findings)
		if err != nil {
			return fmt.Errorf("run: %w", err)
		}
		if err := evaluation.Apply(arc, &verdict, recordLessons); err != nil {
			return fmt.Errorf("run: %w", err)
		}
		return nil
	}
	if err := stage.Execute(ctx, model.Mock{}, renderer, arc, judgment); err != nil {
		return fmt.Errorf("run: %w", err)
	}
	return nil
}

// repoDir is the repository root — the --repo global, or the current directory.
func (cfg *Config) repoDir() string {
	if cfg.Repo != "" {
		return cfg.Repo
	}
	return "."
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
