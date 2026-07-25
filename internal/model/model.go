// Package model is the LLM seam for the five stages (SPEC §1-2). Stages drive
// their work through Client; Mock returns deterministic responses so the relay
// and its tests run without an API key or a per-role model.
package model

import (
	"context"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
)

// Request is one model call: the acting stage and its prompt.
type Request struct {
	Role   adh.Stage
	Prompt string
}

// Response is a model completion.
type Response struct {
	Text string
}

// Mock is a deterministic model client. When Reply is empty it echoes the role.
// It satisfies the Client interface each consuming package declares.
type Mock struct {
	Reply string
}

// Complete returns a fixed response and never makes a network call.
func (m Mock) Complete(_ context.Context, req Request) (Response, error) {
	reply := m.Reply
	if reply == "" {
		reply = "mock " + string(req.Role) + " output"
	}
	return Response{Text: reply}, nil
}
