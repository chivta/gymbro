// Package token makes the random bearer values of web sign-in (login tokens,
// browser nonces, session tokens) and the hashes stored in their place: the
// database never sees a raw token.
package token

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
)

// size is the entropy of a token in bytes. Encoded it is 43 characters of
// base64url without padding (A-Za-z0-9_-), so a login token is also a valid
// Telegram /start payload (that alphabet, at most 64 characters).
const size = 32

// New returns a fresh random token.
func New() string {
	b := make([]byte, size)
	// crypto/rand.Read never returns an error and panics if the OS source fails.
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// Hash is the value stored and looked up instead of the raw token. Tokens are
// high-entropy, so a plain SHA-256 suffices.
func Hash(raw string) []byte {
	sum := sha256.Sum256([]byte(raw))
	return sum[:]
}
