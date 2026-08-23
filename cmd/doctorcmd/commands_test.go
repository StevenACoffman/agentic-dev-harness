package doctorcmd_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/cmd/doctorcmd"
	"github.com/StevenACoffman/agentic-dev-harness/cmd/root"
)

// seedUnit writes a context unit whose prose is body, and returns the repo dir.
func seedUnit(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	store := filepath.Join(dir, ".adh", "context")
	if err := os.MkdirAll(store, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(store, "u1.md"), []byte(body), 0o600); err != nil {
		t.Fatalf("write content: %v", err)
	}
	unit := `{"id":"u1","kind":"requirement","content_path":"u1.md"}`
	if err := os.WriteFile(filepath.Join(store, "u1.json"), []byte(unit), 0o600); err != nil {
		t.Fatalf("write unit: %v", err)
	}
	return dir
}

// TestProseNamingAnAbsentCommandIsReported. "Every instruction the agent reads must be
// backed by a working command" — a unit naming `make verify` claims that target exists,
// and nothing checked it. skillet's claims package finds the claim; resolving it is
// adh's, because $PATH is the environment and skillet must not touch it.
func TestProseNamingAnAbsentCommandIsReported(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		body string
		want bool
	}{
		"an absent command": {
			"Run `definitely-not-a-real-tool build` before shipping.", true,
		},
		// go is on PATH wherever this test runs, so a real command must be silent.
		"a command that resolves": {"Run `go build ./...` first.", false},
		// claims.Commands needs a space and a lowercase leading token; a bare
		// filename is not a claim about a command.
		"a filename in a code span": {"See `README.md` for details.", false},
		"no code spans at all":      {"Just prose about caching.", false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dir := seedUnit(t, tc.body)
			out := runDoctor(t, dir)
			got := strings.Contains(out, "dangling_command")
			if got != tc.want {
				t.Errorf("dangling_command reported = %v, want %v\n%s", got, tc.want, out)
			}
		})
	}
}

// TestOneUninstalledToolIsOneFinding. claims.Commands errs toward finding fewer and its
// documented false positive is lowercase prose in a code span; combined with a $PATH
// lookup on a fresh machine the honest failure mode is chattiness, so a command is
// reported once however often the prose mentions it.
func TestOneUninstalledToolIsOneFinding(t *testing.T) {
	t.Parallel()
	dir := seedUnit(t,
		"Run `definitely-not-a-real-tool build`.\nThen `definitely-not-a-real-tool build` again.\n")

	if n := strings.Count(runDoctor(t, dir), "dangling_command"); n != 1 {
		t.Errorf("got %d findings for one repeated command, want 1", n)
	}
}

// runDoctor runs doctor against dir and returns its combined output.
func runDoctor(t *testing.T, dir string) string {
	t.Helper()
	var out strings.Builder
	r := root.New(func(string) string { return "" }, strings.NewReader(""), &out, &out)
	doctorcmd.New(r)
	// --repo rather than t.Chdir: chdir is process-global and refuses to run under
	// t.Parallel, and doctor already takes the repository root as a flag.
	if err := r.Command.Parse([]string{"--repo", dir, "doctor"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	_ = r.Command.Run(t.Context())
	return out.String()
}
