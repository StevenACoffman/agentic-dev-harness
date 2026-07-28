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
	"github.com/StevenACoffman/agentic-dev-harness/internal/atomicfile"
	"github.com/StevenACoffman/agentic-dev-harness/internal/identity"
)

// Artifact is one declared piece of proof: a repository-relative path and the
// expected content digest.
type Artifact struct {
	Path   string `json:"path"`
	Digest string `json:"sha256"`
}

// Provenance records where a proof packet came from (SPEC §5.4): the commit the
// proof was created against. It is optional and informational — the gate binds a
// packet to its bytes by digest; the git SHA binds it to a commit. Screenshot
// provenance (dimensions, redaction method) is domain-specific and not modeled here.
type Provenance struct {
	GitSHA string `json:"git_sha,omitempty"`
}

// Packet is an arc's declared proof: the artifacts that must exist and match, plus
// optional provenance. Provenance is nil when none was recorded (e.g. no git repo).
type Packet struct {
	Arc        string      `json:"arc"`
	Provenance *Provenance `json:"provenance,omitempty"`
	Artifacts  []Artifact  `json:"artifacts"`
}

// Create builds a proof packet for arc by hashing each path's bytes under root
// (identity.Hash, sha256[:16]). Paths are recorded repo-relative, exactly as
// Verify resolves them. gitSHA records provenance — the commit the proof was
// created against (SPEC §5.4); an empty gitSHA records none (no git repo). The
// caller resolves the SHA and passes it, so proof needs no version-control
// dependency. A proof must cover real bytes: an empty path set is EINVALID and an
// unreadable artifact is an error, never a silent skip.
func Create(root, arc, gitSHA string, paths []string) (Packet, error) {
	const op = "proof.Create"
	if len(paths) == 0 {
		return Packet{}, &adh.Error{
			Code:    adh.EINVALID,
			Message: "proof create requires at least one artifact path",
		}
	}
	artifacts := make([]Artifact, 0, len(paths))
	for _, path := range paths {
		data, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			return Packet{}, &adh.Error{Op: op, Err: err}
		}
		artifacts = append(artifacts, Artifact{Path: path, Digest: identity.Hash(string(data))})
	}
	pkt := Packet{Arc: arc, Artifacts: artifacts}
	if gitSHA != "" {
		pkt.Provenance = &Provenance{GitSHA: gitSHA}
	}
	return pkt, nil
}

// Save writes a packet manifest to path as indented JSON, creating the directory
// if needed and writing atomically so a crash never leaves a half-written manifest.
func Save(path string, pkt *Packet) error {
	const op = "proof.Save"
	data, err := json.MarshalIndent(pkt, "", "  ")
	if err != nil {
		return &adh.Error{Op: op, Err: err}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return &adh.Error{Op: op, Err: err}
	}
	if err := atomicfile.WriteFile(path, append(data, '\n'), 0o600); err != nil {
		return &adh.Error{Op: op, Err: err}
	}
	return nil
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
// ECONFLICT on the first missing artifact or digest mismatch. Provenance is not a
// gate condition (SPEC §5.4): a packet verifies on its bytes regardless of the
// commit it records, so a proof created at one commit still verifies at another.
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
