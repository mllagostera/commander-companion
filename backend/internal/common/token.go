package common

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

// NewOpaqueToken genera un token aleatorio criptográficamente seguro, codificado en
// base64 URL-safe. Se usa para refresh tokens y tokens de verificación de email: el
// valor en claro se entrega una única vez al cliente, nunca se persiste (ver HashToken).
func NewOpaqueToken(byteLen int) (string, error) {
	buf := make([]byte, byteLen)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generating token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// HashToken calcula el hash SHA-256 (hex) que se persiste en base de datos en vez del
// token en claro.
func HashToken(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}
