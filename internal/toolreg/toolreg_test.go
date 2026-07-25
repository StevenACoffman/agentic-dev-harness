package toolreg_test

import (
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/toolreg"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name     string
		reg      toolreg.Registry
		wantCode string
	}{
		{
			name: "valid",
			reg: toolreg.Registry{Tools: []toolreg.Tool{
				{ID: "oracle-diff", Run: "make oracle", Verifies: "equivalence"},
			}},
			wantCode: "",
		},
		{
			name:     "empty id",
			reg:      toolreg.Registry{Tools: []toolreg.Tool{{Run: "x", Verifies: "y"}}},
			wantCode: adh.EINVALID,
		},
		{
			name:     "no run",
			reg:      toolreg.Registry{Tools: []toolreg.Tool{{ID: "t", Verifies: "y"}}},
			wantCode: adh.EINVALID,
		},
		{
			name:     "no verifies",
			reg:      toolreg.Registry{Tools: []toolreg.Tool{{ID: "t", Run: "x"}}},
			wantCode: adh.EINVALID,
		},
		{
			name: "duplicate id",
			reg: toolreg.Registry{Tools: []toolreg.Tool{
				{ID: "t", Run: "x", Verifies: "y"},
				{ID: "t", Run: "z", Verifies: "w"},
			}},
			wantCode: adh.EINVALID,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := adh.ErrorCode(tt.reg.Validate()); got != tt.wantCode {
				t.Errorf("Validate code = %q, want %q", got, tt.wantCode)
			}
		})
	}
}

func TestFindByVerifies(t *testing.T) {
	reg := toolreg.Registry{Tools: []toolreg.Tool{
		{ID: "oracle-diff", Run: "make oracle", Verifies: "reference-vs-native equivalence"},
	}}
	if tool, ok := reg.FindByVerifies(
		"reference-vs-native equivalence",
	); !ok ||
		tool.ID != "oracle-diff" {
		t.Errorf("FindByVerifies = (%v, %v), want oracle-diff", tool.ID, ok)
	}
	if _, ok := reg.FindByVerifies("nothing"); ok {
		t.Errorf("FindByVerifies found a nonexistent capability")
	}
}
