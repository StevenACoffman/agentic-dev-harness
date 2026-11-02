// Package execconfig guards the repository files adh executes or hands a model as
// instructions: the tool and loop registries, prompt-template overrides, and the agent
// configuration Claude Code and other harnesses read.
//
// The threat is a change made inside an arc. The worker can edit `.adh/tools.json`, and
// the next `adh tool run` or adjudication would execute the edit with the maintainer's
// authority, before any reviewer saw it. A review gate alone does not close that window,
// because execution happens first. So adh runs these files only as committed: a working
// copy that differs from HEAD, or one HEAD does not track, is refused until a human
// commits it. The comparison is exact (bytes), so there is no threshold to argue down.
//
// Compare and Executable are pure; Check and Require read git and the working tree.
package execconfig

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/vcs"
)

// The repository-relative paths adh itself executes or renders. They are spelled here
// rather than imported so this package stays a leaf; each owning package's constant
// names the same path, and execconfig_test pins that they agree.
const (
	ToolsFile   = ".adh/tools.json"
	LoopsFile   = ".adh/loops.json"
	PromptsDir  = ".adh/prompts"
	promptsGlob = "*.tmpl"
)

// Drift is one executable-config file whose working copy is not what HEAD holds.
type Drift struct {
	Path   string // repository-relative, slash-separated
	Reason string // why it is refused: "untracked" or "modified since HEAD"
}

// headReader is the slice of vcs this package needs.
type headReader interface {
	HeadContent(path string) (string, bool, error)
}

// Executable reports whether the repository-relative path names executable config:
// adh's tool and loop registries and prompt templates, and the agent configuration a
// harness reads as instructions or runs as hooks (`.claude/settings*.json`,
// `.claude/hooks/`, `.mcp.json`, `CLAUDE.md`, `AGENTS.md`).
//
// It is the one list both the critic's grounding and the approval gate use, so a file
// the gate shows is a file the critic sees.
func Executable(p string) bool {
	p = path.Clean(filepath.ToSlash(p))
	switch {
	case p == ToolsFile, p == LoopsFile, p == ".mcp.json", p == "CLAUDE.md", p == "AGENTS.md":
		return true
	case strings.HasPrefix(p, PromptsDir+"/"), strings.HasPrefix(p, ".claude/hooks/"):
		return true
	}
	dir, base := path.Split(p)
	return dir == ".claude/" && strings.HasPrefix(base, "settings") &&
		strings.HasSuffix(base, ".json")
}

// Compare reports why a working copy may not be used, or "" when it may.
//
// Requires: tracked and head describe HEAD; exists and work describe the working tree.
// Ensures:  "" when the file does not exist in the working tree (nothing would run from
//
//	it) or matches HEAD byte for byte; otherwise a reason naming the difference.
func Compare(head string, tracked bool, work string, exists bool) string {
	switch {
	case !exists:
		return ""
	case !tracked:
		return "untracked"
	case head != work:
		return "modified since HEAD"
	}
	return ""
}

// Check compares each repository-relative path under repoDir with HEAD and returns the
// ones that differ, sorted by path.
//
// Outside a git repository there is no HEAD to compare against and nothing is reported.
// adh's relay is drivable without a repository (worktree's convention), and an arc's
// worker edits a repository, so the guarded case always has one.
func Check(repoDir string, paths []string) ([]Drift, error) {
	repo, err := vcs.Open(repoDir)
	if err != nil {
		return nil, nil //nolint:nilerr // no repository: nothing to compare, see doc
	}
	return check(repo, repoDir, paths)
}

func check(repo headReader, repoDir string, paths []string) ([]Drift, error) {
	const op = "execconfig.Check"
	var drift []Drift
	for _, p := range paths {
		head, tracked, err := repo.HeadContent(p)
		if err != nil {
			return nil, &adh.Error{Op: op, Err: err}
		}
		work, err := os.ReadFile(filepath.Join(repoDir, filepath.FromSlash(p)))
		exists := err == nil
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, &adh.Error{Op: op, Err: err}
		}
		if reason := Compare(head, tracked, string(work), exists); reason != "" {
			drift = append(drift, Drift{Path: p, Reason: reason})
		}
	}
	sort.Slice(drift, func(i, j int) bool { return drift[i].Path < drift[j].Path })
	return drift, nil
}

// Require returns an ECONFLICT error naming every path that differs from HEAD, or nil
// when all of them may be used. Callers invoke it immediately before executing or
// rendering from the files, never earlier, so a later edit cannot slip in between.
func Require(repoDir string, paths ...string) error {
	drift, err := Check(repoDir, paths)
	if err != nil {
		return err
	}
	return refusal(drift)
}

// RequirePrompts is Require over every template in the prompt-override directory.
func RequirePrompts(repoDir string) error {
	matches, err := filepath.Glob(
		filepath.Join(repoDir, filepath.FromSlash(PromptsDir), promptsGlob),
	)
	if err != nil {
		return &adh.Error{Op: "execconfig.RequirePrompts", Err: err}
	}
	paths := make([]string, 0, len(matches))
	for _, m := range matches {
		paths = append(paths, path.Join(PromptsDir, filepath.Base(m)))
	}
	return Require(repoDir, paths...)
}

// refusal renders drift as the error a human acts on, or nil for none.
func refusal(drift []Drift) error {
	if len(drift) == 0 {
		return nil
	}
	parts := make([]string, 0, len(drift))
	for _, d := range drift {
		parts = append(parts, fmt.Sprintf("%s (%s)", d.Path, d.Reason))
	}
	return &adh.Error{
		Code: adh.ECONFLICT,
		Message: "refusing to run uncommitted executable config: " + strings.Join(parts, ", ") +
			"; review and commit it, then retry",
	}
}
