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
	"sort"
	"time"

	git "github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/plumbing/object"

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
