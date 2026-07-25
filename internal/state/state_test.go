package state_test

import (
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/state"
)

func TestStoreRoundTrip(t *testing.T) {
	store := state.NewStore(t.TempDir())

	id, err := store.NextID()
	if err != nil {
		t.Fatalf("NextID: %v", err)
	}
	if id != "arc-0001" {
		t.Fatalf("first NextID = %q, want arc-0001", id)
	}

	arc := adh.Arc{ID: id, Title: "fix crash", Stage: adh.StageStrategy, Status: adh.StatusOpen}
	if err := store.Create(&arc); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := store.Get(id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Title != "fix crash" || got.Stage != adh.StageStrategy {
		t.Errorf("Get = %+v, want title/stage preserved", got)
	}

	if err := store.Create(&arc); adh.ErrorCode(err) != adh.ECONFLICT {
		t.Errorf("duplicate Create code = %q, want conflict", adh.ErrorCode(err))
	}

	next, err := store.NextID()
	if err != nil {
		t.Fatalf("NextID after create: %v", err)
	}
	if next != "arc-0002" {
		t.Errorf("NextID after one arc = %q, want arc-0002", next)
	}

	arcs, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(arcs) != 1 {
		t.Errorf("List len = %d, want 1", len(arcs))
	}
}

func TestStoreGetMissing(t *testing.T) {
	store := state.NewStore(t.TempDir())
	if _, err := store.Get("arc-9999"); adh.ErrorCode(err) != adh.ENOTFOUND {
		t.Errorf("Get missing code = %q, want not_found", adh.ErrorCode(err))
	}
}

func TestStoreListEmpty(t *testing.T) {
	store := state.NewStore(t.TempDir())
	arcs, err := store.List()
	if err != nil {
		t.Fatalf("List empty: %v", err)
	}
	if len(arcs) != 0 {
		t.Errorf("empty workspace List len = %d, want 0", len(arcs))
	}
}
