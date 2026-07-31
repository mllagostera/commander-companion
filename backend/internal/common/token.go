package common

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

// NewOpaqueToken generates a cryptographically secure random token, encoded in
// URL-safe base64. Used for refresh tokens and email verification tokens: the
// plaintext value is delivered to the client only once, and never persisted (see HashToken).
func NewOpaqueToken(byteLen int) (string, error) {
	buf := make([]byte, byteLen)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generating token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// HashToken computes the SHA-256 (hex) hash that gets persisted in the database instead of the
// plaintext token.
func HashToken(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}
