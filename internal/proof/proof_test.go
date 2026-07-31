package proof_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/identity"
	"github.com/StevenACoffman/agentic-dev-harness/internal/proof"
)

func writeArtifact(t *testing.T, root, name, body string) proof.Artifact {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o600); err != nil {
		t.Fatalf("write artifact: %v", err)
	}
	return proof.Artifact{Path: name, Digest: identity.Hash(body)}
}

func TestVerify(t *testing.T) {
	root := t.TempDir()
	good := writeArtifact(t, root, "screenshot.txt", "pixels")

	tests := []struct {
		name     string
		pkt      proof.Packet
		wantCode string
	}{
		{
			name:     "match",
			pkt:      proof.Packet{Arc: "arc-0001", Artifacts: []proof.Artifact{good}},
			wantCode: "",
		},
		{name: "no artifacts", pkt: proof.Packet{Arc: "arc-0001"}, wantCode: adh.EINVALID},
		{
			name: "missing file",
			pkt: proof.Packet{
				Artifacts: []proof.Artifact{{Path: "gone.txt", Digest: good.Digest}},
			},
			wantCode: adh.ECONFLICT,
		},
		{
			name: "digest mismatch",
			pkt: proof.Packet{
				Artifacts: []proof.Artifact{{Path: "screenshot.txt", Digest: "0000000000000000"}},
			},
			wantCode: adh.ECONFLICT,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkt := tt.pkt
			if got := adh.ErrorCode(proof.Verify(root, &pkt)); got != tt.wantCode {
				t.Errorf("Verify code = %q, want %q", got, tt.wantCode)
			}
		})
	}
}

func TestCreateHashesSoVerifyPasses(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("alpha"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "b.txt"), []byte("beta"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	// No git SHA passed: the packet records no provenance.
	pkt, err := proof.Create(root, "arc-0001", "", []string{"a.txt", "b.txt"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if pkt.Arc != "arc-0001" || len(pkt.Artifacts) != 2 {
		t.Fatalf("packet = %+v, want arc-0001 with 2 artifacts", pkt)
	}
	if pkt.Provenance != nil {
		t.Errorf("provenance = %+v, want nil with no git SHA", pkt.Provenance)
	}
	if pkt.Artifacts[0].Digest != identity.Hash("alpha") {
		t.Errorf("digest = %q, want the identity hash of the bytes", pkt.Artifacts[0].Digest)
	}
	if err := proof.Verify(root, &pkt); err != nil {
		t.Errorf("a freshly created packet should verify, got %v", err)
	}
}

func TestCreateRecordsProvenance(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("alpha"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	const sha = "0123456789abcdef0123456789abcdef01234567"
	pkt, err := proof.Create(root, "arc-0001", sha, []string{"a.txt"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if pkt.Provenance == nil || pkt.Provenance.GitSHA != sha {
		t.Errorf("provenance = %+v, want git SHA %q", pkt.Provenance, sha)
	}
	// Provenance is not a gate: the packet still verifies on its bytes, and the
	// recorded SHA does not have to match anything on disk.
	if err := proof.Verify(root, &pkt); err != nil {
		t.Errorf("a packet with provenance should verify on bytes, got %v", err)
	}
}

func TestCreateMissingFileErrors(t *testing.T) {
	if _, err := proof.Create(t.TempDir(), "arc-0001", "", []string{"absent.txt"}); err == nil {
		t.Error("Create over a missing artifact should error, not skip it")
	}
}

func TestCreateEmptyPathsIsInvalid(t *testing.T) {
	_, err := proof.Create(t.TempDir(), "arc-0001", "", nil)
	if adh.ErrorCode(err) != adh.EINVALID {
		t.Errorf("Create with no paths = %v, want EINVALID", err)
	}
}

func TestSaveRoundTrips(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("alpha"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	pkt, err := proof.Create(root, "arc-0001", "deadbeefcafe", []string{"a.txt"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	manifest := filepath.Join(root, "nested", "packet.json")
	if err := proof.Save(manifest, &pkt); err != nil {
		t.Fatalf("Save: %v", err)
	}
	loaded, err := proof.Load(manifest)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Arc != pkt.Arc || len(loaded.Artifacts) != 1 ||
		loaded.Artifacts[0] != pkt.Artifacts[0] {
		t.Errorf("round-trip mismatch: saved %+v, loaded %+v", pkt, loaded)
	}
	// Provenance survives the round-trip.
	if loaded.Provenance == nil || loaded.Provenance.GitSHA != "deadbeefcafe" {
		t.Errorf("round-trip lost provenance: %+v", loaded.Provenance)
	}
}
