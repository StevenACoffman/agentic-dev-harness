// Package loopcmd implements the "loop" CLI command: list, run, tick, and retire
// maintenance loops (SPEC-ADDITIONS §15). `run` senses one loop's invariant and
// `tick` sweeps every loop; on a departure each opens an arc under the loop's
// authority — the connection from a maintenance loop to the arc loop an agent drives.
package loopcmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/agentic-dev-harness/cmd/root"
	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	looplib "github.com/StevenACoffman/agentic-dev-harness/internal/loop"
	"github.com/StevenACoffman/agentic-dev-harness/internal/shell"
	"github.com/StevenACoffman/agentic-dev-harness/internal/state"
)

// onFindingOpenArc is the authorized action that opens an arc when the sensor
// reports a departure (§15 `on_finding`).
const onFindingOpenArc = "open arc"

// sensorRunner runs a loop's sensor and reports whether it found a departure from
// the invariant (a non-zero exit). It is the point-of-use seam: shellSensor is the
// real one; a test injects a fake so no process is spawned.
type sensorRunner interface {
	Sense(ctx context.Context, command, dir string) (finding bool)
}

// shellSensor runs the sensor as `sh -c <command>` in dir through the shared
// internal/shell edge; a non-zero exit is a finding. The command is a
// repository-owned loop-registry entry, never model input.
type shellSensor struct{}

// Config holds the configuration for the loop command.
type Config struct {
	*root.Config
	sensor  sensorRunner
	Flags   *ff.FlagSet
	Command *ff.Command
}

// tickResult is one loop's outcome in a tick sweep: the loop id, the arc opened (if
// any), and a human message.
type tickResult struct {
	Loop    string `json:"loop"`
	Arc     string `json:"arc,omitempty"`
	Message string `json:"message"`
}

// Sense runs command via the shell in dir; a non-zero exit (or an unstartable
// command) means the invariant departed (a finding).
func (shellSensor) Sense(ctx context.Context, command, dir string) bool {
	code, ran := shell.Runner{}.Run(ctx, command, dir)
	return !ran || code != 0
}

// New creates and registers the loop command with the given parent config.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.sensor = shellSensor{}
	cfg.Flags = ff.NewFlagSet("loop").SetParent(parent.Flags)
	cfg.Command = &ff.Command{
		Name:      "loop",
		Usage:     "agentic-dev-harness loop <list|run|tick|retire> [id]",
		ShortHelp: "list, run, tick, and retire maintenance loops",
		LongHelp: "Manage maintenance loops (SPEC-ADDITIONS §15): list them, run one, tick all " +
			"(the session-start sweep that fires every standing loop), or retire one.",
		Flags: cfg.Flags,
		Exec:  cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

func (cfg *Config) exec(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("loop: expected a verb: list, run, tick, or retire")
	}
	reg, err := looplib.Load(looplib.DefaultRegistryFile)
	if err != nil {
		return fmt.Errorf("loop: %w", err)
	}
	switch args[0] {
	case "list":
		return cfg.list(reg)
	case "run":
		return cfg.run(ctx, reg, args[1:])
	case "tick":
		return cfg.tick(ctx, reg)
	case "retire":
		return cfg.retire(reg, args[1:])
	default:
		return fmt.Errorf("loop: unknown verb %q; want list, run, tick, or retire", args[0])
	}
}

func (cfg *Config) list(reg looplib.Registry) error {
	if err := reg.Validate(); err != nil {
		return fmt.Errorf("loop: %w", err)
	}
	for i := range reg.Loops {
		l := &reg.Loops[i]
		_, _ = fmt.Fprintf(cfg.Stdout, "%s\t%s\n", l.ID, l.Goal)
	}
	return nil
}

// run senses one loop's invariant and, on a departure whose action is "open arc",
// opens an arc under the loop's authority for an agent to drive.
func (cfg *Config) run(ctx context.Context, reg looplib.Registry, args []string) error {
	loop, err := requireLoop(reg, args)
	if err != nil {
		return err
	}
	arcID, message, err := cfg.tickOne(ctx, &loop)
	if err != nil {
		return err
	}
	return cfg.reportRun(loop.ID, arcID, message)
}

// tick sweeps every registered loop once — the maintenance heartbeat a session
// start runs so accretion fires without being asked (§15). It senses each loop and
// opens an arc for every departure, reporting the sweep. A finding is queued work,
// not an error, so tick never fails on one.
func (cfg *Config) tick(ctx context.Context, reg looplib.Registry) error {
	if err := reg.Validate(); err != nil {
		return fmt.Errorf("loop: %w", err)
	}
	results := make([]tickResult, 0, len(reg.Loops))
	opened := 0
	for i := range reg.Loops {
		arcID, message, err := cfg.tickOne(ctx, &reg.Loops[i])
		if err != nil {
			return err
		}
		if arcID != "" {
			opened++
		}
		results = append(results, tickResult{Loop: reg.Loops[i].ID, Arc: arcID, Message: message})
	}
	return cfg.reportTick(results, opened)
}

// tickOne senses one loop and, on a departure whose action is "open arc", opens an
// arc under the loop's authority. It returns the opened arc id (empty when none) and
// a human message. It is the unit run (one loop) and tick (all loops) share.
func (cfg *Config) tickOne(
	ctx context.Context,
	loop *looplib.Loop,
) (arcID, message string, err error) {
	if !cfg.sensor.Sense(ctx, loop.Sensor, cfg.repoDir()) {
		return "", "invariant holds", nil
	}
	if loop.OnFinding != onFindingOpenArc {
		return "", "finding; no arc opened (on_finding: " + loop.OnFinding + ")", nil
	}
	arc, err := cfg.openArc(loop)
	if err != nil {
		return "", "", err
	}
	return arc.ID, "opened arc " + arc.ID, nil
}

// openArc creates an open arc for the loop's goal under its owner's routing label,
// so the sensed departure becomes work an agent can drive.
func (cfg *Config) openArc(loop *looplib.Loop) (adh.Arc, error) {
	store := state.Default()
	id, err := store.NextID()
	if err != nil {
		return adh.Arc{}, fmt.Errorf("loop: %w", err)
	}
	arc := adh.Arc{ID: id, Title: loop.Goal, Stage: adh.StageStrategy, Status: adh.StatusOpen}
	if loop.Owner != "" {
		arc.Labels = []string{loop.Owner}
	}
	if err := store.Create(&arc); err != nil {
		return adh.Arc{}, fmt.Errorf("loop: %w", err)
	}
	return arc, nil
}

func (cfg *Config) retire(reg looplib.Registry, args []string) error {
	loop, err := requireLoop(reg, args)
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(cfg.Stdout, "retired %s (retire when: %s)\n", loop.ID, loop.RetireWhen)
	return nil
}

// reportTick emits the sweep outcome: under --jsonl one success outcome carrying
// the per-loop results and the count opened, else a line per loop and a summary.
func (cfg *Config) reportTick(results []tickResult, opened int) error {
	if cfg.JSONL {
		if err := cfg.EmitOK(map[string]any{
			"loops": len(results), "arcs_opened": opened, "results": results,
		}); err != nil {
			return fmt.Errorf("loop: %w", err)
		}
		return nil
	}
	for i := range results {
		_, _ = fmt.Fprintf(cfg.Stdout, "%s\t%s\n", results[i].Loop, results[i].Message)
	}
	_, _ = fmt.Fprintf(cfg.Stdout, "swept %d loop(s), opened %d arc(s)\n", len(results), opened)
	return nil
}

// reportRun emits a loop-run outcome: a success outcome carrying the loop, the
// opened arc (if any), and the message under --jsonl, else the message.
func (cfg *Config) reportRun(loopID, arcID, message string) error {
	if cfg.JSONL {
		data := map[string]string{"loop": loopID, "message": message}
		if arcID != "" {
			data["arc"] = arcID
		}
		if err := cfg.EmitOK(data); err != nil {
			return fmt.Errorf("loop: %w", err)
		}
		return nil
	}
	_, _ = fmt.Fprintf(cfg.Stdout, "loop %s: %s\n", loopID, message)
	return nil
}

// requireLoop resolves the loop id argument run and retire share.
func requireLoop(reg looplib.Registry, args []string) (looplib.Loop, error) {
	if len(args) == 0 {
		return looplib.Loop{}, errors.New("loop: run and retire require a loop id")
	}
	loop, ok := reg.Find(args[0])
	if !ok {
		return looplib.Loop{}, fmt.Errorf("loop: no such loop %q", args[0])
	}
	return loop, nil
}

// repoDir is the repository root — the --repo global, or the current directory.
func (cfg *Config) repoDir() string {
	if cfg.Repo != "" {
		return cfg.Repo
	}
	return "."
}
