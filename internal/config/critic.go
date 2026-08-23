package config

import (
	"sort"
	"strings"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
)

// DeniedInputs is the critic's deny list as domain values.
//
// Requires: the config validated, so every name is known and deniable.
// Ensures: one value per configured name, in configured order. Pure.
func (c *Config) DeniedInputs() []adh.CriticInput {
	out := make([]adh.CriticInput, 0, len(c.Critic.Deny))
	for _, name := range c.Critic.Deny {
		out = append(out, adh.CriticInput(name))
	}
	return out
}

// validateCritic reports why the critic policy cannot be honoured, or nil.
//
// Requires: nothing; the zero Critic is valid, since both lists are optional.
// Ensures: every problem is reported at once, so one failed load tells an author
// everything to fix. EINVALID, because no retry of the same document will help. Pure.
//
// It exists because both lists used to be free-form strings read by nothing. A
// mistyped `deny = ["transript"]` was a line that silently protected nothing, in the
// field a reader is most likely to believe hardens the critic — and TOML ignores
// unknown keys, so deleting the field would have moved that silence rather than ending
// it. Validation is the smallest thing that makes the names mean something.
func validateCritic(c *Critic) error {
	var bad []string
	bad = append(bad, unknownInputs("ground_from", c.GroundFrom)...)
	bad = append(bad, unknownInputs("deny", c.Deny)...)

	for _, name := range c.Deny {
		in := adh.CriticInput(name)
		if in.Known() && !in.Deniable() {
			// Naming the reason rather than the rule: adh assembles the critic's
			// grounding whole and cannot withhold part of it, so a denial it cannot
			// enforce must not read as one it can.
			bad = append(bad, "deny lists "+name+
				", which adh cannot withhold — the critic's grounding is assembled whole; "+
				"only "+string(adh.InputTranscript)+" is omitted structurally")
		}
	}
	for _, name := range c.GroundFrom {
		if contains(c.Deny, name) {
			bad = append(bad, name+" is in both ground_from and deny")
		}
	}
	if len(bad) == 0 {
		return nil
	}
	sort.Strings(bad)
	return &adh.Error{
		Code:    adh.EINVALID,
		Message: "config: [critic]: " + strings.Join(bad, "; "),
	}
}

// unknownInputs names each entry that is not a known critic input.
func unknownInputs(field string, names []string) []string {
	var bad []string
	for _, name := range names {
		if !adh.CriticInput(name).Known() {
			bad = append(bad, field+" names an unknown input "+name+"; known: "+known())
		}
	}
	return bad
}

// known renders the closed set for a diagnostic, so an author fixing a typo does not
// have to find the list in the source.
func known() string {
	all := adh.CriticInputs()
	names := make([]string, 0, len(all))
	for _, in := range all {
		names = append(names, string(in))
	}
	return strings.Join(names, ", ")
}

func contains(names []string, want string) bool {
	for _, n := range names {
		if n == want {
			return true
		}
	}
	return false
}
