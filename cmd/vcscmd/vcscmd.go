// Package vcscmd implements the "vcs" CLI command: inspect and mutate the git
// working tree through the internal/vcs seam (go-git). Verbs: status, branch
// <name>, commit -m <msg>. It is the first consumer of the adapter; wiring
// branch/commit into the arc lifecycle (ops/close) is a follow-up.
package vcscmd

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/agentic-dev-harness/cmd/root"
	"github.com/StevenACoffman/agentic-dev-harness/internal/vcs"
)

// worktreeDir is the repository the command operates on: the working directory.
const worktreeDir = "."

// Config holds the configuration for the vcs command.
type Config struct {
	*root.Config
	Message string
	Flags   *ff.FlagSet
	Command *ff.Command
}

// New creates and registers the vcs command with the given parent config.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("vcs").SetParent(parent.Flags)
	cfg.Flags.StringVar(&cfg.Message, 'm', "message", "", "commit message")
	cfg.Command = &ff.Command{
		Name:      "vcs",
		Usage:     "agentic-dev-harness vcs [-m msg] <status|branch|commit> [name]",
		ShortHelp: "inspect and mutate the git working tree",
		LongHelp:  "Version control via go-git: status, branch <name>, commit -m <msg>.",
		Flags:     cfg.Flags,
		Exec:      cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

func (cfg *Config) exec(_ context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("vcs: expected a verb: status, branch, or commit")
	}
	repo, err := vcs.Open(worktreeDir)
	if err != nil {
		return fmt.Errorf("vcs: %w", err)
	}
	switch args[0] {
	case "status":
		return cfg.status(repo)
	case "branch":
		return cfg.branch(repo, args[1:])
	case "commit":
		return cfg.commit(repo)
	default:
		return fmt.Errorf("vcs: unknown verb %q; want status, branch, or commit", args[0])
	}
}

func (cfg *Config) status(repo *vcs.Git) error {
	state, err := repo.Status()
	if err != nil {
		return fmt.Errorf("vcs: %w", err)
	}
	tree := "clean"
	if !state.Clean {
		tree = "dirty"
	}
	_, _ = fmt.Fprintf(cfg.Stdout, "branch %s (%s)\n", state.Branch, tree)
	for _, path := range state.Changed {
		_, _ = fmt.Fprintf(cfg.Stdout, "  %s\n", path)
	}
	return nil
}

func (cfg *Config) branch(repo *vcs.Git, args []string) error {
	if len(args) == 0 {
		return errors.New("vcs: branch requires a name")
	}
	if err := repo.CreateBranch(args[0]); err != nil {
		return fmt.Errorf("vcs: %w", err)
	}
	_, _ = fmt.Fprintf(cfg.Stdout, "on branch %s\n", args[0])
	return nil
}

func (cfg *Config) commit(repo *vcs.Git) error {
	if cfg.Message == "" {
		return errors.New("vcs: commit requires -m <message>")
	}
	who := vcs.Signature{Name: "adh", Email: "adh@localhost"}
	hash, err := repo.Commit(cfg.Message, who, time.Now())
	if err != nil {
		return fmt.Errorf("vcs: %w", err)
	}
	_, _ = fmt.Fprintf(cfg.Stdout, "committed %s\n", hash)
	return nil
}
