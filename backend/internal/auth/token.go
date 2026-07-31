package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/usuario/commander-companion-backend/internal/common"
)

const refreshTokenBytes = 32

// ErrInvalidToken indicates that an access or refresh token is invalid, expired, or was revoked.
var ErrInvalidToken = common.Unauthorized("invalid or expired token")

// generateAccessToken signs a short-lived JWT with the user ID as the subject.
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

// parseAccessToken validates the signature and expiration of a JWT and returns the user ID (subject).
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

// VerifyAccessToken validates the signature and expiration of an access token JWT and
// returns the user ID (subject). Exported (unlike parseAccessToken) so that modules that
// can't authenticate via the Authorization header of a normal HTTP request —like
// internal/websocket, where the handshake is done by the browser itself and can't
// attach custom headers— can validate the same access token without reimplementing
// signature/expiration verification. See ADR-0005.
func VerifyAccessToken(secret []byte, tokenString string) (string, error) {
	return parseAccessToken(secret, tokenString)
}

// newRefreshTokenPlain generates a cryptographically random opaque (non-JWT) refresh token.
func newRefreshTokenPlain() (string, error) {
	return common.NewOpaqueToken(refreshTokenBytes)
}

// hashRefreshToken computes the hash that gets persisted in the database; the plain
// token is never stored, it's only handed to the client once.
func hashRefreshToken(plain string) string {
	return common.HashToken(plain)
}
