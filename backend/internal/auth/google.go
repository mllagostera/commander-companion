package auth

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/coreos/go-oidc/v3/oidc"
)

const googleIssuer = "https://accounts.google.com"

// ErrGoogleAuthNotConfigured indica que el servidor no tiene GOOGLE_CLIENT_ID configurado.
var ErrGoogleAuthNotConfigured = errors.New("google sign-in is not configured on this server")

// GoogleClaims son los datos relevantes extraídos de un id_token de Google ya verificado.
type GoogleClaims struct {
	Subject       string
	Email         string
	EmailVerified bool
}

// googleVerifier valida id_tokens de Google. El discovery document se resuelve de
// forma perezosa en la primera verificación, para no acoplar el arranque del
// servidor a la disponibilidad de Google.
type googleVerifier struct {
	clientID string

	mu       sync.Mutex
	verifier *oidc.IDTokenVerifier
}

func newGoogleVerifier(clientID string) *googleVerifier {
	return &googleVerifier{clientID: clientID}
}

func (g *googleVerifier) verify(ctx context.Context, rawIDToken string) (*GoogleClaims, error) {
	if g.clientID == "" {
		return nil, ErrGoogleAuthNotConfigured
	}

	verifier, err := g.ensureVerifier(ctx)
	if err != nil {
		return nil, err
	}

	idToken, err := verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidToken, err)
	}

	var claims struct {
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("parsing google id_token claims: %w", err)
	}

	return &GoogleClaims{
		Subject:       idToken.Subject,
		Email:         claims.Email,
		EmailVerified: claims.EmailVerified,
	}, nil
}

func (g *googleVerifier) ensureVerifier(ctx context.Context) (*oidc.IDTokenVerifier, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.verifier != nil {
		return g.verifier, nil
	}

	provider, err := oidc.NewProvider(ctx, googleIssuer)
	if err != nil {
		return nil, fmt.Errorf("connecting to google oidc discovery: %w", err)
	}

	g.verifier = provider.Verifier(&oidc.Config{ClientID: g.clientID})
	return g.verifier, nil
}
