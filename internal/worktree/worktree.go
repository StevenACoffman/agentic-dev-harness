// Package worktree reads the git working tree to ground the relay (§19.1): the
// unified diff of the change under review, the changed code paths, and the arc
// footprint (paths + area labels) an execution turn leaves. Every read is
// best-effort — outside a git repo it yields nothing, never an error — so the
// relay stays drivable without a repository. The step and run command shells both
// use it, keeping the relay engine itself free of version-control I/O.
package worktree

import (
	"slices"
	"strings"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/contextstore"
	"github.com/StevenACoffman/agentic-dev-harness/internal/vcs"
)

// maxDiffBytes caps the diff surfaced to the critic so a large change never bloats
// the prompt unbounded; the excess is dropped with a marker.
const maxDiffBytes = 16 << 10

// Diff renders the change under review at paths as a unified diff for the critic's
// grounding (§19.1). Best-effort — outside a git repo it is empty — and capped, so
// a huge change cannot bloat the prompt (the truncation is marked, never silent).
func Diff(repoDir string, paths []string) string {
	if len(paths) == 0 {
		return ""
	}
	repo, err := vcs.Open(repoDir)
	if err != nil {
		return ""
	}
	diff, err := repo.Diff(paths)
	if err != nil {
		return ""
	}
	if len(diff) > maxDiffBytes {
		diff = diff[:maxDiffBytes] + "\n… diff truncated …\n"
	}
	return diff
}

// ChangedCodePaths reports the working tree's changed paths under repoDir,
// excluding the harness's own .adh/ state. The bool is false when there is no git
// repo — the read is best-effort and never fatal.
func ChangedCodePaths(repoDir string) ([]string, bool) {
	repo, err := vcs.Open(repoDir)
	if err != nil {
		return nil, false
	}
	status, err := repo.Status()
	if err != nil {
		return nil, false
	}
	paths := make([]string, 0, len(status.Changed))
	for _, path := range status.Changed {
		if strings.HasPrefix(path, ".adh/") {
			continue
		}
		paths = append(paths, path)
	}
	return paths, true
}

// CaptureFootprint records what an execution turn touched (§19.1, §19.3): the
// changed paths ground the cold critic on the real change, and the areas they fall
// under become labels so the arc's context routes. Best-effort — outside a git repo
// it is a no-op. Derived labels union with any already declared, preserving hand-set
// ones.
func CaptureFootprint(repoDir string, arc *adh.Arc) {
	paths, ok := ChangedCodePaths(repoDir)
	if !ok {
		return
	}
	arc.Paths = paths
	for _, label := range contextstore.AreaLabels(paths) {
		if !slices.Contains(arc.Labels, label) {
			arc.Labels = append(arc.Labels, label)
		}
	}
}
