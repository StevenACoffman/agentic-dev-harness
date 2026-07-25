// Package sleep implements the "sleep" CLI command: the offline consolidation
// cycle (SPEC-ADDITIONS §18.4). Before trusting the loop it runs the
// negative-control gate self-test (a planted regression must be rejected, exit
// 15). `run` harvests closed arcs, asks the optimizer for a bounded edit, gates
// it on a strict held-out improvement, and stages an accepted proposal under
// .adh/sleep/staging/<id>/ without touching live files (exit 14 while a proposal
// awaits adoption). `adopt` is the only verb that mutates live guiding docs,
// backing them up first; `status` lists what is staged. All decisions live in
// internal/consolidate; this file is the imperative shell around it.
package sleep

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
	"github.com/StevenACoffman/agentic-dev-harness/internal/consolidate"
	"github.com/StevenACoffman/agentic-dev-harness/internal/evidence"
	"github.com/StevenACoffman/agentic-dev-harness/internal/gate"
	"github.com/StevenACoffman/agentic-dev-harness/internal/harness"
	"github.com/StevenACoffman/agentic-dev-harness/internal/state"
)

const (
	artifactDefault = ".adh/context/harness.md"
	sleepRoot       = ".adh/sleep"
	evidenceFile    = sleepRoot + "/evidence.jsonl"
	stagingRoot     = sleepRoot + "/staging"
	backupRoot      = sleepRoot + "/backup"
	rejectedFile    = sleepRoot + "/rejected.json"
)

// Config holds the configuration for the sleep command.
type Config struct {
	*root.Config
	Artifact string
	Flags    *ff.FlagSet
	Command  *ff.Command
}

// manifest records what a staging directory contains: the candidate's id, the
// live path it targets, the gate decision, and the two scores.
type manifest struct {
	StagingID string      `json:"staging_id"`
	Artifact  string      `json:"artifact"`
	Decision  gate.Result `json:"decision"`
	Baseline  float64     `json:"baseline"`
	Candidate float64     `json:"candidate"`
}

// New creates and registers the sleep command with the given parent config.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("sleep").SetParent(parent.Flags)
	cfg.Flags.StringVar(&cfg.Artifact, 'a', "artifact", artifactDefault,
		"the managed guiding artifact the cycle may edit")
	cfg.Command = &ff.Command{
		Name:      "sleep",
		Usage:     "agentic-dev-harness sleep [--artifact path] <run|adopt|status>",
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
		return errors.New("sleep: expected a verb: run, adopt, or status")
	}
	switch args[0] {
	case "run":
		return cfg.run()
	case "adopt":
		return cfg.adopt(args[1:])
	case "status":
		return cfg.status()
	default:
		return fmt.Errorf("sleep: unknown verb %q; want run, adopt, or status", args[0])
	}
}

func (cfg *Config) run() error {
	// The negative control proves the gate has teeth before the loop is trusted:
	// a planted non-improving candidate must be rejected (§18.4), mirroring
	// SkillOpt's harmful-edit probe.
	if err := harness.SelfTest(); err != nil {
		_, _ = fmt.Fprintf(cfg.Stderr, "gate self-test failed: %s\n", err)
		return root.ExitError(15)
	}
	artifact, arcs, rejected, err := cfg.inputs()
	if err != nil {
		return err
	}
	learned := consolidate.Propose(consolidate.Harvest(arcs), consolidate.DefaultConfig())
	cycle, err := consolidate.Plan(artifact, learned, arcs, rejected, consolidate.DefaultConfig())
	if err != nil {
		return fmt.Errorf("sleep: %w", err)
	}
	records := stamp(cycle.Records)
	if err := appendEvidence(evidenceFile, records); err != nil {
		return err
	}
	return cfg.settle(&cycle, rejected)
}

// inputs gathers the cycle's read-only inputs: the current artifact (empty when
// absent), every arc in the workspace, and the rejected-edit buffer.
func (cfg *Config) inputs() (string, []adh.Arc, map[string]bool, error) {
	artifact, err := readArtifact(cfg.Artifact)
	if err != nil {
		return "", nil, nil, err
	}
	arcs, err := state.Default().List()
	if err != nil {
		return "", nil, nil, fmt.Errorf("sleep: %w", err)
	}
	rejected, err := loadRejected(rejectedFile)
	if err != nil {
		return "", nil, nil, err
	}
	return artifact, arcs, rejected, nil
}

// settle stages an accepted proposal (exit 14 pending human adoption), or
// self-explains and remembers a rejected candidate.
func (cfg *Config) settle(cycle *consolidate.Cycle, rejected map[string]bool) error {
	if cycle.Proposed == "" {
		if cycle.StagingID != "" && cycle.Decision.Action == gate.Reject {
			if err := saveRejected(rejectedFile, rejected, cycle.StagingID); err != nil {
				return err
			}
		}
		_, _ = fmt.Fprintf(cfg.Stdout, "no proposal staged: %s\n", note(cycle))
		return nil
	}
	if err := writeStaging(cfg.Artifact, cycle); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(cfg.Stdout,
		"staged %s (selection %.3f -> %.3f); adopt with: adh sleep adopt %s\n",
		cycle.StagingID, cycle.Baseline, cycle.Candidate, cycle.StagingID)
	return root.ExitError(14)
}

func (cfg *Config) adopt(args []string) error {
	if len(args) == 0 {
		return errors.New("sleep: adopt requires a staging id")
	}
	id := args[0]
	man, err := readManifest(id)
	if err != nil {
		return err
	}
	if man.Decision.Action == gate.Reject {
		return &adh.Error{
			Code:    adh.ECONFLICT,
			Message: "sleep: staged proposal " + id + " was not accepted",
		}
	}
	proposed, err := os.ReadFile(filepath.Join(stagingRoot, id, filepath.Base(man.Artifact)))
	if err != nil {
		return fmt.Errorf("sleep: read staged artifact: %w", err)
	}
	if err := backupLive(id, man.Artifact); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(man.Artifact), 0o750); err != nil {
		return fmt.Errorf("sleep: %w", err)
	}
	if err := atomicfile.WriteFile(man.Artifact, proposed, 0o600); err != nil {
		return fmt.Errorf("sleep: adopt: %w", err)
	}
	if err := appendEvidence(evidenceFile, stamp([]evidence.Record{{
		GateAction: string(man.Decision.Action),
		OldScore:   man.Baseline, NewScore: man.Candidate,
		Status: evidence.StatusKeep, Note: "adopted staging " + id,
	}})); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(cfg.Stdout, "adopted %s -> %s (backup under %s)\n",
		id, man.Artifact, filepath.Join(backupRoot, id))
	return nil
}

func (cfg *Config) status() error {
	entries, err := os.ReadDir(stagingRoot)
	if os.IsNotExist(err) {
		_, _ = fmt.Fprintln(cfg.Stdout, "no staged proposals")
		return nil
	}
	if err != nil {
		return fmt.Errorf("sleep: %w", err)
	}
	staged := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		man, manErr := readManifest(entry.Name())
		if manErr != nil {
			return manErr
		}
		staged++
		_, _ = fmt.Fprintf(cfg.Stdout, "%s  %s  %.3f -> %.3f\n",
			man.StagingID, man.Decision.Action, man.Baseline, man.Candidate)
	}
	if staged == 0 {
		_, _ = fmt.Fprintln(cfg.Stdout, "no staged proposals")
	}
	return nil
}

func writeStaging(livePath string, cycle *consolidate.Cycle) error {
	dir := filepath.Join(stagingRoot, cycle.StagingID)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("sleep: %w", err)
	}
	// TODO(redact): §18.4 requires staged text pass through secret redaction
	// before it is written; that is a tracked library offload (gitleaks-style).
	name := filepath.Base(livePath)
	if err := atomicfile.WriteFile(
		filepath.Join(dir, name),
		[]byte(cycle.Proposed),
		0o600,
	); err != nil {
		return fmt.Errorf("sleep: write proposed: %w", err)
	}
	man := manifest{
		StagingID: cycle.StagingID, Artifact: livePath, Decision: cycle.Decision,
		Baseline: cycle.Baseline, Candidate: cycle.Candidate,
	}
	if err := writeJSON(filepath.Join(dir, "manifest.json"), man); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(dir, "report.json"), cycle); err != nil {
		return err
	}
	return appendEvidence(filepath.Join(dir, "evidence.jsonl"), cycle.Records)
}

func backupLive(id, livePath string) error {
	data, err := os.ReadFile(livePath)
	if os.IsNotExist(err) {
		return nil // a fresh artifact has nothing to back up
	}
	if err != nil {
		return fmt.Errorf("sleep: backup read: %w", err)
	}
	dir := filepath.Join(backupRoot, id)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("sleep: %w", err)
	}
	if err := atomicfile.WriteFile(
		filepath.Join(dir, filepath.Base(livePath)),
		data,
		0o600,
	); err != nil {
		return fmt.Errorf("sleep: backup write: %w", err)
	}
	return nil
}

func readArtifact(path string) (string, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("sleep: read artifact: %w", err)
	}
	return string(data), nil
}

func readManifest(id string) (manifest, error) {
	data, err := os.ReadFile(filepath.Join(stagingRoot, id, "manifest.json"))
	if os.IsNotExist(err) {
		return manifest{}, &adh.Error{
			Code:    adh.ENOTFOUND,
			Message: "sleep: no such staging id: " + id,
		}
	}
	if err != nil {
		return manifest{}, fmt.Errorf("sleep: read manifest: %w", err)
	}
	var man manifest
	if err := json.Unmarshal(data, &man); err != nil {
		return manifest{}, fmt.Errorf("sleep: parse manifest: %w", err)
	}
	return man, nil
}

func loadRejected(path string) (map[string]bool, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return map[string]bool{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("sleep: read rejected: %w", err)
	}
	var hashes []string
	if err := json.Unmarshal(data, &hashes); err != nil {
		return nil, fmt.Errorf("sleep: parse rejected: %w", err)
	}
	seen := make(map[string]bool, len(hashes))
	for _, h := range hashes {
		seen[h] = true
	}
	return seen, nil
}

func saveRejected(path string, seen map[string]bool, add string) error {
	seen[add] = true
	hashes := make([]string, 0, len(seen))
	for h := range seen {
		hashes = append(hashes, h)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("sleep: %w", err)
	}
	return writeJSON(path, hashes)
}

func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return &adh.Error{Op: "sleep.writeJSON", Err: err}
	}
	if err := atomicfile.WriteFile(path, append(data, '\n'), 0o600); err != nil {
		return fmt.Errorf("sleep: write %s: %w", filepath.Base(path), err)
	}
	return nil
}

// appendEvidence appends records to the append-only JSONL log at path, creating
// its directory. The file open/append is the imperative shell around the pure
// evidence.Append; evidence is never rewritten, so atomicfile is not used here.
func appendEvidence(path string, records []evidence.Record) error {
	if len(records) == 0 {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("sleep: %w", err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("sleep: %w", err)
	}
	defer func() { _ = file.Close() }()
	if err := evidence.Append(file, records...); err != nil {
		return fmt.Errorf("sleep: %w", err)
	}
	return nil
}

// stamp fills each record's Timestamp with the current UTC time, keeping the
// clock in the shell so the consolidate core stays pure.
func stamp(records []evidence.Record) []evidence.Record {
	now := time.Now().UTC().Format(time.RFC3339)
	for i := range records {
		records[i].Timestamp = now
	}
	return records
}

// note is the human explanation for a cycle that staged nothing (§18.6).
func note(cycle *consolidate.Cycle) string {
	if len(cycle.Records) > 0 && cycle.Records[0].Note != "" {
		return cycle.Records[0].Note
	}
	return "no change"
}
