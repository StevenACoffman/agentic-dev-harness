// Package model is the LLM seam for the five stages (SPEC §1-2). Stages drive
// their work through Client; Mock returns deterministic responses so the relay
// and its tests run without an API key or a per-role model.
package model

import (
	"context"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/authority"
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
// Class is the capability tier it reports to the model-gate (SPEC §5.1); an
// empty Class reports reasoning, so the ordinary relay passes the gate. It
// satisfies the Client interface each consuming package declares.
type Mock struct {
	Reply string
	Class authority.ModelClass
}

// Complete returns a fixed response and never makes a network call.
func (m Mock) Complete(_ context.Context, req Request) (Response, error) {
	reply := m.Reply
	if reply == "" {
		reply = "mock " + string(req.Role) + " output"
	}
	return Response{Text: reply}, nil
}

// ModelClass reports the capability tier this model binding runs at, defaulting
// to reasoning when unset.
func (m Mock) ModelClass() authority.ModelClass {
	if m.Class == "" {
		return authority.ClassReasoning
	}
	return m.Class
}
