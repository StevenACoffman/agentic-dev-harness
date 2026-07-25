// Package proof verifies a proof packet under NO-PROOF-NO-CLOSE (SPEC §5.4):
// every declared artifact exists on disk and its bytes hash to the recorded
// digest. The digest is identity.Hash (sha256[:16]), binding the packet to the
// exact bytes it covers.
package proof

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/identity"
)

// Artifact is one declared piece of proof: a repository-relative path and the
// expected content digest.
type Artifact struct {
	Path   string `json:"path"`
	Digest string `json:"sha256"`
}

// Packet is an arc's declared proof: the artifacts that must exist and match.
type Packet struct {
	Arc       string     `json:"arc"`
	Artifacts []Artifact `json:"artifacts"`
}

// Load reads a packet manifest from a JSON file.
func Load(path string) (Packet, error) {
	const op = "proof.Load"
	data, err := os.ReadFile(path)
	if err != nil {
		return Packet{}, &adh.Error{Op: op, Err: err}
	}
	var pkt Packet
	if err := json.Unmarshal(data, &pkt); err != nil {
		return Packet{}, &adh.Error{Op: op, Err: err}
	}
	return pkt, nil
}

// Verify checks that every artifact in pkt exists under root and matches its
// recorded digest. It returns EINVALID when the packet declares nothing and
// ECONFLICT on the first missing artifact or digest mismatch.
func Verify(root string, pkt *Packet) error {
	const op = "proof.Verify"
	if len(pkt.Artifacts) == 0 {
		return &adh.Error{Code: adh.EINVALID, Message: "proof packet declares no artifacts"}
	}
	for _, artifact := range pkt.Artifacts {
		data, err := os.ReadFile(filepath.Join(root, artifact.Path))
		switch {
		case os.IsNotExist(err):
			return &adh.Error{
				Code:    adh.ECONFLICT,
				Message: "missing proof artifact: " + artifact.Path,
			}
		case err != nil:
			return &adh.Error{Op: op, Err: err}
		}
		if got := identity.Hash(string(data)); got != artifact.Digest {
			return &adh.Error{
				Code:    adh.ECONFLICT,
				Message: "proof artifact digest mismatch: " + artifact.Path + " has " + got + ", want " + artifact.Digest,
			}
		}
	}
	return nil
}
