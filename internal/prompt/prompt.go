// Package prompt renders a stage's model prompt from templates. The defaults are
// embedded so the binary is self-contained; a repo may override any stage by
// dropping a same-named template under .adh/prompts (SPEC-ADDITIONS §18.1, the
// editable guiding artifact). It is a pure renderer: data in, string out, no I/O
// beyond the read-only embedded and override filesystems handed to New.
package prompt

import (
	"bytes"
	"embed"
	"io/fs"
	"os"
	"text/template"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/critic"
)

// RepoOverrideDir is the conventional repo path a project drops stage templates
// into to override the embedded defaults (SPEC-ADDITIONS §18.5 artifact_roots).
const RepoOverrideDir = ".adh/prompts"

//go:embed templates/*.tmpl
var defaults embed.FS

// Renderer renders a stage's prompt. Templates are keyed by "<stage>.tmpl";
// embedded defaults are overlaid by any override filesystems, later ones winning.
type Renderer struct{ tmpl *template.Template }

// view is the data a stage template sees. Ground is the stage's routed working set
// (§10) — context and tools — carried for every stage; the critic additionally
// reviews the diff and proof in it. The critic's view deliberately omits History:
// it runs cold (SPEC §1), reviewing only the change and its grounding, never the
// builder's transcript. The guarantee is enforced here in the data, so a
// hand-edited critic template still cannot leak the transcript.
type view struct {
	ID         string
	Title      string
	Stage      adh.Stage
	Resolution adh.Resolution
	ProofKind  string
	History    []string
	Ground     *critic.Grounding
}

// New builds a Renderer from the embedded default templates, then overlays each
// override filesystem (a *.tmpl set, e.g. os.DirFS(".adh/prompts")). A nil or
// template-free override is skipped; a malformed template is an EINVALID error.
func New(overrides ...fs.FS) (*Renderer, error) {
	const op = "prompt.New"
	tmpl, err := template.ParseFS(defaults, "templates/*.tmpl")
	if err != nil {
		return nil, &adh.Error{Op: op, Err: err}
	}
	for _, override := range overrides {
		if override == nil {
			continue
		}
		matches, globErr := fs.Glob(override, "*.tmpl")
		if globErr != nil {
			return nil, &adh.Error{Op: op, Err: globErr}
		}
		if len(matches) == 0 {
			continue
		}
		if _, parseErr := tmpl.ParseFS(override, "*.tmpl"); parseErr != nil {
			return nil, &adh.Error{Op: op, Err: parseErr}
		}
	}
	return &Renderer{tmpl: tmpl}, nil
}

// Default builds a Renderer from the embedded defaults, overlaid with the repo's
// RepoOverrideDir templates when that directory exists. A missing directory is
// not an error: fs.Glob over it matches nothing, so the defaults stand.
func Default() (*Renderer, error) {
	return New(os.DirFS(RepoOverrideDir))
}

// Render produces the prompt for arc's current stage. ground is the stage's routed
// working set (§10, §19.1); it may be nil (an ungrounded stage). Ops has no prompt
// (it ships via close, not a model step) and an unknown stage has no template; both
// return EINVALID rather than a silent empty prompt.
func (r *Renderer) Render(arc *adh.Arc, ground *critic.Grounding) (string, error) {
	const op = "prompt.Renderer.Render"
	name := string(arc.Stage) + ".tmpl"
	if r.tmpl.Lookup(name) == nil {
		return "", &adh.Error{
			Code:    adh.EINVALID,
			Message: "no prompt template for stage: " + string(arc.Stage),
		}
	}
	v := view{
		ID:         arc.ID,
		Title:      arc.Title,
		Stage:      arc.Stage,
		Resolution: arc.Resolution,
		ProofKind:  arc.Resolution.ProofKind(),
	}
	// Every stage acts against its routed working set (§10). The critic alone stays
	// cold: it sees the grounding but never the builder's history (§19.1); other
	// stages see both.
	v.Ground = ground
	if arc.Stage != adh.StageCritic {
		v.History = arc.History
	}
	var b bytes.Buffer
	if err := r.tmpl.ExecuteTemplate(&b, name, v); err != nil {
		return "", &adh.Error{Op: op, Err: err}
	}
	return b.String(), nil
}
