package device_test

import (
	"context"
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/internal/device"
)

func TestMockValidate(t *testing.T) {
	tests := []struct {
		name    string
		healthy bool
		wantOK  bool
	}{
		{name: "healthy", healthy: true, wantOK: true},
		{name: "unhealthy", healthy: false, wantOK: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rep, err := device.Mock{Healthy: tt.healthy}.Validate(context.Background())
			if err != nil {
				t.Fatalf("Validate: %v", err)
			}
			if rep.OK != tt.wantOK {
				t.Errorf("Report.OK = %v, want %v", rep.OK, tt.wantOK)
			}
		})
	}
}
