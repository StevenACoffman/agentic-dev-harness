// Package toolreg is the tool registry (SPEC-ADDITIONS §13): declared
// capabilities a stage can discover, select by what they verify, and repair. It
// makes the tool surface uniform and extensible instead of hard-coded.
package toolreg

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
)

// DefaultRegistryFile is the conventional repo path the tool registry lives under
// (SPEC-ADDITIONS §13). It is the single owner of that path across the CLI.
const DefaultRegistryFile = ".adh/tools.json"

// Tool is one declared capability: how to run it, the shape of its result, what
// it verifies, and the hint printed when it fails.
type Tool struct {
	ID         string `json:"id"`
	Run        string `json:"run"`
	Result     string `json:"result,omitempty"`
	Verifies   string `json:"verifies"`
	RepairHint string `json:"repair_hint,omitempty"`
}

// Registry is the set of declared tools.
type Registry struct {
	Tools []Tool `json:"tools"`
}

// Validate checks that every tool has an ID, a Run command, and a Verifies
// description, and that IDs are unique. It returns EINVALID on the first defect.
func (r Registry) Validate() error {
	seen := make(map[string]bool, len(r.Tools))
	for _, tool := range r.Tools {
		switch {
		case tool.ID == "":
			return &adh.Error{Code: adh.EINVALID, Message: "tool with empty id"}
		case tool.Run == "":
			return &adh.Error{
				Code:    adh.EINVALID,
				Message: "tool " + tool.ID + " has no run command",
			}
		case tool.Verifies == "":
			return &adh.Error{
				Code:    adh.EINVALID,
				Message: "tool " + tool.ID + " declares no 'verifies'",
			}
		case seen[tool.ID]:
			return &adh.Error{Code: adh.EINVALID, Message: "duplicate tool id: " + tool.ID}
		}
		seen[tool.ID] = true
	}
	return nil
}

// StarterRegistry is the set of external toolchain capabilities the harness knows
// how to orchestrate (SPEC-ADDITIONS §13): distillation (exegesis), skill
// optimization (skillsaw), and domain modeling (modelith). They are declared, not
// vendored — each Run invokes an installed binary with `--json`/`--format json`
// where available, so the worker runs it via `adh tool run <id>` and interprets the
// output in-loop. A repository seeds this via `adh init` and then tailors it; a
// RepairHint names the install command for a capability the operator has not yet
// provided. adh does not require any of them to be installed — an absent binary is
// reported at run time, not a registry defect.
func StarterRegistry() Registry {
	return Registry{Tools: []Tool{
		{
			ID:         "modelith-lint",
			Run:        "modelith lint --format json",
			Result:     "json",
			Verifies:   "domain-model reference integrity (cardinality, ownership, cycles, dangling refs)",
			RepairHint: "install modelith: go install github.com/stacklok/modelith@latest",
		},
		{
			ID:         "modelith-render-check",
			Run:        "modelith render --check",
			Verifies:   "routed domain-model Markdown matches its canonical YAML (anti-drift)",
			RepairHint: "re-run `modelith render` to refresh the drifted Markdown, or install modelith",
		},
		{
			ID:         "skillsaw-eval",
			Run:        "skillsaw eval --json",
			Result:     "json",
			Verifies:   "a skill scores against the 9-dimension rubric floor",
			RepairHint: "install skillsaw: go install github.com/StevenACoffman/skillsaw@latest",
		},
		{
			ID:         "exegesis-verify",
			Run:        "exegesis verify",
			Verifies:   "distilled skills pass the triple-validation gates and carry provenance",
			RepairHint: "install exegesis: go install github.com/StevenACoffman/exegesis@latest",
		},
	}}
}

// FindByVerifies returns the first tool whose Verifies matches, so a stage
// selects a capability by what it proves rather than by a hard-coded command.
func (r Registry) FindByVerifies(verifies string) (Tool, bool) {
	for _, tool := range r.Tools {
		if tool.Verifies == verifies {
			return tool, true
		}
	}
	return Tool{}, false
}

// FindByID returns the tool with the given ID, so a caller that already knows a
// check by name (e.g. an NFR finding's Ref, §19.2) can resolve it to a command.
func (r Registry) FindByID(id string) (Tool, bool) {
	for _, tool := range r.Tools {
		if tool.ID == id {
			return tool, true
		}
	}
	return Tool{}, false
}

// Marshal encodes the registry as indented JSON with a trailing newline — the
// on-disk form Load reads back, so the write and read sides of the registry file
// agree in one place.
func Marshal(r Registry) ([]byte, error) {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return nil, &adh.Error{Op: "toolreg.Marshal", Err: err}
	}
	return append(data, '\n'), nil
}

// Load reads a registry from a JSON file.
func Load(path string) (Registry, error) {
	const op = "toolreg.Load"
	data, err := os.ReadFile(path)
	if err != nil {
		return Registry{}, &adh.Error{Op: op, Err: err}
	}
	var reg Registry
	if err := json.Unmarshal(data, &reg); err != nil {
		return Registry{}, &adh.Error{Op: op, Err: err}
	}
	return reg, nil
}

// LoadRepo reads the registry at repoDir's DefaultRegistryFile, best-effort: an
// absent file is an empty registry (the repository declares no tools), not an
// error, so a caller can surface tools without requiring the file to exist.
func LoadRepo(repoDir string) (Registry, error) {
	path := filepath.Join(repoDir, DefaultRegistryFile)
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return Registry{}, nil
	}
	return Load(path)
}
