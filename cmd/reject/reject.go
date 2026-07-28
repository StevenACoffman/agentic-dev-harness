// Package reject implements the "reject" CLI command: the negative half of the
// human gate (SPEC §5.2, §2.3). Rejecting a blocked arc returns it to Execution
// and undoes the change it was carrying — the symmetric counterpart of approve,
// which advances the same gate. The arc's working-tree changes are reverted so a
// re-execution starts from the base, not the rejected attempt.
package reject

import (
	"context"
	"errors"
	"fmt"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/agentic-dev-harness/cmd/root"
	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/state"
	"github.com/StevenACoffman/agentic-dev-harness/internal/vcs"
)

// Config holds the configuration for the reject command.
type Config struct {
	*root.Config
	Reason  string
	Flags   *ff.FlagSet
	Command *ff.Command
}

// New creates and registers the reject command with the given parent config.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("reject").SetParent(parent.Flags)
	cfg.Flags.StringVar(&cfg.Reason, 'r', "reason", "", "why the gate was rejected")
	cfg.Command = &ff.Command{
		Name:      "reject",
		Usage:     "agentic-dev-harness reject [--reason text] <arc-id>",
		ShortHelp: "reject a pending human gate, returning the arc to Execution",
		LongHelp:  "Reject a pending gate (SPEC §5.2): revert the arc's working-tree changes and return it to Execution to be reworked.",
		Flags:     cfg.Flags,
		Exec:      cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

func (cfg *Config) exec(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("reject: requires an arc id")
	}
	id := args[0]
	store := state.Default()
	arc, err := store.Get(id)
	if err != nil {
		return fmt.Errorf("reject: %w", err)
	}
	// Symmetric with approve: only a blocked arc is waiting at a gate to reject.
	if arc.Status != adh.StatusBlocked {
		return fmt.Errorf("reject: arc %s is not waiting at a gate (status %s)", id, arc.Status)
	}
	reverted := cfg.revert(ctx, arc.Paths)
	returnToExecution(&arc)
	arc.History = append(arc.History, cfg.rejectNote(reverted))
	if err := store.Save(&arc); err != nil {
		return fmt.Errorf("reject: %w", err)
	}
	if cfg.JSONL {
		if err := cfg.EmitOK(map[string]any{
			"arc":      id,
			"status":   "rejected",
			"reverted": reverted,
			"stage":    string(arc.Stage),
		}); err != nil {
			return fmt.Errorf("reject: %w", err)
		}
		return nil
	}
	_, _ = fmt.Fprintf(cfg.Stdout, "rejected %s; returned to execution\n", id)
	return nil
}

// revert undoes the arc's working-tree changes, restoring paths to HEAD. It is
// best-effort: outside a git repo (or with no footprint) there is nothing to undo,
// and a revert error is a surfaced warning, not a failed reject — the arc still
// returns to Execution. It reports whether the working tree was reverted.
func (cfg *Config) revert(ctx context.Context, paths []string) bool {
	if len(paths) == 0 {
		return false
	}
	repo, err := vcs.Open(cfg.repoDir())
	if err != nil {
		return false // no git repo here — nothing to undo
	}
	if err := repo.Revert(paths); err != nil {
		cfg.Log.WarnContext(ctx, "revert skipped", "op", "reject", "err", err)
		return false
	}
	return true
}

// returnToExecution sends the arc back to the start of the loop, open for rework,
// and clears the footprint of the rejected attempt: its pending turn, findings,
// and routing footprint (paths/labels). Re-execution re-derives them.
func returnToExecution(arc *adh.Arc) {
	arc.Stage = adh.StageExecution
	arc.Status = adh.StatusOpen
	arc.Pending = nil
	arc.Findings = nil
	arc.Paths = nil
	arc.Labels = nil
}

// rejectNote records the disposition for the arc's history: the reason (if given)
// and whether the working tree was reverted.
func (cfg *Config) rejectNote(reverted bool) string {
	note := "gate rejected"
	if cfg.Reason != "" {
		note += ": " + cfg.Reason
	}
	if reverted {
		note += " (reverted)"
	}
	return note + "; returned to execution"
}

// repoDir is the repository root — the --repo global, or the current directory.
func (cfg *Config) repoDir() string {
	if cfg.Repo != "" {
		return cfg.Repo
	}
	return "."
}
