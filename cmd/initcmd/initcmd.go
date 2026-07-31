// Package initcmd implements the "init" CLI command: scaffold the .adh workspace
// so later commands have somewhere to store arcs, a starter config to tailor, and
// a context store so the cold critic can be grounded out of the box (SPEC §2.1,
// §3.1, §19.1). It is idempotent: existing files are left untouched.
package initcmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/agentic-dev-harness/cmd/root"
	"github.com/StevenACoffman/agentic-dev-harness/internal/config"
	"github.com/StevenACoffman/agentic-dev-harness/internal/contextstore"
	looplib "github.com/StevenACoffman/agentic-dev-harness/internal/loop"
	"github.com/StevenACoffman/agentic-dev-harness/internal/state"
	"github.com/StevenACoffman/agentic-dev-harness/internal/toolreg"
)

// artifactsDir is the proof-artifact registry root (SPEC §3.1 [proof].archive_dir).
const artifactsDir = ".adh/artifacts"

// Config holds the configuration for the init command.
type Config struct {
	*root.Config
	Flags   *ff.FlagSet
	Command *ff.Command
}

// New creates and registers the init command with the given parent config.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("init").SetParent(parent.Flags)
	cfg.Command = &ff.Command{
		Name:      "init",
		Usage:     "agentic-dev-harness init",
		ShortHelp: "scaffold the .adh workspace",
		LongHelp: "Create the .adh workspace (arc store, starter config, and a context " +
			"store) in the current repository. Idempotent: existing files are kept.",
		Flags: cfg.Flags,
		Exec:  cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

func (cfg *Config) exec(_ context.Context, _ []string) error {
	for _, dir := range []string{state.DefaultArcsDir, contextstore.DefaultStoreDir, artifactsDir} {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return fmt.Errorf("init: create %s: %w", dir, err)
		}
	}
	wrote, err := writeIfAbsent(config.RepoConfigFile, []byte(config.StarterTOML))
	if err != nil {
		return fmt.Errorf("init: write config: %w", err)
	}
	units, err := cfg.scaffoldContext()
	if err != nil {
		return fmt.Errorf("init: %w", err)
	}
	wroteTools, err := cfg.scaffoldTools()
	if err != nil {
		return fmt.Errorf("init: %w", err)
	}
	wroteLoops, err := cfg.scaffoldLoops()
	if err != nil {
		return fmt.Errorf("init: %w", err)
	}
	_, _ = fmt.Fprintf(cfg.Stdout,
		"initialized .adh: config %s, %d context unit(s), tools %s, loops %s\n",
		wroteWord(wrote), units, wroteWord(wroteTools), wroteWord(wroteLoops))
	return nil
}

// scaffoldTools seeds the tool registry (SPEC-ADDITIONS §13) with the external
// toolchain adh knows how to orchestrate (exegesis/skillsaw/modelith), so
// `adh tool list`/`run` discover them out of the box. It is kept if present, so a
// tailored registry is never clobbered; it reports whether it wrote.
func (cfg *Config) scaffoldTools() (bool, error) {
	data, err := toolreg.Marshal(toolreg.StarterRegistry())
	if err != nil {
		return false, fmt.Errorf("marshal tools: %w", err)
	}
	wrote, err := writeIfAbsent(toolreg.DefaultRegistryFile, data)
	if err != nil {
		return false, fmt.Errorf("write tools: %w", err)
	}
	return wrote, nil
}

// scaffoldLoops seeds the maintenance-loop registry (SPEC-ADDITIONS §15) with the
// standing accretion triggers (context-drift, harness-integrity, lesson-backlog),
// so `adh loop run` senses a departure and opens an arc without being prompted —
// accretion as a standing behavior. Kept if present; reports whether it wrote.
func (cfg *Config) scaffoldLoops() (bool, error) {
	data, err := looplib.Marshal(looplib.StarterRegistry())
	if err != nil {
		return false, fmt.Errorf("marshal loops: %w", err)
	}
	wrote, err := writeIfAbsent(looplib.DefaultRegistryFile, data)
	if err != nil {
		return false, fmt.Errorf("write loops: %w", err)
	}
	return wrote, nil
}

// scaffoldContext writes a starter context unit for each top-level directory,
// keyed by that directory (labels and paths) so a change touching it routes to
// the unit (§19.1). It leaves an already-populated store untouched and returns the
// number of units written (0 when the store already had some).
func (cfg *Config) scaffoldContext() (int, error) {
	if populated, err := storeHasUnits(contextstore.DefaultStoreDir); err != nil {
		return 0, err
	} else if populated {
		return 0, nil
	}
	dirs, err := topLevelDirs(".")
	if err != nil {
		return 0, err
	}
	for _, dir := range dirs {
		unit := contextstore.Unit{
			ID:     dir,
			Kind:   "runbook",
			Labels: []string{dir},
			Paths:  []string{dir},
		}
		data, marshalErr := json.MarshalIndent(unit, "", "  ")
		if marshalErr != nil {
			return 0, fmt.Errorf("marshal unit %s: %w", dir, marshalErr)
		}
		path := filepath.Join(contextstore.DefaultStoreDir, dir+".json")
		if _, err := writeIfAbsent(path, append(data, '\n')); err != nil {
			return 0, fmt.Errorf("write unit %s: %w", dir, err)
		}
	}
	return len(dirs), nil
}

// storeHasUnits reports whether dir already holds any *.json context unit.
func storeHasUnits(dir string) (bool, error) {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read %s: %w", dir, err)
	}
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			return true, nil
		}
	}
	return false, nil
}

// topLevelDirs returns the non-hidden directories directly under root, sorted by
// os.ReadDir. Hidden directories (".git", ".adh", …) are skipped.
func topLevelDirs(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", dir, err)
	}
	dirs := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() && entry.Name()[0] != '.' {
			dirs = append(dirs, entry.Name())
		}
	}
	return dirs, nil
}

// writeIfAbsent writes data to path only when it does not already exist, so init
// never clobbers a tailored file. It reports whether it wrote.
func writeIfAbsent(path string, data []byte) (bool, error) {
	switch _, err := os.Stat(path); {
	case err == nil:
		return false, nil
	case !os.IsNotExist(err):
		return false, fmt.Errorf("stat %s: %w", path, err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return false, fmt.Errorf("write %s: %w", path, err)
	}
	return true, nil
}

func wroteWord(wrote bool) string {
	if wrote {
		return "written"
	}
	return "kept"
}
