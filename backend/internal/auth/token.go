package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const refreshTokenBytes = 32

// ErrInvalidToken indica que un access o refresh token es inválido, expiró o fue revocado.
var ErrInvalidToken = errors.New("invalid or expired token")

// generateAccessToken firma un JWT de vida corta con el ID de usuario como subject.
func generateAccessToken(secret []byte, userID string, ttl time.Duration) (string, time.Time, error) {
	now := time.Now()
	expiresAt := now.Add(ttl)

	claims := jwt.RegisteredClaims{
		Subject:   userID,
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(expiresAt),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("signing access token: %w", err)
	}

	return signed, expiresAt, nil
}

// parseAccessToken valida la firma y expiración de un JWT y devuelve el user ID (subject).
func parseAccessToken(secret []byte, tokenString string) (string, error) {
	claims := &jwt.RegisteredClaims{}

	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("%w: unexpected signing method %v", ErrInvalidToken, t.Header["alg"])
		}
		return secret, nil
	})
	if err != nil || !token.Valid {
		return "", ErrInvalidToken
	}

	if claims.Subject == "" {
		return "", ErrInvalidToken
	}

	return claims.Subject, nil
}

// newRefreshTokenPlain genera un refresh token opaco (no JWT) criptográficamente aleatorio.
func newRefreshTokenPlain() (string, error) {
	buf := make([]byte, refreshTokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generating refresh token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// hashRefreshToken calcula el hash que se persiste en base de datos; el token en
// claro nunca se guarda, solo se entrega una vez al cliente.
func hashRefreshToken(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}
