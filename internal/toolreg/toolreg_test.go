package toolreg_test

import (
	"os"
	"path/filepath"
	"reflect"
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
		{
			name: "malformed kpi",
			reg: toolreg.Registry{Tools: []toolreg.Tool{
				{
					ID:       "t",
					Run:      "x",
					Verifies: "y",
					KPIs:     []adh.KPI{{Metric: "run_failure", Direction: "sideways"}},
				},
			}},
			wantCode: adh.EINVALID,
		},
		{
			name: "valid kpi",
			reg: toolreg.Registry{Tools: []toolreg.Tool{
				{ID: "t", Run: "x", Verifies: "y", KPIs: []adh.KPI{
					{Metric: "run_failure", Threshold: 2, Direction: adh.WorseWhenAbove},
				}},
			}},
			wantCode: "",
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

// TestStarterRegistryValid: the seeded starter registry is a valid registry, so
// `adh init` writes a file `tool doctor` accepts and every entry is discoverable.
func TestStarterRegistryValid(t *testing.T) {
	reg := toolreg.StarterRegistry()
	if err := reg.Validate(); err != nil {
		t.Fatalf("StarterRegistry().Validate() = %v, want nil", err)
	}
	if len(reg.Tools) == 0 {
		t.Fatal("StarterRegistry() is empty")
	}
	for _, id := range []string{"modelith-lint", "modelith-render-check", "skillsaw-eval", "exegesis-verify"} {
		if _, ok := reg.FindByID(id); !ok {
			t.Errorf("StarterRegistry() missing tool %q", id)
		}
	}
}

// TestMarshalRoundTrips: Marshal's output decodes back to the same registry, so the
// write side (init) and read side (Load) of the registry file agree.
func TestMarshalRoundTrips(t *testing.T) {
	want := toolreg.StarterRegistry()
	data, err := toolreg.Marshal(want)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	path := filepath.Join(t.TempDir(), "tools.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := toolreg.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got.Tools) != len(want.Tools) {
		t.Fatalf("round-trip tool count = %d, want %d", len(got.Tools), len(want.Tools))
	}
	for i := range want.Tools {
		if !reflect.DeepEqual(got.Tools[i], want.Tools[i]) {
			t.Errorf("round-trip tool %d = %+v, want %+v", i, got.Tools[i], want.Tools[i])
		}
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

func TestFindByID(t *testing.T) {
	reg := toolreg.Registry{Tools: []toolreg.Tool{
		{ID: "nfr-lint", Run: "golangci-lint run", Verifies: "style floor"},
	}}
	if tool, ok := reg.FindByID("nfr-lint"); !ok || tool.Run != "golangci-lint run" {
		t.Errorf("FindByID = (%q, %v), want the nfr-lint command", tool.Run, ok)
	}
	if _, ok := reg.FindByID("absent"); ok {
		t.Errorf("FindByID found a nonexistent tool")
	}
}
