package cmd

import "github.com/StevenACoffman/agentic-dev-harness/cmd/root"

// This file is compiled only under `go test` and adds no exported surface as
// consumers see the package.
//
// register is unexported because wiring the CLI is Run's job and no caller has another
// use for it. The --jsonl contract test does: it derives the command table from the
// registry rather than restating it, so a command added later is covered on the day it
// is registered instead of the day somebody remembers to add a line.

// RegisterForTest wires every command onto r, as Run does.
func RegisterForTest(r *root.Config) { register(r) }
