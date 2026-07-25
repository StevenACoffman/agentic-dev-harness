// Package proof implements the "proof" CLI command: verify an arc's proof
// packet under NO-PROOF-NO-CLOSE (SPEC §5.4). Exit 8 on a proof failure.
package proof

import (
	"context"
	"errors"
	"fmt"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/agentic-dev-harness/cmd/root"
	prooflib "github.com/StevenACoffman/agentic-dev-harness/internal/proof"
)

// Config holds the configuration for the proof command.
type Config struct {
	*root.Config
	Flags   *ff.FlagSet
	Command *ff.Command
}

// New creates and registers the proof command with the given parent config.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("proof").SetParent(parent.Flags)
	cfg.Command = &ff.Command{
		Name:      "proof",
		Usage:     "agentic-dev-harness proof verify <manifest.json>",
		ShortHelp: "verify an arc's proof packet (NO-PROOF-NO-CLOSE)",
		LongHelp:  "Verify a proof packet: every declared artifact exists and matches its digest (SPEC §5.4).",
		Flags:     cfg.Flags,
		Exec:      cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

func (cfg *Config) exec(_ context.Context, args []string) error {
	if len(args) == 0 || args[0] != "verify" {
		return errors.New("proof: expected 'verify <manifest.json>'")
	}
	if len(args) < 2 {
		return errors.New("proof: verify requires a manifest path")
	}
	pkt, err := prooflib.Load(args[1])
	if err != nil {
		return fmt.Errorf("proof: %w", err)
	}
	if verifyErr := prooflib.Verify(".", &pkt); verifyErr != nil {
		_, _ = fmt.Fprintf(cfg.Stderr, "proof failed: %s\n", verifyErr)
		return root.ExitError(8)
	}
	_, _ = fmt.Fprintf(
		cfg.Stdout,
		"proof verified: %d artifacts for %s\n",
		len(pkt.Artifacts),
		pkt.Arc,
	)
	return nil
}
