// Package lesson turns recurring corrections into promotions to a durable owner
// (SPEC-ADDITIONS §11): it distills failures into governing classes and routes
// each to its smallest owner. Promoting to an executable owner is human-gated.
package lesson

import (
	"fmt"
	"sort"
	"strings"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adr"
)

// Owner values, from reversible (context) to executable (check/invariant/type).
const (
	OwnerContext   Owner = "context"
	OwnerSkill     Owner = "skill"
	OwnerDoc       Owner = "doc"
	OwnerDecision  Owner = "decision"
	OwnerCheck     Owner = "check"
	OwnerInvariant Owner = "invariant"
	OwnerType      Owner = "type"
)

// Owner is the durable place a lesson lands.
type Owner string

// Lesson is a governing failure class and the instances that motivated it.
type Lesson struct {
	Class     string
	Instances []string
}

// RequiresApproval reports whether promoting to o changes an executable control
// (check, invariant, or type) and therefore needs human approval (§11.2).
func (o Owner) RequiresApproval() bool {
	switch o {
	case OwnerCheck, OwnerInvariant, OwnerType:
		return true
	case OwnerContext, OwnerSkill, OwnerDoc, OwnerDecision:
		return false
	default:
		return false
	}
}

// Valid reports whether o is a known durable owner.
func (o Owner) Valid() bool {
	switch o {
	case OwnerContext, OwnerSkill, OwnerDoc, OwnerDecision,
		OwnerCheck, OwnerInvariant, OwnerType:
		return true
	default:
		return false
	}
}

// Materializes reports whether promoting to o produces a routable §10 context unit
// the harness writes directly — the reversible content owners. A skill and the
// executable owners (check/invariant/type) need separate authoring, so they gate
// but are not auto-written.
func (o Owner) Materializes() bool {
	switch o {
	case OwnerContext, OwnerDoc, OwnerDecision:
		return true
	default:
		return false
	}
}

// Kind is the §10 context-unit kind a materialized owner writes.
func (o Owner) Kind() string {
	switch o {
	case OwnerDecision:
		return "decision"
	case OwnerDoc:
		return "doc"
	default:
		return "domain-note"
	}
}

// Render produces the durable content for promoting the lesson to owner o (pure;
// the caller writes it). A decision is an ADR skeleton (§12) — the recurring class
// is the forcing context, the decision and its Easier/Harder trade-offs are left
// for a human to complete; any other materializing owner is avoid-this-class
// guidance carrying the recorded instances as evidence.
func (l Lesson) Render(o Owner) string {
	if o == OwnerDecision {
		var context strings.Builder
		fmt.Fprintf(&context, "The %q correction recurred (%d instances):\n\n",
			l.Class, len(l.Instances))
		writeInstances(&context, l.Instances)
		return adr.Render(l.Class, context.String())
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\nAvoid this class of mistake (%d recorded):\n\n",
		l.Class, len(l.Instances))
	writeInstances(&b, l.Instances)
	return b.String()
}

// writeInstances lists a lesson's instances as evidence bullets.
func writeInstances(b *strings.Builder, instances []string) {
	for _, inst := range instances {
		fmt.Fprintf(b, "- %s\n", inst)
	}
}

// Slug turns a class into a filename-safe kebab id (lowercase alphanumerics joined
// by single dashes).
func Slug(class string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(class) {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			dash = false
		case b.Len() > 0 && !dash:
			b.WriteByte('-')
			dash = true
		}
	}
	return strings.TrimRight(b.String(), "-")
}

// Distill groups failure notes by their governing class — the text before the
// first ':' — so a class that recurs becomes one candidate lesson. The result is
// sorted by class for stable output.
func Distill(failures []string) []Lesson {
	byClass := make(map[string][]string)
	for _, f := range failures {
		class := f
		if i := strings.Index(f, ":"); i >= 0 {
			class = strings.TrimSpace(f[:i])
		}
		byClass[class] = append(byClass[class], f)
	}
	lessons := make([]Lesson, 0, len(byClass))
	for class, instances := range byClass {
		lessons = append(lessons, Lesson{Class: class, Instances: instances})
	}
	sort.Slice(lessons, func(i, j int) bool { return lessons[i].Class < lessons[j].Class })
	return lessons
}
