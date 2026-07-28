// Package closecmd implements the "close" CLI command: ship an arc that is
// waiting, approved, at the ops gate. Close is the single mutation that ends an
// arc, and it enforces NO-PROOF-NO-CLOSE (SPEC §5.4): the arc's resolution must
// carry matching, verified proof (§12). Exit 8 on a proof failure.
package closecmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/agentic-dev-harness/cmd/root"
	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/atomicfile"
	metricslib "github.com/StevenACoffman/agentic-dev-harness/internal/metrics"
	prooflib "github.com/StevenACoffman/agentic-dev-harness/internal/proof"
	"github.com/StevenACoffman/agentic-dev-harness/internal/state"
	"github.com/StevenACoffman/agentic-dev-harness/internal/vcs"
)

// metricsFile is the effectiveness ledger the metrics command summarizes.
const metricsFile = ".adh/metrics.json"

// Config holds the configuration for the close command.
type Config struct {
	*root.Config
	As      string
	Proof   string
	Flags   *ff.FlagSet
	Command *ff.Command
}

// New creates and registers the close command with the given parent config.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("close").SetParent(parent.Flags)
	cfg.Flags.StringVar(&cfg.As, 'a', "as", "", "the resolution the arc closed as (§12)")
	cfg.Flags.StringVar(&cfg.Proof, 'p', "proof", "", "path to the proof packet manifest")
	cfg.Command = &ff.Command{
		Name:      "close",
		Usage:     "agentic-dev-harness close [--as res] [--proof manifest] <arc-id>",
		ShortHelp: "ship an approved arc under NO-PROOF-NO-CLOSE",
		LongHelp:  "Close an arc waiting at the ops gate: its resolution must carry matching verified proof (SPEC §5.4). Exit 8 on a proof failure.",
		Flags:     cfg.Flags,
		Exec:      cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

func (cfg *Config) exec(_ context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("close: requires an arc id")
	}
	id := args[0]
	store := state.Default()
	arc, err := store.Get(id)
	if err != nil {
		return fmt.Errorf("close: %w", err)
	}
	if err := readyToClose(&arc); err != nil {
		return err
	}
	if err := cfg.applyResolution(&arc); err != nil {
		return err
	}
	hasProof, err := cfg.verifyProof(&arc)
	if err != nil {
		return err
	}
	// NO-PROOF-NO-CLOSE (§5.4): a missing/mismatched proof is ECONFLICT → exit 8;
	// an unset resolution is EINVALID → a plain error.
	if err := adh.CanClose(&arc, hasProof); err != nil {
		if adh.ErrorCode(err) == adh.ECONFLICT {
			_, _ = fmt.Fprintf(cfg.Stderr, "close: %s\n", err)
			return root.ExitError(8)
		}
		return fmt.Errorf("close: %w", err)
	}
	// The gates have passed (approved at ops + verified proof), so the ship is the
	// irreversible VCS mutation — commit the change (§SPEC 5.2, §12).
	cfg.ship(&arc)
	arc.Status = adh.StatusClosed
	arc.History = append(arc.History, "closed as "+string(arc.Resolution))
	if err := store.Save(&arc); err != nil {
		return fmt.Errorf("close: %w", err)
	}
	// Effectiveness accounting (§16): record the closed arc's cost. The ship is
	// authoritative; a metrics-write failure is surfaced, not fatal.
	if err := recordMetric(&arc); err != nil {
		_, _ = fmt.Fprintf(cfg.Stderr, "close: warning: metrics not recorded: %s\n", err)
	}
	_, _ = fmt.Fprintf(cfg.Stdout, "closed %s as %s\n", id, arc.Resolution)
	return nil
}

// recordMetric appends the closed arc's cost to the effectiveness ledger. The
// attention and compute figures are deterministic proxies (history length and
// bytes) until real telemetry lands; a closed arc counts as accepted.
func recordMetric(arc *adh.Arc) error {
	records, err := loadMetrics()
	if err != nil {
		return err
	}
	tokens := 0
	for _, entry := range arc.History {
		tokens += len(entry)
	}
	records = append(records, metricslib.Record{
		Arc:              arc.ID,
		AttentionMinutes: len(arc.History),
		ComputeTokens:    tokens,
		Accepted:         true,
	})
	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return &adh.Error{Op: "close.recordMetric", Err: err}
	}
	if err := os.MkdirAll(filepath.Dir(metricsFile), 0o750); err != nil {
		return fmt.Errorf("close: %w", err)
	}
	if err := atomicfile.WriteFile(metricsFile, append(data, '\n'), 0o600); err != nil {
		return fmt.Errorf("close: write metrics: %w", err)
	}
	return nil
}

func loadMetrics() ([]metricslib.Record, error) {
	data, err := os.ReadFile(metricsFile)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("close: %w", err)
	}
	var records []metricslib.Record
	if err := json.Unmarshal(data, &records); err != nil {
		return nil, fmt.Errorf("close: %w", err)
	}
	return records, nil
}

// readyToClose checks the arc is parked, approved, at the ops gate. A blocked
// arc must be approved first; only an open arc at ops may ship.
func readyToClose(arc *adh.Arc) error {
	if arc.Stage != adh.StageOps {
		return fmt.Errorf("close: arc %s is at %s, not the ops gate", arc.ID, arc.Stage)
	}
	switch arc.Status {
	case adh.StatusOpen:
		return nil
	case adh.StatusBlocked:
		return fmt.Errorf("close: arc %s is blocked; approve the ops gate first", arc.ID)
	case adh.StatusClosed, adh.StatusFailed:
		return fmt.Errorf("close: arc %s is %s, not open", arc.ID, arc.Status)
	default:
		return fmt.Errorf("close: arc %s is %s, not open", arc.ID, arc.Status)
	}
}

// applyResolution overrides the arc's resolution with --as when given (the
// operator declares how the arc actually closed), validating it first.
func (cfg *Config) applyResolution(arc *adh.Arc) error {
	if cfg.As == "" {
		return nil
	}
	res, err := adh.ParseResolution(cfg.As)
	if err != nil {
		return fmt.Errorf("close: %w", err)
	}
	arc.Resolution = res
	return nil
}

// verifyProof loads and verifies the proof packet, reporting whether matching
// proof is present. The manifest is --proof, or the path `adh proof create`
// recorded on the arc (Arc.Proof) when the flag is omitted. A declared-but-failing
// packet is itself a proof failure (exit 8); no packet means no proof.
func (cfg *Config) verifyProof(arc *adh.Arc) (bool, error) {
	manifest := cfg.Proof
	if manifest == "" {
		manifest = arc.Proof
	}
	if manifest == "" {
		return false, nil
	}
	pkt, err := prooflib.Load(manifest)
	if err != nil {
		return false, fmt.Errorf("close: %w", err)
	}
	if verifyErr := prooflib.Verify(cfg.repoDir(), &pkt); verifyErr != nil {
		_, _ = fmt.Fprintf(cfg.Stderr, "close: proof failed: %s\n", verifyErr)
		return false, root.ExitError(8)
	}
	return true, nil
}

// ship commits a `change` arc's work — the irreversible action the ops gate
// protects. It runs only past the approval + proof gates, so the commit is gated.
// It is best-effort: outside a git repo the arc closes without a commit (silently,
// since not every workspace is under git), and a commit error (e.g. nothing to
// commit) is a surfaced warning, not a failed close. Non-`change` resolutions
// carry no code commit.
func (cfg *Config) ship(arc *adh.Arc) {
	if arc.Resolution != adh.ResolutionChange {
		return
	}
	repo, err := vcs.Open(cfg.repoDir())
	if err != nil {
		return // no git repo here — nothing to commit
	}
	who := vcs.Signature{Name: "adh", Email: "adh@localhost"}
	hash, err := repo.Commit(arc.ID+": "+arc.Title, who, time.Now())
	if err != nil {
		_, _ = fmt.Fprintf(cfg.Stderr, "close: warning: commit skipped: %s\n", err)
		return
	}
	arc.History = append(arc.History, "committed "+hash)
	_, _ = fmt.Fprintf(cfg.Stdout, "committed %s as %s\n", arc.ID, hash)
}

// repoDir is the repository root — the --repo global, or the current directory.
func (cfg *Config) repoDir() string {
	if cfg.Repo != "" {
		return cfg.Repo
	}
	return "."
}
