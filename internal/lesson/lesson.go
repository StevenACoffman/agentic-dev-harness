// Package lesson turns recurring corrections into promotions to a durable owner
// (SPEC-ADDITIONS §11): it distills failures into governing classes and routes
// each to its smallest owner. Promoting to an executable owner is human-gated.
package lesson

import (
	"sort"
	"strings"
)

// Owner values, from reversible (context) to executable (check/invariant/type).
const (
	OwnerContext   Owner = "context"
	OwnerSkill     Owner = "skill"
	OwnerDoc       Owner = "doc"
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
	case OwnerContext, OwnerSkill, OwnerDoc:
		return false
	default:
		return false
	}
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
