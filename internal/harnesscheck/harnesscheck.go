// Package harnesscheck is the pure harness-integrity self-check (SPEC-ADDITIONS
// §10.4): given the loaded context store, tool registry, loop registry, and NFR
// specs, it reports whether the harness is intact and internally consistent —
// every registry is structurally valid, unit ids are unique, and the cross-
// references between the pieces resolve (a routed unit's integrity check names a
// declared §13 tool). It performs no I/O; the command loads the inputs and calls
// Check, so "is the whole harness consistent?" answers deterministically.
package harnesscheck

import (
	"sort"

	"github.com/StevenACoffman/agentic-dev-harness/internal/contextstore"
	"github.com/StevenACoffman/agentic-dev-harness/internal/loop"
	"github.com/StevenACoffman/agentic-dev-harness/internal/nfr"
	"github.com/StevenACoffman/agentic-dev-harness/internal/toolreg"
)

// Problem kinds — a stable machine token per defect class, so an agent branches on
// the kind rather than the prose.
const (
	KindToolRegistry      = "tool_registry"      // the §13 registry is structurally invalid
	KindLoopRegistry      = "loop_registry"      // the §15 registry is structurally invalid
	KindUnitFields        = "unit_fields"        // a context unit is missing its id or kind
	KindDuplicateUnit     = "duplicate_unit"     // two units answer to the same id
	KindNFRSpec           = "nfr_spec"           // an NFR spec is not well-formed Planguage
	KindDanglingIntegrity = "dangling_integrity" // a unit's integrity check names no declared tool
	KindDanglingSupersede = "dangling_supersede" // a unit's superseded_by names no existing unit
	KindInvalidTrust      = "invalid_trust"      // a unit's trust tier is outside the taxonomy
)

// Inputs bundles the loaded harness state Check reasons over.
type Inputs struct {
	Units []contextstore.Unit
	Tools toolreg.Registry
	Loops loop.Registry
	Specs []nfr.Spec
}

// Problem is one harness-integrity defect: its machine kind, the referent it
// concerns (a unit or spec id, empty for a whole-registry defect), and a human
// detail.
type Problem struct {
	Kind   string `json:"kind"`
	Ref    string `json:"ref,omitempty"`
	Detail string `json:"detail"`
}

// Check runs the deterministic harness-integrity self-check and returns every
// problem it finds, in a stable order (by kind, then ref, then detail). An empty
// result means the harness is intact. It is pure.
func Check(in *Inputs) []Problem {
	problems := make([]Problem, 0)
	problems = appendRegistryProblems(problems, in)
	problems = appendUnitProblems(problems, in.Units)
	problems = appendSpecProblems(problems, in.Specs)
	problems = appendCrossRefProblems(problems, in.Units, in.Tools)
	problems = appendLifecycleProblems(problems, in.Units)
	sort.Slice(problems, func(i, j int) bool {
		switch {
		case problems[i].Kind != problems[j].Kind:
			return problems[i].Kind < problems[j].Kind
		case problems[i].Ref != problems[j].Ref:
			return problems[i].Ref < problems[j].Ref
		default:
			return problems[i].Detail < problems[j].Detail
		}
	})
	return problems
}

// appendRegistryProblems reports a structurally invalid tool or loop registry.
func appendRegistryProblems(problems []Problem, in *Inputs) []Problem {
	if err := in.Tools.Validate(); err != nil {
		problems = append(problems, Problem{Kind: KindToolRegistry, Detail: err.Error()})
	}
	if err := in.Loops.Validate(); err != nil {
		problems = append(problems, Problem{Kind: KindLoopRegistry, Detail: err.Error()})
	}
	return problems
}

// appendUnitProblems reports units missing an id or kind and duplicate unit ids.
func appendUnitProblems(problems []Problem, units []contextstore.Unit) []Problem {
	for i := range units {
		if units[i].ID == "" || units[i].Kind == "" {
			problems = append(problems, Problem{
				Kind: KindUnitFields, Ref: units[i].ID, Detail: "unit is missing its id or kind",
			})
		}
	}
	for _, id := range contextstore.DuplicateIDs(units) {
		problems = append(problems, Problem{
			Kind: KindDuplicateUnit, Ref: id, Detail: "unit id appears more than once",
		})
	}
	return problems
}

// appendSpecProblems reports NFR specs that are not well-formed Planguage.
func appendSpecProblems(problems []Problem, specs []nfr.Spec) []Problem {
	for i := range specs {
		if err := specs[i].Valid(); err != nil {
			problems = append(problems, Problem{
				Kind: KindNFRSpec, Ref: specs[i].ID, Detail: err.Error(),
			})
		}
	}
	return problems
}

// appendCrossRefProblems reports units whose integrity check names a tool the §13
// registry does not declare — a dangling reference that would make `context verify`
// fail to resolve its anti-drift check.
func appendCrossRefProblems(
	problems []Problem,
	units []contextstore.Unit,
	tools toolreg.Registry,
) []Problem {
	for i := range units {
		integrity := units[i].Integrity
		if integrity == "" {
			continue
		}
		if _, ok := tools.FindByID(integrity); !ok {
			problems = append(problems, Problem{
				Kind:   KindDanglingIntegrity,
				Ref:    units[i].ID,
				Detail: "integrity check " + integrity + " is not a declared tool (§13)",
			})
		}
	}
	return problems
}

// appendLifecycleProblems reports the OKF lifecycle defects (§10.4): a supersession
// pointing at a unit that does not exist, and a trust tier outside the taxonomy.
func appendLifecycleProblems(problems []Problem, units []contextstore.Unit) []Problem {
	for _, id := range contextstore.DanglingSupersessions(units) {
		problems = append(problems, Problem{
			Kind: KindDanglingSupersede, Ref: id,
			Detail: "superseded_by names a unit that does not exist",
		})
	}
	for _, id := range contextstore.InvalidTrust(units) {
		problems = append(problems, Problem{
			Kind: KindInvalidTrust, Ref: id, Detail: "trust tier is outside the taxonomy",
		})
	}
	return problems
}
