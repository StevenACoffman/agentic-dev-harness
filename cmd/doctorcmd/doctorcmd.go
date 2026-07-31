// Package doctorcmd implements the "doctor" CLI command: the harness-integrity
// self-check (SPEC-ADDITIONS §10.4). It loads the context store, tool registry,
// loop registry, and NFR specs and asks internal/harnesscheck whether the harness
// is intact and consistent — every registry valid, unit ids unique, and every
// cross-reference resolving. It is the cheap, high-trust check a session start or
// the harness-integrity §15 loop runs.
package doctorcmd

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/agentic-dev-harness/cmd/root"
	"github.com/StevenACoffman/agentic-dev-harness/internal/contextstore"
	"github.com/StevenACoffman/agentic-dev-harness/internal/harnesscheck"
	looplib "github.com/StevenACoffman/agentic-dev-harness/internal/loop"
	"github.com/StevenACoffman/agentic-dev-harness/internal/nfr"
	"github.com/StevenACoffman/agentic-dev-harness/internal/toolreg"
)

// integrityCode is the exit code for a harness-integrity failure (SPEC §7).
const integrityCode = 16

// Config holds the configuration for the doctor command.
type Config struct {
	*root.Config
	Flags   *ff.FlagSet
	Command *ff.Command
}

// New creates and registers the doctor command with the given parent config.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("doctor").SetParent(parent.Flags)
	cfg.Command = &ff.Command{
		Name:      "doctor",
		Usage:     "agentic-dev-harness doctor",
		ShortHelp: "check that the harness is intact and consistent",
		LongHelp: "Run the harness-integrity self-check (SPEC-ADDITIONS §10.4): every registry " +
			"is structurally valid, unit ids are unique, NFR specs are well-formed, and every " +
			"cross-reference resolves (a unit's integrity check names a declared §13 tool). " +
			"Exits non-zero when the harness is inconsistent, so a session start or the " +
			"harness-integrity loop can gate on it.",
		Flags: cfg.Flags,
		Exec:  cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

func (cfg *Config) exec(_ context.Context, _ []string) error {
	in, err := cfg.load()
	if err != nil {
		return err
	}
	return cfg.report(harnesscheck.Check(&in))
}

// load gathers the harness state the check reasons over from the repo root — the
// context store, the tool and loop registries, and the NFR specs.
func (cfg *Config) load() (harnesscheck.Inputs, error) {
	repo := cfg.repoDir()
	units, err := contextstore.Load(filepath.Join(repo, contextstore.DefaultStoreDir))
	if err != nil {
		return harnesscheck.Inputs{}, fmt.Errorf("doctor: %w", err)
	}
	tools, err := toolreg.LoadRepo(repo)
	if err != nil {
		return harnesscheck.Inputs{}, fmt.Errorf("doctor: %w", err)
	}
	loops, err := looplib.Load(filepath.Join(repo, looplib.DefaultRegistryFile))
	if err != nil {
		return harnesscheck.Inputs{}, fmt.Errorf("doctor: %w", err)
	}
	specs, err := nfr.Load(filepath.Join(repo, nfr.DefaultDir))
	if err != nil {
		return harnesscheck.Inputs{}, fmt.Errorf("doctor: %w", err)
	}
	return harnesscheck.Inputs{Units: units, Tools: tools, Loops: loops, Specs: specs}, nil
}

// report emits the self-check result: under --jsonl one outcome carrying the
// problems (an error outcome when any exist), else a line per problem. It exits
// integrityCode when the harness is inconsistent.
func (cfg *Config) report(problems []harnesscheck.Problem) error {
	if cfg.JSONL {
		if err := cfg.emit(problems); err != nil {
			return err
		}
	} else {
		cfg.print(problems)
	}
	if len(problems) > 0 {
		return root.ExitError(integrityCode)
	}
	return nil
}

// emit writes the --jsonl outcome: an error outcome carrying the problems when the
// harness is inconsistent, an ok one otherwise.
func (cfg *Config) emit(problems []harnesscheck.Problem) error {
	status, reason, code := root.StatusOK, "", 0
	if len(problems) > 0 {
		status, reason, code = root.StatusError, "harness_integrity", integrityCode
	}
	if err := cfg.EmitJSONL(root.Outcome{
		Status: status, Code: code, Reason: reason,
		Data: map[string]any{"problems": problems},
	}); err != nil {
		return fmt.Errorf("doctor: %w", err)
	}
	return nil
}

// print writes the human self-check result, a line per problem.
func (cfg *Config) print(problems []harnesscheck.Problem) {
	if len(problems) == 0 {
		_, _ = fmt.Fprintln(cfg.Stdout, "harness intact: all checks pass")
		return
	}
	for i := range problems {
		_, _ = fmt.Fprintf(cfg.Stderr, "%s\t%s\t%s\n",
			problems[i].Kind, problems[i].Ref, problems[i].Detail)
	}
	_, _ = fmt.Fprintf(cfg.Stderr, "%d harness-integrity problem(s)\n", len(problems))
}

// repoDir is the repository root — the --repo global, or the current directory.
func (cfg *Config) repoDir() string {
	if cfg.Repo != "" {
		return cfg.Repo
	}
	return "."
}
