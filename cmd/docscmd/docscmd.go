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

// exitStatusBody documents the process exit codes (SPEC §7 plus the additions the
// commands emit). The exit code is the primary machine signal; --jsonl carries the
// same class in the outcome's code field.
const exitStatusBody = "The exit code is the primary machine signal (SPEC §7).\n\n" +
	"0   success\n\n" +
	"1   generic error; also a terminal evaluation fail (reason failed, §4.1)\n\n" +
	"2   usage or invalid arguments\n\n" +
	"3   model-gate refused: a judgment role on an under-powered model\n\n" +
	"4   pending human gate; approval required before advancing\n\n" +
	"5   oracle divergence detected\n\n" +
	"6   invariant violation\n\n" +
	"7   on-device validation failed\n\n" +
	"8   proof verification failed (NO-PROOF-NO-CLOSE)\n\n" +
	"9   worker changed; requalify before continuing (§14)\n\n" +
	"12  critic routing gap: the environment did not ground the review (§19.1)\n\n" +
	"13  lesson promotion to an executable owner requires approval (§11.2)"

// environmentBody documents the env-var override rule ff applies to every flag.
const environmentBody = "Every flag can be set by an AGENTIC_DEV_HARNESS_-prefixed " +
	"environment variable: uppercase the flag and replace dashes with underscores " +
	"(--jsonl becomes AGENTIC_DEV_HARNESS_JSONL). A flag given on the command line " +
	"always overrides its environment variable."

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
// reason-token, and environment reference an agent needs but which ff cannot derive
// from the flags. It lives in the command so the pure renderer stays free of adh
// domain vocabulary. The reason tokens read from the root.Reason* constants, so the
// page cannot drift from the tokens the commands actually emit.
func (cfg *Config) rootSections() []docSection {
	return []docSection{
		{Title: "EXIT STATUS", Body: exitStatusBody},
		{Title: "REASON TOKENS", Body: reasonTokensBody()},
		{Title: "ENVIRONMENT", Body: environmentBody},
	}
}

// reasonTokensBody documents the stable machine tokens an --jsonl blocked or error
// outcome carries in its reason field, built from the root.Reason* constants so the
// page tracks the tokens the commands emit.
func reasonTokensBody() string {
	tokens := []struct{ token, meaning string }{
		{root.ReasonAtOps, "arc reached the ops ship gate; approve then close"},
		{
			root.ReasonUngrounded,
			"critic routing gap; the environment did not ground the review (exit 12)",
		},
		{root.ReasonGate, "a stage requires a human gate / pending approval (exit 4)"},
		{root.ReasonProof, "proof verification failed (exit 8)"},
		{root.ReasonRequalify, "worker changed; run worker requalify (exit 9)"},
		{root.ReasonFailed, "arc failed evaluation past its rework budget (exit 1)"},
	}
	var b strings.Builder
	b.WriteString("An --jsonl blocked or error outcome carries a stable reason token " +
		"an agent branches on instead of matching prose (SPEC §8):\n\n")
	for _, t := range tokens {
		fmt.Fprintf(&b, "%s   %s\n\n", t.token, t.meaning)
	}
	b.WriteString("A confirmed finding's kind (oracle, invariant, device, nfr, contract) " +
		"and an error class (not_found, invalid, conflict, internal) also appear as reasons.")
	return b.String()
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
