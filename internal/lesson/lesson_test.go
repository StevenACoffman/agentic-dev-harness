package lesson_test

import (
	"strings"
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

func TestOwnerMaterializesAndApproval(t *testing.T) {
	t.Parallel()
	for _, o := range []lesson.Owner{lesson.OwnerContext, lesson.OwnerDoc, lesson.OwnerDecision} {
		if !o.Materializes() || o.RequiresApproval() {
			t.Errorf("%s: want materializes+no-approval", o)
		}
	}
	for _, o := range []lesson.Owner{lesson.OwnerCheck, lesson.OwnerInvariant, lesson.OwnerType} {
		if o.Materializes() || !o.RequiresApproval() {
			t.Errorf("%s: want executable (gated, not materialized)", o)
		}
	}
}

func TestRender(t *testing.T) {
	t.Parallel()
	l := lesson.Lesson{
		Class:     "oracle drift",
		Instances: []string{"oracle: clears differ", "oracle: seed off"},
	}
	adr := l.Render(lesson.OwnerDecision)
	for _, want := range []string{"# Decision: oracle drift", "## Context", "clears differ", "### Easier", "### Harder"} {
		if !strings.Contains(adr, want) {
			t.Errorf("decision render missing %q:\n%s", want, adr)
		}
	}
	note := l.Render(lesson.OwnerContext)
	if !strings.Contains(note, "Avoid this class") || !strings.Contains(note, "seed off") {
		t.Errorf("context render = %q", note)
	}
}

func TestSlug(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"oracle drift":  "oracle-drift",
		"  Foo: Bar!! ": "foo-bar",
		"a__b":          "a-b",
	}
	for in, want := range cases {
		if got := lesson.Slug(in); got != want {
			t.Errorf("Slug(%q) = %q, want %q", in, got, want)
		}
	}
}
