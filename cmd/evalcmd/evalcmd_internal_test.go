package evalcmd

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/cmd/root"
	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/state"
)

// fakeAdjudicator confirms exactly the findings whose kind is failKind, so a test
// can drive the confirmed/blocking path without a real failing artifact.
type fakeAdjudicator struct{ failKind adh.FindingKind }

func (f fakeAdjudicator) Adjudicate(
	_ context.Context,
	finding adh.Finding,
) (ran, failed bool, err error) {
	return true, finding.Kind == f.failKind, nil
}

// TestExecBlocksViaInjectedAdjudicator exercises the Adjudicator seam: a fake
// confirms a device finding, so eval returns the arc to execution with exit 7.
func TestExecBlocksViaInjectedAdjudicator(t *testing.T) {
	t.Chdir(t.TempDir())
	arc := adh.Arc{
		ID:       "arc-0001",
		Stage:    adh.StageEvaluation,
		Status:   adh.StatusOpen,
		Findings: []adh.Finding{{Summary: "screen renders wrong", Kind: adh.FindingDevice}},
	}
	if err := state.Default().Save(&arc); err != nil {
		t.Fatalf("seed arc: %v", err)
	}
	var out bytes.Buffer
	cfg := &Config{
		Config: &root.Config{
			Stdout: &out,
			Stderr: &out,
			Getenv: func(string) string { return "" },
		},
		adjudicator: fakeAdjudicator{failKind: adh.FindingDevice},
	}
	err := cfg.exec(context.Background(), []string{"arc-0001"})
	var exit root.ExitError
	if !errors.As(err, &exit) || int(exit) != exitDevice {
		t.Fatalf("confirmed device finding = %v, want ExitError(%d)", err, exitDevice)
	}
	reloaded, getErr := state.Default().Get("arc-0001")
	if getErr != nil {
		t.Fatalf("reload arc: %v", getErr)
	}
	if reloaded.Stage != adh.StageExecution {
		t.Errorf("stage = %s, want execution", reloaded.Stage)
	}
}
