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
