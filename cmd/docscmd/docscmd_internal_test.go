package docscmd

import (
	"strings"
	"testing"

	"github.com/peterbourgon/ff/v4"
)

// tree builds a minimal root command with a global flag and one subcommand, so
// the pure renderer is exercised without standing up the whole CLI.
func tree() (rootCmd, sub *ff.Command) {
	fs := ff.NewFlagSet("adh")
	var jsonl bool
	fs.BoolVar(&jsonl, 0, "jsonl", "emit machine output as JSON Lines")
	sub = &ff.Command{
		Name:      "run",
		ShortHelp: "advance an arc through the loop",
		Flags:     ff.NewFlagSet("run").SetParent(fs),
	}
	rootCmd = &ff.Command{
		Name:        "adh",
		ShortHelp:   "drive a change through the arc loop",
		Flags:       fs,
		Subcommands: []*ff.Command{sub},
	}
	return rootCmd, sub
}

func TestRenderPageCoversTreeAndSections(t *testing.T) {
	t.Parallel()
	rootCmd, _ := tree()
	page, err := renderPage(
		rootCmd,
		[]docSection{{Title: "EXIT STATUS", Body: "0 success"}},
	)
	if err != nil {
		t.Fatalf("renderPage: %v", err)
	}
	// The page names the tool, its global flag, its subcommand, and the injected
	// section — the surface an agent reads.
	for _, want := range []string{"adh", "--jsonl", "run", "EXIT STATUS"} {
		if !strings.Contains(page, want) {
			t.Errorf("man page missing %q:\n%s", want, page)
		}
	}
}

func TestPageName(t *testing.T) {
	t.Parallel()
	rootCmd, sub := tree()
	if got := pageName(rootCmd, rootCmd); got != "adh.1" {
		t.Errorf("root pageName = %q, want adh.1", got)
	}
	if got := pageName(rootCmd, sub); got != "adh-run.1" {
		t.Errorf("subcommand pageName = %q, want adh-run.1", got)
	}
}

// TestReasonTokensBodyTracksConstants checks the reason section lists the tokens
// the commands emit — the drift guard the body is built from the constants for.
func TestReasonTokensBodyTracksConstants(t *testing.T) {
	t.Parallel()
	body := reasonTokensBody()
	for _, want := range []string{"at_ops", "requalify", "failed", "ungrounded"} {
		if !strings.Contains(body, want) {
			t.Errorf("reason section missing token %q:\n%s", want, body)
		}
	}
}

// TestVocabularySection: the root page carries the validation/verification
// vocabulary (§10.5), so the stages name the two consistency questions they answer.
func TestVocabularySection(t *testing.T) {
	if !strings.Contains(vocabularyBody, "validation") ||
		!strings.Contains(vocabularyBody, "verification") {
		t.Fatal("vocabulary section must define validation and verification")
	}
	root, _ := tree()
	page, err := renderPage(root, []docSection{
		{Title: "VOCABULARY", Body: vocabularyBody},
	})
	if err != nil {
		t.Fatalf("renderPage: %v", err)
	}
	if !strings.Contains(page, "VOCABULARY") || !strings.Contains(page, "validation") {
		t.Errorf("rendered page missing the VOCABULARY section:\n%s", page)
	}
}
