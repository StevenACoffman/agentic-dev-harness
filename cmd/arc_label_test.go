package cmd_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/internal/state"
)

// TestArcNewWithLabels checks that `arc new --label` seeds the arc's routing
// labels (§10). Flags precede the verb because ff stops parsing at the first
// positional; the labels are deduped and sorted.
func TestArcNewWithLabels(t *testing.T) {
	t.Chdir(t.TempDir())
	id := strings.TrimSpace(
		mustRun(
			t,
			"arc",
			"--label",
			"routing",
			"--label",
			"api",
			"--label",
			"api",
			"new",
			"fix the thing",
		),
	)
	arc, err := state.Default().Get(id)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if want := []string{"api", "routing"}; !slices.Equal(arc.Labels, want) {
		t.Errorf("labels = %v, want %v (sorted, deduped)", arc.Labels, want)
	}
}

// TestArcNewWithoutLabels confirms a bare arc carries no labels — Execution
// derives them from the change later.
func TestArcNewWithoutLabels(t *testing.T) {
	t.Chdir(t.TempDir())
	id := strings.TrimSpace(mustRun(t, "arc", "new", "no labels here"))
	arc, err := state.Default().Get(id)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if len(arc.Labels) != 0 {
		t.Errorf("labels = %v, want none", arc.Labels)
	}
}
