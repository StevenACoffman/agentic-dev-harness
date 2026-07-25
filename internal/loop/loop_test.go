package loop_test

import (
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/loop"
)

func TestValidate(t *testing.T) {
	valid := loop.Loop{
		ID: "dep-scan", Goal: "no vulnerable dep ships", Sensor: "adh tool run dep-scan",
		RetireWhen: "policy moves into a repo check",
	}
	tests := []struct {
		name     string
		reg      loop.Registry
		wantCode string
	}{
		{name: "valid", reg: loop.Registry{Loops: []loop.Loop{valid}}, wantCode: ""},
		{
			name:     "no retirement condition",
			reg:      loop.Registry{Loops: []loop.Loop{{ID: "x", Goal: "g", Sensor: "s"}}},
			wantCode: adh.EINVALID,
		},
		{
			name:     "no sensor",
			reg:      loop.Registry{Loops: []loop.Loop{{ID: "x", Goal: "g", RetireWhen: "w"}}},
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

func TestFind(t *testing.T) {
	reg := loop.Registry{
		Loops: []loop.Loop{{ID: "dep-scan", Goal: "g", Sensor: "s", RetireWhen: "w"}},
	}
	if got, ok := reg.Find("dep-scan"); !ok || got.ID != "dep-scan" {
		t.Errorf("Find = (%v, %v), want dep-scan", got.ID, ok)
	}
	if _, ok := reg.Find("missing"); ok {
		t.Errorf("Find found a nonexistent loop")
	}
}
