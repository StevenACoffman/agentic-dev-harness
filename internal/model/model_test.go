package model_test

import (
	"context"
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/internal/authority"
	"github.com/StevenACoffman/agentic-dev-harness/internal/model"
)

func TestRelayReturnsResponse(t *testing.T) {
	relay := model.Relay{Response: "the operator's reply"}
	resp, err := relay.Complete(context.Background(), model.Request{Prompt: "ignored"})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if resp.Text != "the operator's reply" {
		t.Errorf("Relay.Complete = %q, want the operator's reply", resp.Text)
	}
}

func TestModelClassDefaultsReasoning(t *testing.T) {
	tests := []struct {
		name string
		got  authority.ModelClass
		want authority.ModelClass
	}{
		{"relay unset", model.Relay{}.ModelClass(), authority.ClassReasoning},
		{"relay fast", model.Relay{Class: authority.ClassFast}.ModelClass(), authority.ClassFast},
		{"mock unset", model.Mock{}.ModelClass(), authority.ClassReasoning},
		{"mock fast", model.Mock{Class: authority.ClassFast}.ModelClass(), authority.ClassFast},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("ModelClass = %q, want %q", tt.got, tt.want)
			}
		})
	}
}
