// Package sleep implements the "sleep" CLI command: the offline consolidation
// cycle (SPEC-ADDITIONS §18.4). Before trusting the loop it runs the
// negative-control gate self-test (a planted regression must be rejected);
// staged proposals await an explicit human adopt.
package sleep

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/agentic-dev-harness/cmd/root"
	"github.com/StevenACoffman/agentic-dev-harness/internal/evidence"
	"github.com/StevenACoffman/agentic-dev-harness/internal/harness"
)

const evidenceFile = ".adh/sleep/evidence.jsonl"

// Config holds the configuration for the sleep command.
type Config struct {
	*root.Config
	Flags   *ff.FlagSet
	Command *ff.Command
}

// New creates and registers the sleep command with the given parent config.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("sleep").SetParent(parent.Flags)
	cfg.Command = &ff.Command{
		Name:      "sleep",
		Usage:     "agentic-dev-harness sleep <run|adopt>",
		ShortHelp: "run the offline consolidation cycle",
		LongHelp:  "Run the offline self-optimization cycle behind a held-out gate with a negative-control self-test (SPEC-ADDITIONS §18.4).",
		Flags:     cfg.Flags,
		Exec:      cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

func (cfg *Config) exec(_ context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("sleep: expected a verb: run or adopt")
	}
	switch args[0] {
	case "run":
		return cfg.run()
	case "adopt":
		return errors.New("sleep: nothing staged to adopt")
	default:
		return fmt.Errorf("sleep: unknown verb %q; want run or adopt", args[0])
	}
}

func (cfg *Config) run() error {
	// The negative control proves the self-optimization gate has teeth before the
	// loop is trusted: a planted non-improving candidate must be rejected by the
	// ratchet (SPEC-ADDITIONS §18.4), mirroring SkillOpt's harmful-edit probe.
	if err := harness.SelfTest(); err != nil {
		_, _ = fmt.Fprintf(cfg.Stderr, "gate self-test failed: %s\n", err)
		return root.ExitError(15)
	}
	if err := appendEvidence(&evidence.Record{
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		Status:     evidence.StatusBaseline,
		GateAction: "reject",
		Note:       "mock backend proposed no strict held-out improvement",
	}); err != nil {
		return err
	}
	_, _ = fmt.Fprintln(
		cfg.Stdout,
		"gate self-test passed; mock backend proposed no strict held-out improvement; nothing staged",
	)
	return nil
}

// appendEvidence records one line of the night's evidentiary chain. The file
// open/append is the imperative shell around the pure evidence.Append.
func appendEvidence(rec *evidence.Record) error {
	if err := os.MkdirAll(filepath.Dir(evidenceFile), 0o750); err != nil {
		return fmt.Errorf("sleep: %w", err)
	}
	file, err := os.OpenFile(evidenceFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("sleep: %w", err)
	}
	defer func() { _ = file.Close() }()
	if err := evidence.Append(file, *rec); err != nil {
		return fmt.Errorf("sleep: %w", err)
	}
	return nil
}
