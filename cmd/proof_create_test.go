package cmd_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/state"
	"github.com/StevenACoffman/agentic-dev-harness/internal/vcs"
	"github.com/StevenACoffman/skillet/proof"
)

func TestProofCreateWritesManifestAndRecordsOnArc(t *testing.T) {
	t.Chdir(t.TempDir())
	arc := adh.Arc{
		ID:         "arc-0001",
		Stage:      adh.StageOps,
		Status:     adh.StatusOpen,
		Resolution: adh.ResolutionChange,
	}
	if err := state.Default().Save(&arc); err != nil {
		t.Fatalf("seed arc: %v", err)
	}
	if err := os.WriteFile("result.txt", []byte("the finding"), 0o600); err != nil {
		t.Fatalf("write artifact: %v", err)
	}

	out := mustRun(t, "proof", "create", arc.ID, "result.txt")
	if !strings.Contains(out, "proof created") || !strings.Contains(out, "arc-0001") {
		t.Errorf("create output = %q, want a created notice", out)
	}

	// The manifest exists at the default path and verifies.
	manifest := filepath.Join(".adh", "proof", "arc-0001.json")
	if _, err := os.Stat(manifest); err != nil {
		t.Fatalf("manifest not written: %v", err)
	}
	pkt, err := proof.Load(manifest)
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}
	if err := proof.Verify(".", &pkt); err != nil {
		t.Errorf("created manifest should verify: %v", err)
	}

	// The arc now points at the manifest, closing the grounding/close loop.
	reloaded, err := state.Default().Get(arc.ID)
	if err != nil {
		t.Fatalf("reload arc: %v", err)
	}
	if reloaded.Proof != manifest {
		t.Errorf("arc.Proof = %q, want %q", reloaded.Proof, manifest)
	}
}

func TestProofCreateVerifyRoundTrip(t *testing.T) {
	t.Chdir(t.TempDir())
	arc := adh.Arc{ID: "arc-0001", Stage: adh.StageOps, Status: adh.StatusOpen}
	if err := state.Default().Save(&arc); err != nil {
		t.Fatalf("seed arc: %v", err)
	}
	if err := os.WriteFile("proof.txt", []byte("evidence"), 0o600); err != nil {
		t.Fatalf("write artifact: %v", err)
	}
	// --out after the create subcommand now parses (nested ff subcommand).
	mustRun(t, "proof", "create", "--out", "packet.json", arc.ID, "proof.txt")
	if _, err := os.Stat("packet.json"); err != nil {
		t.Fatalf("--out manifest not written: %v", err)
	}
	if _, err := run(t, "proof", "verify", "packet.json"); err != nil {
		t.Errorf("verify of a just-created packet should pass, got %v", err)
	}
}

func TestProofCreateRecordsGitProvenance(t *testing.T) {
	t.Chdir(t.TempDir())
	repo, err := vcs.Init(".")
	if err != nil {
		t.Fatalf("git init: %v", err)
	}
	if err := os.WriteFile("result.txt", []byte("the finding"), 0o600); err != nil {
		t.Fatalf("write artifact: %v", err)
	}
	// Commit so HEAD names a real SHA; proof create should record it as provenance.
	who := vcs.Signature{Name: "adh", Email: "adh@example.test"}
	if _, err := repo.Commit(
		"seed",
		who,
		time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC),
	); err != nil {
		t.Fatalf("commit: %v", err)
	}
	arc := adh.Arc{ID: "arc-0001", Stage: adh.StageOps, Status: adh.StatusOpen}
	if err := state.Default().Save(&arc); err != nil {
		t.Fatalf("seed arc: %v", err)
	}

	out := mustRun(t, "proof", "create", arc.ID, "result.txt")
	if !strings.Contains(out, "(git ") {
		t.Errorf("create output = %q, want it to name the git provenance", out)
	}

	sha, err := repo.HeadSHA()
	if err != nil {
		t.Fatalf("HeadSHA: %v", err)
	}
	pkt, err := proof.Load(filepath.Join(".adh", "proof", "arc-0001.json"))
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}
	if pkt.Provenance == nil || pkt.Provenance.GitSHA != sha {
		t.Errorf("manifest provenance = %+v, want git SHA %q", pkt.Provenance, sha)
	}
}

func TestProofCreateMissingArtifactErrors(t *testing.T) {
	t.Chdir(t.TempDir())
	arc := adh.Arc{ID: "arc-0001", Stage: adh.StageOps, Status: adh.StatusOpen}
	if err := state.Default().Save(&arc); err != nil {
		t.Fatalf("seed arc: %v", err)
	}
	if _, err := run(t, "proof", "create", arc.ID, "does-not-exist.txt"); err == nil {
		t.Error("create over a missing artifact should error")
	}
}
