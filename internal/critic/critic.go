// Package critic assembles the cold critic's grounding (SPEC-ADDITIONS §19.1):
// the repository-owned working set a critic reviews against — the arc's touched
// paths, its acceptance bar, the proof packet the builder left, and the context
// units routed for its labels and paths (§10). It withholds exactly one input,
// the builder's transcript; that isolation lives in the prompt renderer. Ground
// is a pure assembly; Load and ForStage are the thin I/O shells around it.
package critic

import (
	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/contextstore"
	"github.com/StevenACoffman/agentic-dev-harness/internal/proof"
)

// MaxContextUnits caps how many routed context units enter a critic's working
// set, matching the context command's working-set size.
const MaxContextUnits = contextstore.DefaultWorkingSet

// Grounding is the critic's working set (§19.1): everything repository-owned the
// review is judged against. It never carries the builder's transcript.
type Grounding struct {
	Paths         []string            // the arc's touched repository paths
	Diff          string              // unified diff of the change under review (§19.1)
	AcceptanceBar string              // what the resolution's proof must show (§5.4)
	Proof         []proof.Artifact    // the proof packet the builder left
	Context       []contextstore.Unit // units routed for the arc's labels/paths (§10)
}

// Inputs are the grounding facts the shell computes and hands to the pure core:
// the acceptance bar (the deployment's configured proof contract, §19.4) and the
// diff of the change under review (§19.1). Bundling them keeps the critic free of
// config and vcs — they arrive as data — and lets more inputs be added without
// churning every Ground/Load/ForStage signature.
type Inputs struct {
	AcceptanceBar string
	Diff          string
}

// Ground assembles the working set from repository state already read: it routes
// units by the arc's labels and touched paths, carries the proof packet's
// artifacts, and carries the shell-supplied inputs (bar + diff). It mutates
// nothing; a nil packet contributes no artifacts.
func Ground(
	arc *adh.Arc,
	units []contextstore.Unit,
	pkt *proof.Packet,
	in Inputs,
) Grounding {
	g := Grounding{
		Paths:         arc.Paths,
		Diff:          in.Diff,
		AcceptanceBar: in.AcceptanceBar,
		Context:       contextstore.Route(units, arc.Labels, arc.Paths, MaxContextUnits),
	}
	if pkt != nil {
		g.Proof = pkt.Artifacts
	}
	return g
}

// HasGrounding reports whether the environment taught the critic anything
// repository-specific: a proof packet to check or a routed context unit. The
// acceptance bar alone (generic proof-kind text) does not count.
func (g *Grounding) HasGrounding() bool {
	return len(g.Proof) > 0 || len(g.Context) > 0
}

// loadInputs reads the raw grounding inputs from disk: the context store under
// storeDir and the proof packet at arc.Proof when set. A missing store yields no
// units (not an error); an unreadable proof manifest propagates.
func loadInputs(arc *adh.Arc, storeDir string) ([]contextstore.Unit, *proof.Packet, error) {
	const op = "critic.loadInputs"
	units, err := contextstore.Load(storeDir)
	if err != nil {
		return nil, nil, &adh.Error{Op: op, Err: err}
	}
	var pkt *proof.Packet
	if arc.Proof != "" {
		loaded, loadErr := proof.Load(arc.Proof)
		if loadErr != nil {
			return nil, nil, &adh.Error{Op: op, Err: loadErr}
		}
		pkt = &loaded
	}
	return units, pkt, nil
}

// Load assembles the critic's working set. in carries the shell-supplied bar and
// diff, passed through to Ground.
func Load(arc *adh.Arc, storeDir string, in Inputs) (Grounding, error) {
	units, pkt, err := loadInputs(arc, storeDir)
	if err != nil {
		return Grounding{}, err
	}
	return Ground(arc, units, pkt, in), nil
}

// ForStage returns the grounding a stage needs and whether it is a routing gap.
// Only the critic is grounded; other stages return (nil, false, nil). A routing
// gap (§19.1) is an arc that declared a footprint (labels or touched paths)
// against a context store that *exists* — has units — yet routes nothing, with no
// proof: the environment is set up but did not teach this arc, so the critic would
// fall back on its own priors. Callers surface a gap as exit 12 (§10). An empty or
// absent store is simply ungrounded, not a gap (grounding is not configured);
// likewise an arc that declared no footprint. The prompt says so in both cases.
func ForStage(arc *adh.Arc, storeDir string, in Inputs) (*Grounding, bool, error) {
	if arc.Stage != adh.StageCritic {
		return nil, false, nil
	}
	units, pkt, err := loadInputs(arc, storeDir)
	if err != nil {
		return nil, false, err
	}
	g := Ground(arc, units, pkt, in)
	declared := len(arc.Labels) > 0 || len(arc.Paths) > 0
	gap := declared && len(units) > 0 && !g.HasGrounding()
	return &g, gap, nil
}
