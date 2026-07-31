package cmd_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/cmd"
	"github.com/StevenACoffman/agentic-dev-harness/cmd/root"
	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/consolidate"
	"github.com/StevenACoffman/agentic-dev-harness/internal/identity"
	"github.com/StevenACoffman/agentic-dev-harness/internal/proof"
	"github.com/StevenACoffman/agentic-dev-harness/internal/state"
)

func TestHarnessEvalDispatch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "skill.md")
	doc := "# Skill\nIf the build fails, retry.\n## Boundary\nNot for zero-to-one work.\n"
	if err := os.WriteFile(path, []byte(doc), 0o600); err != nil {
		t.Fatalf("write artifact: %v", err)
	}
	out, err := run(t, "harness", "eval", path)
	if err != nil {
		t.Fatalf("harness eval returned error: %v", err)
	}
	if !strings.Contains(out, "det score: 100.0/100") {
		t.Errorf("harness eval output = %q, want a full det score", out)
	}
}

func TestHarnessEvalMinFloorPasses(t *testing.T) {
	path := filepath.Join(t.TempDir(), "skill.md")
	doc := "# Skill\nIf the build fails, retry.\n## Boundary\nNot for zero-to-one.\n"
	if err := os.WriteFile(path, []byte(doc), 0o600); err != nil {
		t.Fatalf("write artifact: %v", err)
	}
	if _, err := run(t, "harness", "eval", "--min", "50", path); err != nil {
		t.Errorf("eval --min 50 on a 100-scoring doc should pass, got %v", err)
	}
}

func TestHarnessEvalMinFloorFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "skill.md")
	doc := "# Skill\nIf the build fails, retry.\n## Boundary\nNot for zero-to-one.\n"
	if err := os.WriteFile(path, []byte(doc), 0o600); err != nil {
		t.Fatalf("write artifact: %v", err)
	}
	_, err := run(t, "harness", "eval", "--min", "101", path)
	var exit root.ExitError
	if !errors.As(err, &exit) || int(exit) != 1 {
		t.Fatalf("eval below --min floor = %v, want ExitError(1)", err)
	}
}

func TestHarnessUnknownVerb(t *testing.T) {
	if _, err := run(t, "harness", "frobnicate"); err == nil {
		t.Errorf("unknown harness verb should return an error")
	}
}

func run(t *testing.T, args ...string) (string, error) {
	t.Helper()
	return runWithEnv(t, nil, args...)
}

// runWithEnv drives cmd.Run with an injected environment (nil = empty), so a
// test can exercise ADH_* config precedence without touching the process env.
func runWithEnv(t *testing.T, env map[string]string, args ...string) (string, error) {
	t.Helper()
	getenv := func(key string) string { return env[key] }
	var out, errb bytes.Buffer
	err := cmd.Run(context.Background(), args, getenv, strings.NewReader(""), &out, &errb)
	return out.String(), err
}

// mustRun runs a command expected to succeed, returning its stdout.
func mustRun(t *testing.T, args ...string) string {
	t.Helper()
	out, err := run(t, args...)
	if err != nil {
		t.Fatalf("%v: %v", args, err)
	}
	return out
}

// parkedAtOps creates an arc, relays it to the ops gate, and approves it, so it
// is open at ops ready to close. It returns the arc id.
func parkedAtOps(t *testing.T) string {
	t.Helper()
	id := strings.TrimSpace(mustRun(t, "arc", "new", "ship me"))
	mustRun(t, "run", id)                     // parks at the ops gate (blocked)
	mustRun(t, "approve", "--phrase", id, id) // unblocks: open at ops
	return id
}

func TestCloseBeforeApproveFails(t *testing.T) {
	t.Chdir(t.TempDir())
	id := strings.TrimSpace(mustRun(t, "arc", "new", "x"))
	mustRun(t, "run", id) // blocked at ops, not approved
	if _, err := run(t, "close", id); err == nil {
		t.Errorf("close of a blocked (unapproved) arc should fail")
	}
}

func TestCloseWithoutProofExit8(t *testing.T) {
	t.Chdir(t.TempDir())
	id := parkedAtOps(t)
	_, err := run(t, "close", id) // no --proof
	var exit root.ExitError
	if !errors.As(err, &exit) || int(exit) != 8 {
		t.Fatalf("close without proof = %v, want ExitError(8) (NO-PROOF-NO-CLOSE)", err)
	}
}

// writeProof lays down a proof artifact and a matching manifest.json in the
// working directory for the given arc.
func writeProof(t *testing.T, arcID string) {
	t.Helper()
	body := "screenshot-bytes"
	if err := os.WriteFile("proof.txt", []byte(body), 0o600); err != nil {
		t.Fatalf("write proof artifact: %v", err)
	}
	pkt := proof.Packet{
		Arc:       arcID,
		Artifacts: []proof.Artifact{{Path: "proof.txt", Digest: identity.Hash(body)}},
	}
	data, err := json.Marshal(pkt)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	if err := os.WriteFile("manifest.json", data, 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
}

func TestCloseWithProofCloses(t *testing.T) {
	t.Chdir(t.TempDir())
	id := parkedAtOps(t)
	writeProof(t, id)
	mustRun(t, "close", "--proof", "manifest.json", id)
	if show := mustRun(t, "arc", "show", id); !strings.Contains(show, "closed") {
		t.Errorf("arc not closed after close with proof:\n%s", show)
	}
}

func TestCloseRecordsMetric(t *testing.T) {
	t.Chdir(t.TempDir())
	id := parkedAtOps(t)
	writeProof(t, id)
	mustRun(t, "close", "--proof", "manifest.json", id)
	data, err := os.ReadFile(filepath.Join(".adh", "metrics.json"))
	if err != nil {
		t.Fatalf("read metrics ledger: %v", err)
	}
	if !strings.Contains(string(data), id) {
		t.Errorf("close did not record a metric for %s:\n%s", id, data)
	}
	if out := mustRun(t, "metrics"); !strings.Contains(out, "accepted:") {
		t.Errorf("metrics summary missing the accepted line:\n%s", out)
	}
}

func TestCloseBadResolutionFails(t *testing.T) {
	t.Chdir(t.TempDir())
	id := parkedAtOps(t)
	if _, err := run(t, "close", "--as", "bogus", id); err == nil {
		t.Errorf("close --as bogus should fail on an unknown resolution")
	}
}

func TestHarnessGateAcceptDispatch(t *testing.T) {
	out, err := run(t, "harness", "gate", "--candidate", "90", "--current", "84")
	if err != nil {
		t.Fatalf("harness gate accept returned error: %v", err)
	}
	if !strings.Contains(out, "accept_new_best") {
		t.Errorf("harness gate accept output = %q, want accept_new_best", out)
	}
}

func TestHarnessGateRejectExitCode(t *testing.T) {
	_, err := run(t, "harness", "gate", "--candidate", "80", "--current", "84")
	var exit root.ExitError
	if !errors.As(err, &exit) || int(exit) != 1 {
		t.Fatalf("harness gate reject error = %v, want ExitError(1)", err)
	}
}

func TestOracleSelfTestDispatch(t *testing.T) {
	out, err := run(t, "oracle", "selftest")
	if err != nil {
		t.Fatalf("oracle selftest returned error: %v", err)
	}
	if !strings.Contains(out, "passed") {
		t.Errorf("oracle selftest output = %q, want a pass", out)
	}
}

func TestUnknownVerb(t *testing.T) {
	if _, err := run(t, "arc", "frobnicate"); err == nil {
		t.Errorf("unknown arc verb should return an error")
	}
}

func TestWorkerRequalifyUsesConfig(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.MkdirAll(".adh", 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	body := "[models]\nreasoning = \"strong\"\nfast = \"quick\"\n"
	if err := os.WriteFile(filepath.Join(".adh", "config.toml"), []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	mustRun(t, "worker", "requalify")
	show := mustRun(t, "worker", "show")
	if !strings.Contains(show, "strong") || !strings.Contains(show, "quick") {
		t.Errorf("worker epoch should bind the configured models:\n%s", show)
	}
}

func TestApproveRequiresPhrase(t *testing.T) {
	t.Chdir(t.TempDir())
	id := strings.TrimSpace(mustRun(t, "arc", "new", "x"))
	mustRun(t, "run", id) // blocked at ops
	_, err := run(t, "approve", id)
	var exit root.ExitError
	if !errors.As(err, &exit) || int(exit) != 4 {
		t.Fatalf("approve without --phrase = %v, want ExitError(4)", err)
	}
}

func TestRunBlocksAtOpsGate(t *testing.T) {
	t.Chdir(t.TempDir())
	out, err := run(t, "arc", "new", "ship me")
	if err != nil {
		t.Fatalf("arc new: %v", err)
	}
	id := strings.TrimSpace(out)
	if _, err := run(t, "run", id); err != nil {
		t.Fatalf("run: %v", err)
	}
	show, err := run(t, "arc", "show", id)
	if err != nil {
		t.Fatalf("arc show: %v", err)
	}
	if !strings.Contains(show, "blocked") || !strings.Contains(show, "ops") {
		t.Errorf("arc after run =\n%s\nwant blocked at ops", show)
	}
}

func TestRunHonorsAutonomyEnv(t *testing.T) {
	t.Chdir(t.TempDir())
	id := strings.TrimSpace(mustRun(t, "arc", "new", "gated"))
	// At L1 the relay stops after the first stage (AutoAdvances(strategy, L1) is
	// false), parking blocked at execution rather than relaying to the ops gate.
	if _, err := runWithEnv(t, map[string]string{"ADH_AUTONOMY": "L1"}, "run", id); err != nil {
		t.Fatalf("run: %v", err)
	}
	show := mustRun(t, "arc", "show", id)
	if !strings.Contains(show, "blocked") || !strings.Contains(show, "execution") {
		t.Errorf("ADH_AUTONOMY=L1 should park the arc blocked at execution:\n%s", show)
	}
}

func TestAutonomySetLowersGate(t *testing.T) {
	t.Chdir(t.TempDir())
	mustRun(t, "autonomy", "set", "L0")
	if show := mustRun(t, "autonomy", "show"); !strings.Contains(show, "L0") {
		t.Errorf("autonomy show = %q, want L0 after set", show)
	}
	id := strings.TrimSpace(mustRun(t, "arc", "new", "gated"))
	mustRun(t, "run", id)
	show := mustRun(t, "arc", "show", id)
	if !strings.Contains(show, "blocked") || !strings.Contains(show, "execution") {
		t.Errorf("at L0 (.adh/autonomy) the relay should block at execution:\n%s", show)
	}
}

func TestStepRefusesOps(t *testing.T) {
	t.Chdir(t.TempDir())
	out, _ := run(t, "arc", "new", "ship me")
	id := strings.TrimSpace(out)
	if _, err := run(t, "run", id); err != nil { // relay parks it at the ops gate
		t.Fatalf("run: %v", err)
	}
	if _, err := run(t, "approve", "--phrase", id, id); err != nil { // unblock, still at ops
		t.Fatalf("approve: %v", err)
	}
	if _, err := run(t, "step", id); err == nil {
		t.Errorf("step at ops should refuse (ship via close)")
	}
}

// selectionFailure returns a failure class that hashes to the selection split,
// so a seeded arc's mined task is held out for acceptance. Deterministic.
func selectionFailure(t *testing.T) string {
	t.Helper()
	for i := range 200 {
		class := fmt.Sprintf("missing-thing-%d", i)
		if consolidate.SplitFor(class) == consolidate.SplitSelection {
			return class
		}
	}
	t.Fatal("no synthetic class hashed to the selection split")
	return ""
}

// seedSleepWorkspace writes two closed arcs sharing one failure class and a
// realistically sized managed artifact into the current (temp) directory.
func seedSleepWorkspace(t *testing.T, failure string) {
	t.Helper()
	store := state.Default()
	for _, id := range []string{"arc-0001", "arc-0002"} {
		arc := adh.Arc{ID: id, Status: adh.StatusClosed, History: []string{"critic: " + failure}}
		if err := store.Save(&arc); err != nil {
			t.Fatalf("seed arc %s: %v", id, err)
		}
	}
	if err := os.MkdirAll(".adh/context", 0o750); err != nil {
		t.Fatalf("mkdir context: %v", err)
	}
	art := "# Harness\n\n" + strings.Repeat("Guidance for doing the work well and safely.\n", 6)
	if err := os.WriteFile(".adh/context/harness.md", []byte(art), 0o600); err != nil {
		t.Fatalf("seed artifact: %v", err)
	}
}

func stagedIDs(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(".adh/sleep/staging")
	if err != nil {
		t.Fatalf("read staging: %v", err)
	}
	var ids []string
	for _, entry := range entries {
		if entry.IsDir() {
			ids = append(ids, entry.Name())
		}
	}
	return ids
}

func TestSleepRunStagesProposal(t *testing.T) {
	t.Chdir(t.TempDir())
	seedSleepWorkspace(t, selectionFailure(t))
	_, err := run(t, "sleep", "run")
	var exit root.ExitError
	if !errors.As(err, &exit) || int(exit) != 14 {
		t.Fatalf("sleep run = %v, want ExitError(14) (proposal staged, adoption pending)", err)
	}
	if ids := stagedIDs(t); len(ids) != 1 {
		t.Fatalf("staged %d proposals, want exactly 1", len(ids))
	}
}

func TestSleepAdoptAppliesProposal(t *testing.T) {
	t.Chdir(t.TempDir())
	failure := selectionFailure(t)
	seedSleepWorkspace(t, failure)
	_, _ = run(t, "sleep", "run") // stages (ExitError(14))
	ids := stagedIDs(t)
	if len(ids) != 1 {
		t.Fatalf("expected one staged id, got %v", ids)
	}
	if _, err := run(t, "sleep", "adopt", ids[0]); err != nil {
		t.Fatalf("sleep adopt: %v", err)
	}
	live, err := os.ReadFile(".adh/context/harness.md")
	if err != nil {
		t.Fatalf("read adopted artifact: %v", err)
	}
	if !strings.Contains(string(live), failure) {
		t.Errorf("adopted artifact does not contain the learned class %q", failure)
	}
	if !strings.Contains(string(live), "ADH:LEARNED") {
		t.Errorf("adopted artifact is missing the protected-region marker")
	}
}

func TestSleepRunNoImprovement(t *testing.T) {
	t.Chdir(t.TempDir())
	arc := adh.Arc{
		ID:      "arc-0001",
		Status:  adh.StatusClosed,
		History: []string{"critic: looks good"},
	}
	if err := state.Default().Save(&arc); err != nil {
		t.Fatalf("seed arc: %v", err)
	}
	out, err := run(t, "sleep", "run")
	if err != nil {
		t.Fatalf("sleep run with nothing to learn should exit 0, got %v", err)
	}
	if !strings.Contains(out, "no proposal staged") {
		t.Errorf("output = %q, want a self-explanation", out)
	}
}

func TestSleepRunReportsLongitudinal(t *testing.T) {
	t.Chdir(t.TempDir())
	seedSleepWorkspace(t, selectionFailure(t))
	out, _ := run(t, "sleep", "run") // stages (ExitError(14))
	if !strings.Contains(out, "longitudinal") {
		t.Errorf("run summary = %q, want a longitudinal report", out)
	}
	guidance := filepath.Join(".adh", "sleep", "staging", stagedIDs(t)[0], "longitudinal.md")
	if _, err := os.Stat(guidance); err != nil {
		t.Errorf("slow-update guidance not staged: %v", err)
	}
}

func TestSleepUnknownVerb(t *testing.T) {
	if _, err := run(t, "sleep", "frobnicate"); err == nil {
		t.Errorf("unknown sleep verb should return an error")
	}
}
