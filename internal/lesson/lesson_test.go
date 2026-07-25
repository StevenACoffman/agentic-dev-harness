package lesson_test

import (
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/internal/lesson"
)

func TestRequiresApproval(t *testing.T) {
	tests := []struct {
		owner lesson.Owner
		want  bool
	}{
		{owner: lesson.OwnerContext, want: false},
		{owner: lesson.OwnerSkill, want: false},
		{owner: lesson.OwnerDoc, want: false},
		{owner: lesson.OwnerCheck, want: true},
		{owner: lesson.OwnerInvariant, want: true},
		{owner: lesson.OwnerType, want: true},
	}
	for _, tt := range tests {
		t.Run(string(tt.owner), func(t *testing.T) {
			if got := tt.owner.RequiresApproval(); got != tt.want {
				t.Errorf("%s.RequiresApproval() = %v, want %v", tt.owner, got, tt.want)
			}
		})
	}
}

func TestDistill(t *testing.T) {
	failures := []string{
		"nil-deref: arc-1 crashed on empty input",
		"nil-deref: arc-3 crashed on nil map",
		"missing-timeout: arc-2 hung",
	}
	lessons := lesson.Distill(failures)
	if len(lessons) != 2 {
		t.Fatalf("Distill produced %d classes, want 2", len(lessons))
	}
	if lessons[0].Class != "missing-timeout" || lessons[1].Class != "nil-deref" {
		t.Errorf(
			"classes = %q,%q, want missing-timeout,nil-deref",
			lessons[0].Class,
			lessons[1].Class,
		)
	}
	if len(lessons[1].Instances) != 2 {
		t.Errorf("nil-deref instances = %d, want 2", len(lessons[1].Instances))
	}
}
