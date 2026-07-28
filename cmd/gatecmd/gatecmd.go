// Package gatecmd implements the "gate" CLI command (SPEC §2.3): list the pending
// human gates across arcs — the arcs parked blocked, awaiting an approve/reject.
// The self-optimization ratchet lives under `harness gate`; this is the human gate.
package gatecmd

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/agentic-dev-harness/cmd/root"
	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/state"
)

// pendingGate is the machine-readable record for one blocked arc under --jsonl.
type pendingGate struct {
	Arc    string `json:"arc"`
	Stage  string `json:"stage"`
	Reason string `json:"reason"`
}

// Config holds the configuration for the gate command.
type Config struct {
	*root.Config
	Flags   *ff.FlagSet
	Command *ff.Command
}

// New creates and registers the gate command with the given parent config.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("gate").SetParent(parent.Flags)
	cfg.Command = &ff.Command{
		Name:      "gate",
		Usage:     "agentic-dev-harness gate list",
		ShortHelp: "list pending human gates across arcs",
		LongHelp:  "List the arcs parked at a human gate (blocked), awaiting approve/reject (SPEC §2.3, §5.2).",
		Flags:     cfg.Flags,
		Exec:      cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

func (cfg *Config) exec(_ context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("gate: expected a verb: list")
	}
	switch args[0] {
	case "list":
		return cfg.list()
	default:
		return fmt.Errorf("gate: unknown verb %q; want list", args[0])
	}
}

func (cfg *Config) list() error {
	arcs, err := state.Default().List()
	if err != nil {
		return fmt.Errorf("gate: %w", err)
	}
	pending := 0
	for i := range arcs {
		arc := &arcs[i]
		if arc.Status != adh.StatusBlocked {
			continue
		}
		pending++
		if cfg.JSONL {
			if err := cfg.EmitOK(pendingGate{
				Arc:    arc.ID,
				Stage:  string(arc.Stage),
				Reason: gateReason(arc),
			}); err != nil {
				return fmt.Errorf("gate: %w", err)
			}
			continue
		}
		_, _ = fmt.Fprintf(cfg.Stdout, "%s\t%s\t%s\n", arc.ID, arc.Stage, gateReason(arc))
	}
	// In JSONL mode, no gates is an empty stream (zero lines); the human path says so.
	if pending == 0 && !cfg.JSONL {
		_, _ = fmt.Fprintln(cfg.Stdout, "no pending gates")
	}
	return nil
}

// gateReason recovers why an arc is blocked from the last "blocked: ..." history
// entry the run/relay loop records, or a default when none is present.
func gateReason(arc *adh.Arc) string {
	for i := len(arc.History) - 1; i >= 0; i-- {
		if reason, ok := strings.CutPrefix(arc.History[i], "blocked: "); ok {
			return reason
		}
	}
	return "awaiting approval"
}
