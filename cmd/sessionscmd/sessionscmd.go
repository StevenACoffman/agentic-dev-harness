// Package sessionscmd implements the "sessions" CLI command: mine foreign agent-session
// stores for corrections that never became arcs, and report what was found.
//
// **`report` is the only verb, and that is the design.** The decision recorded in
// SPEC-ADDITIONS §18 step 1 is that mined signals do not enter the held-out splits until
// the mining rule has earned it, and the step before earning it is measuring how often it
// is wrong. A command that can only report cannot quietly become a command that feeds the
// loop; adding a second verb would be the moment to re-read that decision.
package sessionscmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/agentic-dev-harness/cmd/root"
	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/session"
)

// Config holds the configuration for the sessions command.
type Config struct {
	*root.Config
	Flags   *ff.FlagSet
	Command *ff.Command
}

// New creates and registers the sessions command with the given parent config.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("sessions").SetParent(parent.Flags)
	cfg.Command = &ff.Command{
		Name:      "sessions",
		Usage:     "agentic-dev-harness sessions report <dir>",
		ShortHelp: "mine foreign agent sessions for corrections, and report",
		LongHelp: "Read Claude Code transcripts under <dir> and report the corrective " +
			"turns they contain — a prompt answered by saying the last attempt was " +
			"wrong. Reports only: nothing here enters the consolidation loop, because " +
			"the detection is a heuristic whose error rate has not been measured. See " +
			"SPEC-ADDITIONS §18 step 1.",
		Flags: cfg.Flags,
		Exec:  cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

func (cfg *Config) exec(_ context.Context, args []string) error {
	if len(args) == 0 {
		return root.MissingVerbError{Scope: "sessions", Verbs: []string{"report"}}
	}
	if args[0] != "report" {
		return root.UnknownVerbError{
			Scope: "sessions", Got: args[0], Verbs: []string{"report"},
		}
	}
	if len(args) < 2 {
		return root.MissingOperandError{Scope: "sessions report", Kind: root.OperandDir}
	}
	digests, err := loadDir(args[1])
	if err != nil {
		return err
	}
	return cfg.report(session.Summarise(digests))
}

// loadDir reads every .jsonl transcript under dir, one level deep and one below, since a
// session store keeps one directory per project.
func loadDir(dir string) ([]session.Digest, error) {
	paths, err := transcripts(dir)
	if err != nil {
		return nil, err
	}
	digests := make([]session.Digest, 0, len(paths))
	for _, path := range paths {
		file, openErr := os.Open(path)
		if openErr != nil {
			return nil, &adh.Error{Op: "sessions.loadDir", Err: openErr}
		}
		digest, loadErr := session.LoadClaude(file)
		_ = file.Close()
		if loadErr != nil {
			return nil, fmt.Errorf("sessions: %w", loadErr)
		}
		if digest.ID == "" {
			digest.ID = filepath.Base(path)
		}
		digests = append(digests, digest)
	}
	return digests, nil
}

// transcripts lists the .jsonl files in dir and in its immediate subdirectories.
func transcripts(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, &adh.Error{Op: "sessions.transcripts", Err: err}
	}
	var paths []string
	for _, entry := range entries {
		full := filepath.Join(dir, entry.Name())
		if !entry.IsDir() {
			if filepath.Ext(entry.Name()) == ".jsonl" {
				paths = append(paths, full)
			}
			continue
		}
		nested, nestedErr := os.ReadDir(full)
		if nestedErr != nil {
			return nil, &adh.Error{Op: "sessions.transcripts", Err: nestedErr}
		}
		for _, sub := range nested {
			if !sub.IsDir() && filepath.Ext(sub.Name()) == ".jsonl" {
				paths = append(paths, filepath.Join(full, sub.Name()))
			}
		}
	}
	sort.Strings(paths)
	return paths, nil
}

// report renders the mined corpus.
func (cfg *Config) report(rep session.Report) error {
	if cfg.JSONL {
		if err := cfg.EmitOK(map[string]any{
			"sessions": rep.Sessions, "turns": rep.Turns,
			"retries": len(rep.Retries), "by_marker": rep.ByMarker,
		}); err != nil {
			return fmt.Errorf("sessions: %w", err)
		}
		return nil
	}
	_, _ = fmt.Fprintf(cfg.Stdout, "%d session(s), %d turn(s), %d corrective turn(s)\n",
		rep.Sessions, rep.Turns, len(rep.Retries))
	for _, marker := range sortedKeys(rep.ByMarker) {
		_, _ = fmt.Fprintf(cfg.Stdout, "  %-16s %d\n", marker, rep.ByMarker[marker])
	}
	return nil
}

// sortedKeys orders a marker tally for stable output.
func sortedKeys(m map[string]int) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
