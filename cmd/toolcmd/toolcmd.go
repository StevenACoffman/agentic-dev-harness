// Package toolcmd implements the "tool" CLI command: inspect and invoke the tool
// registry (SPEC-ADDITIONS §13) — list declared tools, run doctor to validate
// them, or run one so the worker can invoke an external capability in-loop and
// interpret its output.
package toolcmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/agentic-dev-harness/cmd/root"
	"github.com/StevenACoffman/agentic-dev-harness/internal/shell"
	"github.com/StevenACoffman/agentic-dev-harness/internal/toolreg"
)

// Exit codes for the tool command (SPEC §7), kept in one family distinct from the
// arc gates (4, 5–8, 9, 12). A tool that ran and exited non-zero propagates its own
// code instead — the worker sees the real result.
const (
	codeRegistry = 10 // registry-level problem: invalid, unknown id, or uninstalled binary
)

// Config holds the configuration for the tool command.
type Config struct {
	*root.Config
	Flags   *ff.FlagSet
	Command *ff.Command
}

// New creates and registers the tool command with the given parent config.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("tool").SetParent(parent.Flags)
	cfg.Command = &ff.Command{
		Name:      "tool",
		Usage:     "agentic-dev-harness tool <list|doctor|run <id>>",
		ShortHelp: "list, check, and run the registered tools",
		LongHelp: "Inspect and invoke the tool registry (SPEC-ADDITIONS §13): list declared " +
			"tools, validate them with doctor, or `run <id>` to invoke an external " +
			"capability (exegesis/skillsaw/modelith) and interpret its output in-loop. " +
			"Under --jsonl a run's captured stdout/stderr and exit code are carried in " +
			"the outcome's data.",
		Flags: cfg.Flags,
		Exec:  cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

func (cfg *Config) exec(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("tool: expected a verb: list, doctor, or run")
	}
	reg, err := toolreg.LoadRepo(cfg.repoDir())
	if err != nil {
		return fmt.Errorf("tool: %w", err)
	}
	switch args[0] {
	case "list":
		return cfg.list(reg)
	case "doctor":
		return cfg.doctor(reg)
	case "run":
		return cfg.run(ctx, reg, args[1:])
	default:
		return fmt.Errorf("tool: unknown verb %q; want list, doctor, or run", args[0])
	}
}

// list prints each declared tool and what it verifies.
func (cfg *Config) list(reg toolreg.Registry) error {
	for _, tool := range reg.Tools {
		_, _ = fmt.Fprintf(cfg.Stdout, "%s\tverifies: %s\n", tool.ID, tool.Verifies)
	}
	return nil
}

// doctor validates the registry's structure (unique ids, required fields). An
// absent registry is a valid empty one, not a defect (toolreg.LoadRepo).
func (cfg *Config) doctor(reg toolreg.Registry) error {
	if verr := reg.Validate(); verr != nil {
		_, _ = fmt.Fprintf(cfg.Stderr, "tool registry invalid: %s\n", verr)
		return root.ExitError(codeRegistry)
	}
	_, _ = fmt.Fprintf(cfg.Stdout, "%d tools declared, registry valid\n", len(reg.Tools))
	return nil
}

// run invokes the declared tool named by args[0] and translates its result. It
// resolves the tool by id, runs its repository-owned Run command in the repo, and
// surfaces the tool's own exit code as the process exit code so the worker sees the
// real result. Under --jsonl the captured stdout/stderr and exit code ride in the
// outcome's data (one clean line); otherwise they stream live to the worker.
func (cfg *Config) run(ctx context.Context, reg toolreg.Registry, args []string) error {
	if len(args) == 0 {
		return errors.New("tool: run requires a tool id (see `tool list`)")
	}
	id := args[0]
	tool, ok := reg.FindByID(id)
	if !ok {
		return cfg.reportRunProblem(codeRegistry, "unknown_tool",
			fmt.Sprintf("no tool %q in the registry; see `adh tool list`", id))
	}
	if cfg.JSONL {
		return cfg.runCaptured(ctx, &tool)
	}
	return cfg.runStreamed(ctx, &tool)
}

// runStreamed runs the tool with its output wired live to the worker's streams and
// propagates its exit code. A command that could not start (an uninstalled binary)
// is a registry-level problem carrying the repair hint.
func (cfg *Config) runStreamed(ctx context.Context, tool *toolreg.Tool) error {
	exit, ran := shell.Runner{}.RunIO(ctx, tool.Run, cfg.repoDir(), cfg.Stdout, cfg.Stderr)
	if shell.NotRun(exit, ran) {
		return cfg.reportRunProblem(codeRegistry, "tool_unavailable", unavailableMessage(tool))
	}
	if exit != 0 {
		return root.ExitError(exit)
	}
	return nil
}

// runCaptured runs the tool capturing its output into the --jsonl outcome, so the
// worker parses one structured envelope carrying the exit code, stdout, and stderr.
func (cfg *Config) runCaptured(ctx context.Context, tool *toolreg.Tool) error {
	var stdout, stderr bytes.Buffer
	exit, ran := shell.Runner{}.RunIO(ctx, tool.Run, cfg.repoDir(), &stdout, &stderr)
	if shell.NotRun(exit, ran) {
		return cfg.reportRunProblem(codeRegistry, "tool_unavailable", unavailableMessage(tool))
	}
	status, reason := root.StatusOK, ""
	if exit != 0 {
		status, reason = root.StatusError, "tool_failed"
	}
	if emitErr := cfg.EmitJSONL(root.Outcome{
		Status: status,
		Code:   exit,
		Reason: reason,
		Data: map[string]any{
			"id":     tool.ID,
			"run":    tool.Run,
			"exit":   exit,
			"stdout": stdout.String(),
			"stderr": stderr.String(),
		},
	}); emitErr != nil {
		return fmt.Errorf("tool: %w", emitErr)
	}
	if exit != 0 {
		return root.ExitError(exit)
	}
	return nil
}

// reportRunProblem reports a run problem adh itself detected (an unknown or
// uninstalled tool) — a structured error outcome under --jsonl, a stderr line
// otherwise — and returns the exit code without a usage banner.
func (cfg *Config) reportRunProblem(code int, reason, message string) error {
	if cfg.JSONL {
		if emitErr := cfg.EmitError(code, reason, message); emitErr != nil {
			return fmt.Errorf("tool: %w", emitErr)
		}
	} else {
		_, _ = fmt.Fprintf(cfg.Stderr, "tool: %s\n", message)
	}
	return root.ExitError(code)
}

// unavailableMessage explains an uninstalled tool, appending its repair hint when
// the registry entry declares one.
func unavailableMessage(tool *toolreg.Tool) string {
	msg := fmt.Sprintf("tool %q could not run (is it installed?)", tool.ID)
	if tool.RepairHint != "" {
		msg += ": " + tool.RepairHint
	}
	return msg
}

// repoDir is the repository root the registry loads from and tools run in — the
// --repo global, or the current directory.
func (cfg *Config) repoDir() string {
	if cfg.Repo != "" {
		return cfg.Repo
	}
	return "."
}
