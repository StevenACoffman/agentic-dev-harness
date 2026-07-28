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
	AcceptanceBar string              // what the resolution's proof must show (§5.4)
	Proof         []proof.Artifact    // the proof packet the builder left
	Context       []contextstore.Unit // units routed for the arc's labels/paths (§10)
}

// Ground assembles the working set from repository state already read: it routes
// units by the arc's labels and touched paths and carries the proof packet's
// artifacts. The acceptance bar is passed in as data (the deployment's configured
// proof contract, §19.4) so the pure grounding never reads config. It mutates
// nothing; a nil packet contributes no artifacts.
func Ground(
	arc *adh.Arc,
	units []contextstore.Unit,
	pkt *proof.Packet,
	acceptanceBar string,
) Grounding {
	g := Grounding{
		Paths:         arc.Paths,
		AcceptanceBar: acceptanceBar,
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

// Load reads the working set from disk: the context store under storeDir and the
// proof packet at arc.Proof when set. acceptanceBar is the configured proof
// contract, passed through to Ground. A missing store yields no units (not an
// error); an unreadable proof manifest propagates.
func Load(arc *adh.Arc, storeDir, acceptanceBar string) (Grounding, error) {
	const op = "critic.Load"
	units, err := contextstore.Load(storeDir)
	if err != nil {
		return Grounding{}, &adh.Error{Op: op, Err: err}
	}
	var pkt *proof.Packet
	if arc.Proof != "" {
		loaded, loadErr := proof.Load(arc.Proof)
		if loadErr != nil {
			return Grounding{}, &adh.Error{Op: op, Err: loadErr}
		}
		pkt = &loaded
	}
	return Ground(arc, units, pkt, acceptanceBar), nil
}

// ForStage returns the grounding a stage needs and whether it is a routing gap.
// Only the critic is grounded; other stages return (nil, false, nil). A routing
// gap (§19.1) is an arc that declared a footprint — labels or touched paths —
// yet routed no context and left no proof: the environment did not teach the
// critic, so it would fall back on its own priors. Callers surface a gap as
// exit 12 (§10). An arc that declared no footprint is not a gap; its critic is
// simply ungrounded and the prompt says so.
func ForStage(arc *adh.Arc, storeDir, acceptanceBar string) (*Grounding, bool, error) {
	if arc.Stage != adh.StageCritic {
		return nil, false, nil
	}
	g, err := Load(arc, storeDir, acceptanceBar)
	if err != nil {
		return nil, false, err
	}
	declared := len(arc.Labels) > 0 || len(arc.Paths) > 0
	gap := declared && !g.HasGrounding()
	return &g, gap, nil
}
