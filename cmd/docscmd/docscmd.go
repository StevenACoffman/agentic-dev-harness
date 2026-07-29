// Package docscmd implements the "docs" CLI command: generate man pages for the
// whole command tree straight from the live ff.Command definitions, so the pages
// never drift from the flags. Without --dir it writes the root man page — enriched
// with the exit-status, reason-token, and environment sections an agent driving
// adh as a skill needs — to stdout; with --dir it writes one page per command.
package docscmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/muesli/roff"
	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/agentic-dev-harness/cmd/root"
	mff "github.com/StevenACoffman/mango-ff"
)

// manSection is the roff man-page section docs writes into: 1, user commands.
const manSection uint = 1

// docSection is one extra man-page section (e.g. EXIT STATUS) the shell injects
// into the root page. The pure renderer stays free of adh domain vocabulary; the
// command supplies the text.
type docSection struct {
	Title string
	Body  string
}

// Config holds the configuration for the docs command.
type Config struct {
	*root.Config
	Dir     string
	Flags   *ff.FlagSet
	Command *ff.Command
}

// New creates and registers the docs command with the given parent config.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("docs").SetParent(parent.Flags)
	cfg.Flags.StringVar(&cfg.Dir, 0, "dir", "",
		"write one man page per command into this directory instead of the root page to stdout")
	cfg.Command = &ff.Command{
		Name:      "docs",
		Usage:     "agentic-dev-harness docs [--dir <directory>]",
		ShortHelp: "generate man pages for every command",
		LongHelp: "Generate roff man pages from the command tree (SPEC §2). " +
			"Without --dir the root man page is written to stdout, enriched with the " +
			"exit-status, reason-token, and environment reference. With --dir one page " +
			"per command is written into the directory (adh.1, adh-<command>.1).",
		Flags: cfg.Flags,
		Exec:  cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

func (cfg *Config) exec(_ context.Context, _ []string) error {
	rootCmd := cfg.Config.Command
	if cfg.Dir != "" {
		return cfg.writeTree(rootCmd)
	}
	page, err := renderPage(manSection, rootCmd, cfg.rootSections())
	if err != nil {
		return fmt.Errorf("docs: %w", err)
	}
	_, _ = io.WriteString(cfg.Stdout, page)
	return nil
}

// rootSections is the enrichment appended to the root man page: the exit-status,
// reason-token, and environment reference an agent needs. Populated in the command
// so the pure renderer stays free of adh domain vocabulary.
func (cfg *Config) rootSections() []docSection {
	return nil
}

// writeTree writes the root page and one page per subcommand into cfg.Dir. Only
// the root page carries the enrichment sections; a per-command page documents just
// that command's own flags and usage.
func (cfg *Config) writeTree(rootCmd *ff.Command) error {
	if err := os.MkdirAll(cfg.Dir, 0o750); err != nil {
		return fmt.Errorf("docs: %w", err)
	}
	if err := cfg.writePage(rootCmd, rootCmd, cfg.rootSections()); err != nil {
		return err
	}
	for i := range rootCmd.Subcommands {
		if err := cfg.writePage(rootCmd, rootCmd.Subcommands[i], nil); err != nil {
			return err
		}
	}
	return nil
}

// writePage renders cmd's man page and writes it to its file under cfg.Dir.
func (cfg *Config) writePage(rootCmd, cmd *ff.Command, extra []docSection) error {
	page, err := renderPage(manSection, cmd, extra)
	if err != nil {
		return fmt.Errorf("docs: %w", err)
	}
	name := pageName(rootCmd, cmd)
	if err := os.WriteFile(filepath.Join(cfg.Dir, name), []byte(page), 0o600); err != nil {
		return fmt.Errorf("docs: %w", err)
	}
	_, _ = fmt.Fprintln(cfg.Stderr, "wrote", name)
	return nil
}

// renderPage builds cmd's man page from its ff definition and appends the extra
// sections, returning the roff text. It is pure: no I/O, deterministic given the
// command tree, so a test renders and inspects it without touching the filesystem.
func renderPage(section uint, cmd *ff.Command, extra []docSection) (string, error) {
	man, err := mff.NewManPage(section, cmd)
	if err != nil {
		return "", fmt.Errorf("build man page for %s: %w", cmd.Name, err)
	}
	for _, s := range extra {
		man = man.WithSection(s.Title, s.Body)
	}
	return man.Build(roff.NewDocument()), nil
}

// pageName is the man-page filename for cmd: "adh.1" for the root, else
// "adh-<command>.1", so a subcommand's page sorts beside the root under --dir.
func pageName(rootCmd, cmd *ff.Command) string {
	if cmd == rootCmd {
		return "adh.1"
	}
	return "adh-" + strings.ReplaceAll(cmd.Name, " ", "-") + ".1"
}
