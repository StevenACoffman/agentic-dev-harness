// Package approve implements the "approve" CLI command: satisfy a pending human
// safety gate on an arc (SPEC §5.2). Approval requires an explicit phrase — the
// arc's own ID — typed by a human via --phrase. --yes and --dry-run never
// satisfy a gate, so the agent has no code route to self-grant.
package approve

import (
	"context"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/agentic-dev-harness/cmd/root"
	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/authority"
	"github.com/StevenACoffman/agentic-dev-harness/internal/state"
	"github.com/StevenACoffman/agentic-dev-harness/internal/worktree"
)

// Config holds the configuration for the approve command.
type Config struct {
	*root.Config
	Phrase  string
	Flags   *ff.FlagSet
	Command *ff.Command
}

// New creates and registers the approve command with the given parent config.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("approve").SetParent(parent.Flags)
	cfg.Flags.StringVar(
		&cfg.Phrase,
		'p',
		"phrase",
		"",
		"the approval phrase (the arc ID), typed by a human",
	)
	cfg.Command = &ff.Command{
		Name:      "approve",
		Usage:     "agentic-dev-harness approve <arc-id> --phrase <arc-id>",
		ShortHelp: "approve a pending human gate on an arc",
		LongHelp:  "Approve a pending gate with an explicit human phrase (SPEC §5.2). --yes never satisfies a gate.",
		Flags:     cfg.Flags,
		Exec:      cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

func (cfg *Config) exec(_ context.Context, args []string) error {
	if len(args) == 0 {
		return root.MissingOperandError{Scope: "approve", Kind: root.OperandArc}
	}
	id := args[0]
	if cfg.Yes {
		_, _ = fmt.Fprintln(
			cfg.Stderr,
			"note: --yes does not satisfy a safety gate; the phrase is still required",
		)
	}
	store := state.Default()
	arc, err := store.Get(id)
	if err != nil {
		return fmt.Errorf("approve: %w", err)
	}
	if arc.Status != adh.StatusBlocked {
		return &adh.Error{
			Code: adh.ECONFLICT,
			Message: fmt.Sprintf(
				"approve: arc %s is not waiting at a gate (status %s)",
				id,
				arc.Status,
			),
		}
	}
	cfg.showExecutableConfig()
	required := authority.RequiredApprovalPhrase(id)
	if !authority.GateSatisfied(required, cfg.Phrase, cfg.DryRun) {
		msg := "gate not satisfied: pass --phrase " + required + " to approve"
		if cfg.JSONL {
			if err := cfg.EmitBlocked(4, root.ReasonGate, msg); err != nil {
				return fmt.Errorf("approve: %w", err)
			}
		} else {
			_, _ = fmt.Fprintln(cfg.Stderr, msg)
		}
		return root.ExitError(4)
	}
	arc.Status = adh.StatusOpen
	arc.History = append(arc.History, "gate approved")
	if err := store.Save(&arc); err != nil {
		return fmt.Errorf("approve: %w", err)
	}
	if cfg.JSONL {
		if err := cfg.EmitOK(map[string]string{"arc": id, "status": "approved"}); err != nil {
			return fmt.Errorf("approve: %w", err)
		}
		return nil
	}
	_, _ = fmt.Fprintf(cfg.Stdout, "approved %s\n", id)
	return nil
}

// showExecutableConfig puts any uncommitted change to executable config in front of the
// human about to approve (execconfig). The critic sees these changes too, but a gate is
// where a human decides, and a changed tool registry or CLAUDE.md is exactly what they
// should not approve unseen. To stderr, so --jsonl stdout stays one clean envelope.
func (cfg *Config) showExecutableConfig() {
	paths, diff := worktree.ExecutableConfigChanges(cfg.repoDir())
	if len(paths) == 0 {
		return
	}
	_, _ = fmt.Fprintf(cfg.Stderr,
		"executable config changed in the working tree (adh runs it only once committed): %s\n",
		strings.Join(paths, ", "))
	if diff != "" {
		_, _ = fmt.Fprint(cfg.Stderr, diff)
	}
}

// repoDir is the repository root — the --repo global, or the current directory.
func (cfg *Config) repoDir() string {
	if cfg.Repo != "" {
		return cfg.Repo
	}
	return "."
}
