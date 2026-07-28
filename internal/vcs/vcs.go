// Package vcs is the version-control seam: the small set of git operations the
// harness needs — inspect the working tree, branch, and commit. Git is the
// adapter over go-git (v6); Mock is an in-memory implementation with the same
// methods, so a consumer declares a point-of-use interface and swaps them (no
// unused interface lives here). Errors from go-git are translated to adh.Error
// at this boundary, so callers see domain codes, not library types. Merge and
// revert are intentionally absent: go-git's merge is experimental, so those
// mutations remain a `git` shell-out follow-up (TODO).
package vcs

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	billy "github.com/go-git/go-billy/v6"
	git "github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/plumbing/object"
	"github.com/hexops/gotextdiff"
	"github.com/hexops/gotextdiff/myers"
	"github.com/hexops/gotextdiff/span"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
)

// shortHashLen is how many hex characters of a commit hash Commit returns.
const shortHashLen = 12

// Status is a snapshot of the working tree: the current branch, whether it is
// clean, and the sorted set of changed (staged, modified, or untracked) paths.
type Status struct {
	Branch  string   `json:"branch"`
	Clean   bool     `json:"clean"`
	Changed []string `json:"changed"`
}

// Signature is a commit author.
type Signature struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// Git is a working repository backed by go-git. Commit takes the author time
// from the caller so the adapter reads no clock (the shell owns it).
type Git struct {
	repo *git.Repository
}

// Open opens an existing repository rooted at dir.
func Open(dir string) (*Git, error) {
	repo, err := git.PlainOpen(dir)
	if err != nil {
		return nil, &adh.Error{Op: "vcs.Open", Err: err}
	}
	return newGit(repo)
}

// Init creates a new, empty repository at dir.
func Init(dir string) (*Git, error) {
	repo, err := git.PlainInit(dir, false)
	if err != nil {
		return nil, &adh.Error{Op: "vcs.Init", Err: err}
	}
	return newGit(repo)
}

// newGit wraps repo and disables commit signing in its local config. go-git
// honors an ambient commit.gpgSign=true but has no signer registered, which
// would fail every Commit; adh authors plain commits, so we turn signing off for
// this repository rather than depend on the host's GPG setup.
func newGit(repo *git.Repository) (*Git, error) {
	cfg, err := repo.Config()
	if err != nil {
		return nil, &adh.Error{Op: "vcs.newGit", Err: err}
	}
	cfg.Raw.Section("commit").SetOption("gpgsign", "false")
	if err := repo.SetConfig(cfg); err != nil {
		return nil, &adh.Error{Op: "vcs.newGit", Err: err}
	}
	return &Git{repo: repo}, nil
}

// CurrentBranch returns the checked-out branch. Before the first commit HEAD has
// no target commit, so it falls back to the symbolic HEAD's branch name.
func (g *Git) CurrentBranch() (string, error) {
	if head, err := g.repo.Head(); err == nil {
		return head.Name().Short(), nil
	}
	ref, err := g.repo.Reference(plumbing.HEAD, false)
	if err != nil {
		return "", &adh.Error{Op: "vcs.Git.CurrentBranch", Err: err}
	}
	return ref.Target().Short(), nil
}

// Status reports the branch, cleanliness, and changed paths of the working tree.
func (g *Git) Status() (Status, error) {
	branch, err := g.CurrentBranch()
	if err != nil {
		return Status{}, err
	}
	worktree, err := g.repo.Worktree()
	if err != nil {
		return Status{}, &adh.Error{Op: "vcs.Git.Status", Err: err}
	}
	state, err := worktree.Status()
	if err != nil {
		return Status{}, &adh.Error{Op: "vcs.Git.Status", Err: err}
	}
	changed := make([]string, 0, len(state))
	for path := range state {
		changed = append(changed, path)
	}
	sort.Strings(changed)
	return Status{Branch: branch, Clean: state.IsClean(), Changed: changed}, nil
}

// CreateBranch creates a branch at HEAD and checks it out. It requires at least
// one commit, since a branch points at a commit.
func (g *Git) CreateBranch(name string) error {
	worktree, err := g.repo.Worktree()
	if err != nil {
		return &adh.Error{Op: "vcs.Git.CreateBranch", Err: err}
	}
	err = worktree.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName(name),
		Create: true,
	})
	if err != nil {
		return &adh.Error{Op: "vcs.Git.CreateBranch", Err: err}
	}
	return nil
}

// Commit stages every change in the working tree (git add -A) and records a
// commit authored by who at when, returning the short hash.
func (g *Git) Commit(msg string, who Signature, when time.Time) (string, error) {
	worktree, err := g.repo.Worktree()
	if err != nil {
		return "", &adh.Error{Op: "vcs.Git.Commit", Err: err}
	}
	if err := worktree.AddWithOptions(&git.AddOptions{All: true}); err != nil {
		return "", &adh.Error{Op: "vcs.Git.Commit", Err: err}
	}
	hash, err := worktree.Commit(msg, &git.CommitOptions{
		Author: &object.Signature{Name: who.Name, Email: who.Email, When: when},
	})
	if err != nil {
		return "", &adh.Error{Op: "vcs.Git.Commit", Err: err}
	}
	return hash.String()[:shortHashLen], nil
}

// Diff renders a unified diff of the given repository-relative paths between the
// committed HEAD content and the current working tree — the pre-commit change
// under review (SPEC-ADDITIONS §19.1). It reads both sides through the go-git
// handle and formats the text with gotextdiff, so it needs no `git` binary. A path
// unchanged between HEAD and the tree contributes nothing; a path absent from HEAD
// reads as all-added, one absent from the tree as all-removed.
func (g *Git) Diff(paths []string) (string, error) {
	const op = "vcs.Git.Diff"
	worktree, err := g.repo.Worktree()
	if err != nil {
		return "", &adh.Error{Op: op, Err: err}
	}
	head, err := g.headTree()
	if err != nil {
		return "", err
	}
	var b strings.Builder
	for _, path := range paths {
		before, err := treeContent(head, path)
		if err != nil {
			return "", err
		}
		after, err := worktreeContent(worktree.Filesystem(), path)
		if err != nil {
			return "", err
		}
		if before == after {
			continue
		}
		edits := myers.ComputeEdits(span.URIFromPath(path), before, after)
		_, _ = fmt.Fprintf(&b, "%s", gotextdiff.ToUnified("a/"+path, "b/"+path, before, edits))
	}
	return b.String(), nil
}

// headTree returns the tree of the current HEAD commit, or nil before the first
// commit (when every path reads as new).
func (g *Git) headTree() (*object.Tree, error) {
	const op = "vcs.Git.headTree"
	ref, err := g.repo.Head()
	switch {
	case errors.Is(err, plumbing.ErrReferenceNotFound):
		//nolint:nilnil // no commit yet: an empty HEAD is no tree, not an error
		return nil, nil
	case err != nil:
		return nil, &adh.Error{Op: op, Err: err}
	}
	commit, err := g.repo.CommitObject(ref.Hash())
	if err != nil {
		return nil, &adh.Error{Op: op, Err: err}
	}
	tree, err := commit.Tree()
	if err != nil {
		return nil, &adh.Error{Op: op, Err: err}
	}
	return tree, nil
}

// treeContent returns the committed content of path in tree, or "" when the tree
// is nil (no commit) or the path is absent (a new file).
func treeContent(tree *object.Tree, path string) (string, error) {
	if tree == nil {
		return "", nil
	}
	file, err := tree.File(path)
	if errors.Is(err, object.ErrFileNotFound) {
		return "", nil
	}
	if err != nil {
		return "", &adh.Error{Op: "vcs.treeContent", Err: err}
	}
	content, err := file.Contents()
	if err != nil {
		return "", &adh.Error{Op: "vcs.treeContent", Err: err}
	}
	return content, nil
}

// worktreeContent returns the working-tree content of path, or "" when it is
// absent (a deleted file).
func worktreeContent(fsys billy.Filesystem, path string) (string, error) {
	const op = "vcs.worktreeContent"
	file, err := fsys.Open(path)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", &adh.Error{Op: op, Err: err}
	}
	defer func() { _ = file.Close() }()
	data, err := io.ReadAll(file)
	if err != nil {
		return "", &adh.Error{Op: op, Err: err}
	}
	return string(data), nil
}
