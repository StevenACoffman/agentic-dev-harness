// Package toolreg is the tool registry (SPEC-ADDITIONS §13): declared
// capabilities a stage can discover, select by what they verify, and repair. It
// makes the tool surface uniform and extensible instead of hard-coded.
package toolreg

import (
	"encoding/json"
	"os"

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
