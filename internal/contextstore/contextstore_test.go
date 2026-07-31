package contextstore_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/internal/contextstore"
)

func units() []contextstore.Unit {
	return []contextstore.Unit{
		{ID: "crypto-note", Labels: []string{"security"}, Paths: []string{"internal/auth"}},
		{ID: "ui-skill", Labels: []string{"frontend", "ui"}},
		{ID: "deploy-runbook", Labels: []string{"ops"}, Paths: []string{"deploy"}},
	}
}

func TestRouteByLabel(t *testing.T) {
	got := contextstore.Route(units(), []string{"frontend"}, nil, 8)
	if len(got) != 1 || got[0].ID != "ui-skill" {
		t.Fatalf("Route by label = %v, want [ui-skill]", ids(got))
	}
}

func TestRouteByPathPrefix(t *testing.T) {
	got := contextstore.Route(units(), nil, []string{"internal/auth/token.go"}, 8)
	if len(got) != 1 || got[0].ID != "crypto-note" {
		t.Fatalf("Route by path = %v, want [crypto-note]", ids(got))
	}
}

func TestRouteRanksAndCaps(t *testing.T) {
	// ui-skill matches two labels; deploy-runbook matches one. Cap to 1 keeps the top.
	got := contextstore.Route(units(), []string{"frontend", "ui", "ops"}, nil, 1)
	if len(got) != 1 || got[0].ID != "ui-skill" {
		t.Fatalf("Route ranking/cap = %v, want [ui-skill]", ids(got))
	}
}

func TestRouteNoMatch(t *testing.T) {
	if got := contextstore.Route(units(), []string{"nope"}, nil, 8); len(got) != 0 {
		t.Fatalf("Route no match = %v, want none", ids(got))
	}
}

func ids(units []contextstore.Unit) []string {
	out := make([]string, len(units))
	for i := range units {
		out[i] = units[i].ID
	}
	return out
}

func TestAreaLabels(t *testing.T) {
	tests := []struct {
		name  string
		paths []string
		want  []string
	}{
		{
			"dirs deduped and sorted",
			[]string{"internal/a.go", "cmd/c.go", "internal/b.go"},
			[]string{"cmd", "internal"},
		},
		{"none", nil, []string{}},
		{"top-level file has no area", []string{"main.go"}, []string{}},
		{"leading dot-slash trimmed", []string{"./cmd/x.go"}, []string{"cmd"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := contextstore.AreaLabels(tt.paths)
			if len(got) != len(tt.want) {
				t.Fatalf("AreaLabels(%v) = %v, want %v", tt.paths, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("AreaLabels(%v)[%d] = %q, want %q", tt.paths, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestContent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(dir, "note.md"),
		[]byte("approved crypto library: X"),
		0o600,
	); err != nil {
		t.Fatalf("write: %v", err)
	}

	// A unit with a content path returns the file's text.
	got, err := contextstore.Content(dir, &contextstore.Unit{ID: "n", ContentPath: "note.md"})
	if err != nil {
		t.Fatalf("Content: %v", err)
	}
	if got != "approved crypto library: X" {
		t.Errorf("Content = %q, want the file text", got)
	}

	// A metadata-only unit (no content path) is not an error.
	if got, err := contextstore.Content(dir, &contextstore.Unit{ID: "n"}); err != nil || got != "" {
		t.Errorf("empty ContentPath = (%q, %v), want (\"\", nil)", got, err)
	}

	// A set-but-missing path is an error — the routing promised text not there.
	if _, err := contextstore.Content(
		dir,
		&contextstore.Unit{ID: "n", ContentPath: "gone.md"},
	); err == nil {
		t.Errorf("missing content file should error")
	}

	// A path escaping the store is refused.
	if _, err := contextstore.Content(
		dir,
		&contextstore.Unit{ID: "n", ContentPath: "../secret"},
	); err == nil {
		t.Errorf("content_path escaping the store should error")
	}
}

func TestDuplicateIDs(t *testing.T) {
	tests := []struct {
		name  string
		units []contextstore.Unit
		want  []string
	}{
		{"none", []contextstore.Unit{{ID: "a"}, {ID: "b"}}, []string{}},
		{"one dup", []contextstore.Unit{{ID: "a"}, {ID: "a"}, {ID: "b"}}, []string{"a"}},
		{
			"two dups sorted",
			[]contextstore.Unit{{ID: "z"}, {ID: "z"}, {ID: "a"}, {ID: "a"}},
			[]string{"a", "z"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := contextstore.DuplicateIDs(tt.units)
			if len(got) != len(tt.want) {
				t.Fatalf("DuplicateIDs = %v, want %v", got, tt.want)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Errorf("DuplicateIDs[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestProposeRoutes(t *testing.T) {
	misses := []contextstore.Miss{
		{Arc: "a1", Labels: []string{"security"}, Paths: []string{"pkg/auth"}},
		{Arc: "a2", Labels: []string{"security"}},
		{Arc: "a3", Labels: []string{"perf"}},
	}
	// Threshold 2: only "security" (2 hits) is proposed; "perf"/"pkg/auth" (1 each) are not.
	got := contextstore.ProposeRoutes(misses, 2)
	if len(got) != 1 {
		t.Fatalf("ProposeRoutes = %+v, want one proposal", got)
	}
	if got[0].Key != "security" || got[0].Kind != "label" || got[0].Count != 2 {
		t.Errorf("proposal = %+v, want security/label/2", got[0])
	}
}

func TestAppendLoadMissesRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "misses.jsonl")
	want := []contextstore.Miss{
		{Arc: "a1", Labels: []string{"x"}},
		{Arc: "a2", Paths: []string{"cmd"}},
	}
	for _, m := range want {
		if err := contextstore.AppendMiss(path, m); err != nil {
			t.Fatalf("AppendMiss: %v", err)
		}
	}
	got, err := contextstore.LoadMisses(path)
	if err != nil {
		t.Fatalf("LoadMisses: %v", err)
	}
	if len(got) != 2 || got[0].Arc != "a1" || got[1].Arc != "a2" {
		t.Fatalf("round-trip = %+v, want a1 then a2", got)
	}
	// An absent log is no misses, not an error.
	empty, err := contextstore.LoadMisses(filepath.Join(t.TempDir(), "none.jsonl"))
	if err != nil || len(empty) != 0 {
		t.Errorf("LoadMisses(absent) = (%v, %v), want ([], nil)", empty, err)
	}
}
