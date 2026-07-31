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
	"github.com/StevenACoffman/agentic-dev-harness/internal/adr"
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

func (cfg *Config) exec(ctx context.Context, args []string) error {
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
			return cfg.proofFail(err.Error())
		}
		return fmt.Errorf("close: %w", err)
	}
	// The gates have passed (approved at ops + verified proof), so the ship is the
	// irreversible VCS mutation — commit the change (§SPEC 5.2, §12).
	hash, branch := cfg.ship(ctx, &arc)
	arc.Status = adh.StatusClosed
	arc.History = append(arc.History, "closed as "+string(arc.Resolution))
	if err := store.Save(&arc); err != nil {
		return fmt.Errorf("close: %w", err)
	}
	// Effectiveness accounting (§16): record the closed arc's cost. The ship is
	// authoritative; a metrics-write failure is a diagnostic, not fatal.
	if err := recordMetric(&arc); err != nil {
		cfg.Log.WarnContext(ctx, "metrics not recorded", "op", "close", "arc", arc.ID, "err", err)
	}
	return cfg.reportClosed(&arc, hash, branch)
}

// reportClosed emits the close outcome: a success outcome carrying the resolution
// and any commit under --jsonl, else the human commit/closed lines.
func (cfg *Config) reportClosed(arc *adh.Arc, hash, branch string) error {
	if cfg.JSONL {
		data := map[string]any{"arc": arc.ID, "resolution": string(arc.Resolution)}
		if hash != "" {
			data["commit"] = hash
			data["branch"] = branch
		}
		if err := cfg.EmitOK(data); err != nil {
			return fmt.Errorf("close: %w", err)
		}
		return nil
	}
	if hash != "" {
		_, _ = fmt.Fprintf(cfg.Stdout, "committed %s as %s on %s\n", arc.ID, hash, branch)
	}
	_, _ = fmt.Fprintf(cfg.Stdout, "closed %s as %s\n", arc.ID, arc.Resolution)
	return nil
}

// proofFail renders a proof-gate failure (exit 8, SPEC §5.4): an error outcome
// under --jsonl, else a stderr line.
func (cfg *Config) proofFail(message string) error {
	if cfg.JSONL {
		if err := cfg.EmitError(8, root.ReasonProof, message); err != nil {
			return fmt.Errorf("close: %w", err)
		}
	} else {
		_, _ = fmt.Fprintf(cfg.Stderr, "close: %s\n", message)
	}
	return root.ExitError(8)
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
	// A decision's proof is a well-formed ADR (§12), not a hash manifest: the durable
	// record of the trade-off is itself the evidence. Every other resolution verifies
	// a proof packet's artifact digests.
	if arc.Resolution == adh.ResolutionDecision {
		return cfg.verifyDecisionProof(manifest)
	}
	pkt, err := prooflib.Load(manifest)
	if err != nil {
		return false, fmt.Errorf("close: %w", err)
	}
	if verifyErr := prooflib.Verify(cfg.repoDir(), &pkt); verifyErr != nil {
		return false, cfg.proofFail("proof failed: " + verifyErr.Error())
	}
	return true, nil
}

// verifyDecisionProof validates that path is a complete ADR (§12): the structural
// proof a decision-resolution arc closes with. An unreadable file or an unfilled
// skeleton is a proof failure (exit 8), so a decision cannot ship undocumented.
func (cfg *Config) verifyDecisionProof(path string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, cfg.proofFail("decision proof unreadable: " + err.Error())
	}
	if validErr := adr.Valid(string(data)); validErr != nil {
		return false, cfg.proofFail("decision proof is not a complete ADR: " + validErr.Error())
	}
	return true, nil
}

// ship commits a `change` arc's work — the irreversible action the ops gate
// protects. It runs only past the approval + proof gates, so the commit is gated.
// The change lands on its own branch adh/<arc-id> (branch-per-arc), leaving the
// base branch untouched and the commit ready to open as a PR. It is best-effort:
// outside a git repo the arc closes without a commit (silently, since not every
// workspace is under git), and a commit error (e.g. nothing to commit) is a
// surfaced warning, not a failed close. Non-`change` resolutions carry no commit.
// It returns the short commit hash and the branch it landed on, or empty strings
// when nothing was committed; the caller renders the outcome.
func (cfg *Config) ship(ctx context.Context, arc *adh.Arc) (hash, branch string) {
	if arc.Resolution != adh.ResolutionChange {
		return "", ""
	}
	repo, err := vcs.Open(cfg.repoDir())
	if err != nil {
		return "", "" // no git repo here — nothing to commit
	}
	branch = shipBranch(repo, arc)
	who := vcs.Signature{Name: "adh", Email: "adh@localhost"}
	hash, err = repo.Commit(arc.ID+": "+arc.Title, who, time.Now())
	if err != nil {
		cfg.Log.WarnContext(ctx, "commit skipped", "op", "close", "arc", arc.ID, "err", err)
		return "", ""
	}
	arc.History = append(arc.History, "committed "+hash+" on "+branch)
	cfg.Log.InfoContext(
		ctx,
		"committed",
		"op",
		"close",
		"arc",
		arc.ID,
		"commit",
		hash,
		"branch",
		branch,
	)
	return hash, branch
}

// shipBranch isolates the arc's commit on its own branch adh/<arc-id>, created
// from HEAD, and returns the branch the commit will land on. Best-effort: a repo
// with no commit yet (a branch needs one) or an existing branch of that name
// leaves the commit on the current branch, whose name is returned instead.
func shipBranch(repo *vcs.Git, arc *adh.Arc) string {
	name := "adh/" + arc.ID
	if err := repo.CreateBranch(name); err == nil {
		return name
	}
	current, err := repo.CurrentBranch()
	if err != nil {
		return name // unreachable in practice; the commit still proceeds
	}
	return current
}

// repoDir is the repository root — the --repo global, or the current directory.
func (cfg *Config) repoDir() string {
	if cfg.Repo != "" {
		return cfg.Repo
	}
	return "."
}
