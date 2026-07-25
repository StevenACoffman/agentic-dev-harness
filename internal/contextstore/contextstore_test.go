package contextstore_test

import (
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
	for i, u := range units {
		out[i] = u.ID
	}
	return out
}
