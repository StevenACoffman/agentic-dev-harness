package adh

import "sort"

// The inputs a critic's view can name (SPEC-ADDITIONS §19.4).
//
// They exist so `[critic] ground_from` and `deny` are checked against a closed set
// rather than being free-form strings nobody validates. A mistyped input name in
// either list used to be a line that silently did nothing — which is the whole defect
// this vocabulary was added to end, and it is worse in a field a reader believes
// hardens the critic.
//
// InputUnset is the zero value and names no input, so a value nobody populated cannot
// be mistaken for a real one.
const (
	InputUnset CriticInput = ""

	// InputTranscript is the builder's turn history. It is the one input cold review
	// withholds (§19.1), and the only one the renderer can currently enforce.
	InputTranscript CriticInput = "transcript"

	// The grounding a critic reviews, as critic.Grounding assembles it.
	InputDiff          CriticInput = "diff"
	InputProof         CriticInput = "proof"
	InputAcceptanceBar CriticInput = "acceptance_bar"
	InputContext       CriticInput = "context"
	InputPaths         CriticInput = "paths"
	InputTools         CriticInput = "tools"
	InputCoverage      CriticInput = "coverage"
	InputNoisy         CriticInput = "noisy"
)

// CriticInput names one input a critic's view can carry.
type CriticInput string

// CriticInputs is every input a critic's view can name, sorted.
//
// Requires: nothing.
// Ensures: sorted, so a diagnostic listing them is reproducible. Pure.
func CriticInputs() []CriticInput {
	all := []CriticInput{
		InputTranscript, InputDiff, InputProof, InputAcceptanceBar,
		InputContext, InputPaths, InputTools, InputCoverage, InputNoisy,
	}
	sort.Slice(all, func(i, j int) bool { return all[i] < all[j] })
	return all
}

// Known reports whether c names an input this build understands.
//
// Requires: nothing.
// Ensures: false for InputUnset, so an unpopulated value never validates.
func (c CriticInput) Known() bool {
	for _, k := range CriticInputs() {
		if c == k {
			return true
		}
	}
	return false
}

// Deniable reports whether the renderer can enforce a denial of this input.
//
// Requires: nothing.
// Ensures: true only for InputTranscript today.
//
// **adh does not filter a critic's grounding**, and this method is where that fact is
// enforced rather than assumed. The critic's view omits the transcript structurally —
// the renderer never populates it — so a denial of it is an assertion the renderer can
// check. Every other input is assembled by `critic.Ground` and handed over whole;
// denying one would be a request to filter, which is a capability nobody has built.
//
// Accepting such a name silently is what the config used to do. Refusing it names the
// gap, and the day filtering exists this method is the one place that changes.
func (c CriticInput) Deniable() bool { return c == InputTranscript }
