// Package auth provides passwordless magic-link primitives: token generation,
// hashing for at-rest storage, and session-id minting. There are no passwords
// anywhere in the system.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
)

// SessionCookieName is the HttpOnly cookie that carries the opaque session id.
const SessionCookieName = "lost_session"

// NewToken returns a URL-safe random token (used in the magic-link URL) and its
// SHA-256 hash (stored server-side). Only the hash is persisted, so a database
// leak does not expose usable login links.
func NewToken() (raw, hash string) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err) // crypto/rand failure is unrecoverable
	}
	raw = base64.RawURLEncoding.EncodeToString(b)
	return raw, HashToken(raw)
}

// HashToken hashes a raw token for constant-time-safe lookup by digest.
func HashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// NewSessionID mints an opaque, unguessable session identifier.
func NewSessionID() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
