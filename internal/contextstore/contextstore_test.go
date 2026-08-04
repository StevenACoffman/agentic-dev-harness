package contextstore_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
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
		// "security" missed twice across two distinct strata → proposed.
		{Arc: "a1", Labels: []string{"security"}, Stratum: "2026-07"},
		{Arc: "a2", Labels: []string{"security"}, Stratum: "2026-08"},
		// "burst" missed twice but in one stratum → NOT proposed (temporal gate).
		{Arc: "a3", Labels: []string{"burst"}, Stratum: "2026-08"},
		{Arc: "a4", Labels: []string{"burst"}, Stratum: "2026-08"},
		// "perf" once → below the count threshold.
		{Arc: "a5", Labels: []string{"perf"}, Stratum: "2026-06"},
	}
	got := contextstore.ProposeRoutes(misses, 2)
	if len(got) != 1 {
		t.Fatalf("ProposeRoutes = %+v, want one proposal (only cross-stratum security)", got)
	}
	if got[0].Key != "security" || got[0].Count != 2 || got[0].Strata != 2 {
		t.Errorf("proposal = %+v, want security count 2 strata 2", got[0])
	}
}

func TestAppendLoadMissesRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "misses.jsonl")
	want := []contextstore.Miss{
		{Arc: "a1", Labels: []string{"x"}},
		{Arc: "a2", Paths: []string{"cmd"}},
	}
	for i := range want {
		if err := contextstore.AppendMiss(path, &want[i]); err != nil {
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

func TestEvaluateRouting(t *testing.T) {
	units := []contextstore.Unit{
		{ID: "sec", Labels: []string{"security"}},
		{ID: "auth", Paths: []string{"pkg/auth"}},
	}
	cases := []contextstore.RoutingCase{
		{Name: "label hit", Labels: []string{"security"}, Want: []string{"sec"}},
		{Name: "path hit", Paths: []string{"pkg/auth"}, Want: []string{"auth"}},
		{Name: "none applies", Labels: []string{"nope"}, Want: nil},
		{Name: "wrong expectation", Labels: []string{"security"}, Want: []string{"auth"}},
	}
	report := contextstore.EvaluateRouting(units, cases, contextstore.DefaultWorkingSet)
	if report.Cases != 4 || report.Passed != 3 {
		t.Fatalf("report = %+v, want 3/4 passed", report)
	}
	if len(report.Failures) != 1 || report.Failures[0] != "wrong expectation" {
		t.Errorf("failures = %v, want [wrong expectation]", report.Failures)
	}
	// TP=2 (sec, auth), FP=1 (sec routed but auth wanted), FN=1 (auth wanted, not routed).
	if report.Precision < 0.66 || report.Precision > 0.67 {
		t.Errorf("precision = %.3f, want ~0.667", report.Precision)
	}
	if report.Recall < 0.66 || report.Recall > 0.67 {
		t.Errorf("recall = %.3f, want ~0.667", report.Recall)
	}
}

func TestTrustTier(t *testing.T) {
	if !contextstore.TrustTier("").Valid() || !contextstore.HumanReviewed.Valid() {
		t.Error("empty and human-reviewed must be valid tiers")
	}
	if contextstore.TrustTier("bogus").Valid() {
		t.Error("an unknown tier must be invalid")
	}
	if contextstore.HumanReviewed.Rank() <= contextstore.MachineConfirmed.Rank() ||
		contextstore.MachineConfirmed.Rank() <= contextstore.Unverified.Rank() {
		t.Error("rank must order human-reviewed > machine-confirmed > unverified")
	}
}

func TestRouteTrustTieBreakAndSupersession(t *testing.T) {
	units := []contextstore.Unit{
		{ID: "a-low", Labels: []string{"sec"}, Verified: contextstore.Unverified},
		{ID: "b-high", Labels: []string{"sec"}, Verified: contextstore.HumanReviewed},
		{ID: "old", Labels: []string{"sec"}, SupersededBy: "b-high"},
	}
	routed := contextstore.Route(units, []string{"sec"}, nil, 0)
	// The superseded unit is dropped; the human-reviewed unit outranks the unverified
	// one on the score tie.
	if len(routed) != 2 {
		t.Fatalf("routed %d units, want 2 (superseded dropped): %+v", len(routed), routed)
	}
	if routed[0].ID != "b-high" {
		t.Errorf("routed[0] = %q, want b-high (higher trust wins the tie)", routed[0].ID)
	}
	for _, u := range routed {
		if u.ID == "old" {
			t.Error("a superseded unit must not route")
		}
	}
}

func TestWikiLintHelpers(t *testing.T) {
	units := []contextstore.Unit{
		{ID: "ok", Labels: []string{"x"}},
		{ID: "orphan", Kind: "note"},                                // no labels or paths
		{ID: "stale", Labels: []string{"y"}, SupersededBy: "ghost"}, // dangling
		{ID: "bad", Labels: []string{"z"}, Verified: "sideways"},    // invalid tier
	}
	if got := contextstore.Orphans(units); len(got) != 1 || got[0] != "orphan" {
		t.Errorf("Orphans = %v, want [orphan]", got)
	}
	if got := contextstore.DanglingSupersessions(units); len(got) != 1 || got[0] != "stale" {
		t.Errorf("DanglingSupersessions = %v, want [stale]", got)
	}
	if got := contextstore.InvalidTrust(units); len(got) != 1 || got[0] != "bad" {
		t.Errorf("InvalidTrust = %v, want [bad]", got)
	}
}

func TestIndex(t *testing.T) {
	units := []contextstore.Unit{
		{
			ID:         "rule",
			Kind:       "base-rule",
			Labels:     []string{"security"},
			Verified:   contextstore.HumanReviewed,
			Provenance: "OWASP",
		},
		{ID: "old", Kind: "note", SupersededBy: "rule"}, // superseded → excluded
	}
	idx := contextstore.Index(units)
	if !strings.Contains(idx, "rule (base-rule) [human-reviewed] security — OWASP") {
		t.Errorf("index missing the rule row:\n%s", idx)
	}
	if strings.Contains(idx, "old") {
		t.Errorf("index must exclude a superseded unit:\n%s", idx)
	}
}

func TestLooksLikePath(t *testing.T) {
	tests := []struct {
		src  string
		want bool
	}{
		{"docs/security.md", true},
		{"README.md", true},
		{"https://owasp.org/asvs", false}, // URL
		{"OWASP ASVS v4", false},          // prose (whitespace)
		{"", false},
		{"glossary", false}, // bare word, no sep/ext
	}
	for _, tt := range tests {
		if got := contextstore.LooksLikePath(tt.src); got != tt.want {
			t.Errorf("LooksLikePath(%q) = %v, want %v", tt.src, got, tt.want)
		}
	}
}

func TestDanglingSources(t *testing.T) {
	units := []contextstore.Unit{
		{ID: "rule", Sources: []string{"docs/present.md", "https://ok.example", "prose citation"}},
		{ID: "note", Sources: []string{"docs/missing.md"}},
	}
	exists := func(p string) bool { return p == "docs/present.md" }
	got := contextstore.DanglingSources(units, exists)
	if len(got) != 1 || got[0] != "note: docs/missing.md" {
		t.Errorf("DanglingSources = %v, want [note: docs/missing.md]", got)
	}
}

func TestUnverifiedClaims(t *testing.T) {
	units := []contextstore.Unit{
		{ID: "traced", Claims: []contextstore.Claim{{Quote: "found me", Source: "a.go"}}},
		{ID: "absent", Claims: []contextstore.Claim{{Quote: "not here", Source: "a.go"}}},
		{ID: "unreadable", Claims: []contextstore.Claim{{Quote: "x", Source: "gone.go"}}},
		{ID: "urlclaim", Claims: []contextstore.Claim{{Quote: "x", Source: "https://ok.example"}}},
	}
	read := func(p string) (string, error) {
		if p == "a.go" {
			return "prefix found me suffix", nil
		}
		return "", os.ErrNotExist
	}
	got := contextstore.UnverifiedClaims(units, read)
	want := []string{"absent: not here", "unreadable: x"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("UnverifiedClaims = %v, want %v", got, want)
	}
}

func TestInvalidKPIs(t *testing.T) {
	units := []contextstore.Unit{
		{ID: "ok", KPIs: []adh.KPI{{Metric: "grounded_miss", Direction: adh.WorseWhenAbove}}},
		{ID: "no-metric", KPIs: []adh.KPI{{Metric: "", Direction: adh.WorseWhenAbove}}},
		{ID: "bad-dir", KPIs: []adh.KPI{{Metric: "x", Direction: "sideways"}}},
		{ID: "none"},
	}
	got := contextstore.InvalidKPIs(units)
	want := []string{"bad-dir", "no-metric"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("InvalidKPIs = %v, want %v", got, want)
	}
}
