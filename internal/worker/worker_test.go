package worker_test

import (
	"path/filepath"
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/internal/worker"
)

func TestRequalRequired(t *testing.T) {
	base := worker.Epoch{
		ID:     "e1",
		Models: map[string]string{"critic": "reasoning", "execution": "fast"},
	}
	tests := []struct {
		name    string
		current worker.Epoch
		want    bool
	}{
		{name: "same bindings", current: base, want: false},
		{
			name: "changed model",
			current: worker.Epoch{
				Models: map[string]string{"critic": "reasoning-2", "execution": "fast"},
			},
			want: true,
		},
		{
			name: "added role",
			current: worker.Epoch{
				Models: map[string]string{
					"critic":    "reasoning",
					"execution": "fast",
					"ops":       "fast",
				},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := base.RequalRequired(tt.current); got != tt.want {
				t.Errorf("RequalRequired = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "worker.json")
	want := worker.Epoch{ID: "e1", Models: map[string]string{"critic": "reasoning"}}
	if err := worker.Save(path, want); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := worker.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.ID != want.ID || got.Models["critic"] != "reasoning" {
		t.Errorf("round trip = %+v, want %+v", got, want)
	}
}

func TestLoadAbsent(t *testing.T) {
	got, err := worker.Load(filepath.Join(t.TempDir(), "nope.json"))
	if err != nil {
		t.Fatalf("Load absent: %v", err)
	}
	if got.ID != "" {
		t.Errorf("absent epoch = %+v, want zero", got)
	}
}
