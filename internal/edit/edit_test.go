package edit_test

import (
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/internal/edit"
)

func TestWithinSizeBudget(t *testing.T) {
	tests := []struct {
		name      string
		orig, new int
		ratio     float64
		want      bool
	}{
		{name: "same size", orig: 100, new: 100, ratio: 1.5, want: true},
		{name: "at the cap", orig: 100, new: 150, ratio: 1.5, want: true},
		{name: "over the cap", orig: 100, new: 151, ratio: 1.5, want: false},
		{name: "empty from nothing", orig: 0, new: 0, ratio: 1.5, want: true},
		{name: "growth from nothing", orig: 0, new: 1, ratio: 1.5, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := edit.WithinSizeBudget(tt.orig, tt.new, tt.ratio); got != tt.want {
				t.Errorf(
					"WithinSizeBudget(%d,%d,%v) = %v, want %v",
					tt.orig,
					tt.new,
					tt.ratio,
					got,
					tt.want,
				)
			}
		})
	}
}

func TestIsNoOp(t *testing.T) {
	if !edit.IsNoOp("same", "same") {
		t.Errorf("identical content should be a no-op")
	}
	if edit.IsNoOp("before", "after") {
		t.Errorf("changed content should not be a no-op")
	}
}
