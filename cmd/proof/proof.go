// Package proof implements the "proof" CLI command with real nested subcommands:
// `create` hashes an arc's artifacts into a manifest and records it on the arc,
// and `verify` checks a manifest under NO-PROOF-NO-CLOSE (SPEC §5.4). Exit 8 on a
// verification failure.
package proof

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/agentic-dev-harness/cmd/root"
	prooflib "github.com/StevenACoffman/agentic-dev-harness/internal/proof"
	"github.com/StevenACoffman/agentic-dev-harness/internal/state"
	"github.com/StevenACoffman/agentic-dev-harness/internal/vcs"
)

// shortSHALen is how many leading hex characters of a provenance SHA the create
// summary prints.
const shortSHALen = 12

// Config holds the configuration for the proof command and its subcommands.
type Config struct {
	*root.Config
	Out     string
	Flags   *ff.FlagSet
	Command *ff.Command
}

// New creates and registers the proof command (with create/verify subcommands) on
// the given parent config.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("proof").SetParent(parent.Flags)
	cfg.Command = &ff.Command{
		Name:      "proof",
		Usage:     "agentic-dev-harness proof <create|verify> [flags] ...",
		ShortHelp: "create and verify an arc's proof packet (NO-PROOF-NO-CLOSE)",
		LongHelp: "Create a proof packet by hashing an arc's artifacts and recording it " +
			"on the arc, or verify that a manifest's artifacts exist and match their " +
			"digests (SPEC §5.4).",
		Flags: cfg.Flags,
	}
	cfg.addCreate()
	cfg.addVerify()
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

func (cfg *Config) addCreate() {
	fs := ff.NewFlagSet("create").SetParent(cfg.Flags)
	fs.StringVar(&cfg.Out, 'o', "out", "",
		"manifest output path (default .adh/proof/<arc>.json)")
	cfg.Command.Subcommands = append(cfg.Command.Subcommands, &ff.Command{
		Name:      "create",
		Usage:     "agentic-dev-harness proof create [--out path] <arc-id> <path>...",
		ShortHelp: "hash an arc's artifacts into a proof packet",
		LongHelp: "Hash each artifact (identity.Hash, sha256[:16]) into a manifest, write " +
			"it (default .adh/proof/<arc>.json), record it on the arc, and verify it.",
		Flags: fs,
		Exec:  func(_ context.Context, args []string) error { return cfg.create(args) },
	})
}

func (cfg *Config) addVerify() {
	fs := ff.NewFlagSet("verify").SetParent(cfg.Flags)
	cfg.Command.Subcommands = append(cfg.Command.Subcommands, &ff.Command{
		Name:      "verify",
		Usage:     "agentic-dev-harness proof verify <manifest.json>",
		ShortHelp: "verify a proof packet (exit 8 on failure)",
		LongHelp:  "Verify a manifest: every declared artifact exists and matches its digest (SPEC §5.4).",
		Flags:     fs,
		Exec:      func(_ context.Context, args []string) error { return cfg.verify(args) },
	})
}

// create hashes the arc's artifacts into a manifest, records the manifest path on
// the arc, and prints where it landed. It verifies the packet it just wrote so a
// bad manifest never advances past the generator.
func (cfg *Config) create(args []string) error {
	if len(args) < 2 {
		return errors.New("proof: create requires an arc id and at least one artifact path")
	}
	arcID, paths := args[0], args[1:]
	store := state.Default()
	arc, err := store.Get(arcID)
	if err != nil {
		return fmt.Errorf("proof: %w", err)
	}
	repoRoot := cfg.repoDir()
	pkt, err := prooflib.Create(repoRoot, arc.ID, headSHA(repoRoot), paths)
	if err != nil {
		return fmt.Errorf("proof: %w", err)
	}
	out := cfg.Out
	if out == "" {
		out = filepath.Join(".adh", "proof", arc.ID+".json")
	}
	if err := prooflib.Save(out, &pkt); err != nil {
		return fmt.Errorf("proof: %w", err)
	}
	if err := prooflib.Verify(repoRoot, &pkt); err != nil {
		return fmt.Errorf("proof: %w", err)
	}
	arc.Proof = out
	if err := store.Save(&arc); err != nil {
		return fmt.Errorf("proof: %w", err)
	}
	if cfg.JSONL {
		data := map[string]any{"arc": arc.ID, "artifacts": len(pkt.Artifacts), "manifest": out}
		if pkt.Provenance != nil && pkt.Provenance.GitSHA != "" {
			data["git_sha"] = pkt.Provenance.GitSHA
		}
		if err := cfg.EmitOK(data); err != nil {
			return fmt.Errorf("proof: %w", err)
		}
		return nil
	}
	_, _ = fmt.Fprintf(cfg.Stdout,
		"proof created: %d artifacts for %s at %s%s\n",
		len(pkt.Artifacts), arc.ID, out, provenanceNote(&pkt))
	return nil
}

// headSHA resolves the current commit for proof provenance (SPEC §5.4),
// best-effort: outside a git repo, or before the first commit, it returns "" and
// the packet simply records no provenance. Provenance is informational, so a
// resolution failure never fails the create.
func headSHA(repoRoot string) string {
	repo, err := vcs.Open(repoRoot)
	if err != nil {
		return ""
	}
	sha, err := repo.HeadSHA()
	if err != nil {
		return ""
	}
	return sha
}

// provenanceNote renders the short git SHA for the create summary, or "" when the
// packet recorded no provenance.
func provenanceNote(pkt *prooflib.Packet) string {
	if pkt.Provenance == nil || pkt.Provenance.GitSHA == "" {
		return ""
	}
	sha := pkt.Provenance.GitSHA
	if len(sha) > shortSHALen {
		sha = sha[:shortSHALen]
	}
	return " (git " + sha + ")"
}

func (cfg *Config) verify(args []string) error {
	if len(args) == 0 {
		return errors.New("proof: verify requires a manifest path")
	}
	pkt, err := prooflib.Load(args[0])
	if err != nil {
		return fmt.Errorf("proof: %w", err)
	}
	if verifyErr := prooflib.Verify(cfg.repoDir(), &pkt); verifyErr != nil {
		if cfg.JSONL {
			if err := cfg.EmitError(
				8,
				root.ReasonProof,
				"proof failed: "+verifyErr.Error(),
			); err != nil {
				return fmt.Errorf("proof: %w", err)
			}
		} else {
			_, _ = fmt.Fprintf(cfg.Stderr, "proof failed: %s\n", verifyErr)
		}
		return root.ExitError(8)
	}
	if cfg.JSONL {
		if err := cfg.EmitOK(map[string]any{
			"arc":       pkt.Arc,
			"artifacts": len(pkt.Artifacts),
			"verified":  true,
		}); err != nil {
			return fmt.Errorf("proof: %w", err)
		}
		return nil
	}
	_, _ = fmt.Fprintf(cfg.Stdout,
		"proof verified: %d artifacts for %s\n", len(pkt.Artifacts), pkt.Arc)
	return nil
}

// repoDir is the root the packet's paths resolve against — the --repo global, or
// the current directory.
func (cfg *Config) repoDir() string {
	if cfg.Repo != "" {
		return cfg.Repo
	}
	return "."
}
