// Package identity computes content-identity hashes used as no-op guards and
// cache keys across the harness self-optimization loop (SPEC-ADDITIONS §18.2).
// It is a port of SkillOpt's skill_hash: the first 16 hex characters of the
// SHA-256 of the content. Two inputs with the same Hash are byte-identical.
package identity

import (
	"crypto/sha256"
	"encoding/hex"
)

// hashHexLen is the number of leading hex characters kept, matching SkillOpt's
// skill_hash (16 characters = 64 bits, ample to detect an edit is a no-op).
const hashHexLen = 16

// Hash returns the first 16 hex characters of the SHA-256 of content. An
// unchanged Hash across an edit means the edit changed nothing.
func Hash(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])[:hashHexLen]
}
