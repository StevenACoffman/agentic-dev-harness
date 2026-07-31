// Package model is the LLM seam for the five stages (SPEC §1-2). Stages drive
// their work through Client. Mock returns deterministic responses so the relay
// and its tests run without an API key or a per-role model. Relay carries a
// response supplied out of band by an operator (Claude driving adh via a skill),
// so the same Execute path advances an arc whether the reasoning came from a
// mock, a relayed operator, or a real API client.
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

// Relay is a Client whose completion is supplied out of band: the step command
// emits the stage's prompt to an operator (Claude driving adh via a skill) and
// feeds the operator's reply back on a second invocation as Response. Relay
// returns that reply from Complete, so the arc advances through the ordinary
// Execute path. It never makes a network call. Class is the tier it reports to
// the model-gate (SPEC §5.1); an empty Class reports reasoning, because the
// operator supplying the reply is treated as a reasoning-class worker.
type Relay struct {
	Response string
	Class    authority.ModelClass
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
	return defaultClass(m.Class)
}

// Complete returns the operator-supplied response verbatim. The request is
// ignored: its prompt was already emitted when the turn was opened.
func (r Relay) Complete(_ context.Context, _ Request) (Response, error) {
	return Response{Text: r.Response}, nil
}

// ModelClass reports the capability tier the relayed worker runs at, defaulting
// to reasoning when unset so judgment roles pass the gate.
func (r Relay) ModelClass() authority.ModelClass {
	return defaultClass(r.Class)
}

// defaultClass resolves an unset model class to reasoning, the tier a judgment
// role requires (SPEC §5.1). It is the single source of the default so Mock and
// Relay cannot drift apart.
func defaultClass(c authority.ModelClass) authority.ModelClass {
	if c == "" {
		return authority.ClassReasoning
	}
	return c
}
