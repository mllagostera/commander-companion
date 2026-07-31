package auth

import (
	"context"
	"fmt"
	"sync"

	"github.com/coreos/go-oidc/v3/oidc"

	"github.com/usuario/commander-companion-backend/internal/common"
)

const googleIssuer = "https://accounts.google.com"

// ErrGoogleAuthNotConfigured indicates that the server doesn't have GOOGLE_CLIENT_ID configured.
var ErrGoogleAuthNotConfigured = common.NotImplemented("google sign-in is not configured on this server")

// GoogleClaims are the relevant data extracted from an already-verified Google id_token.
type GoogleClaims struct {
	Subject       string
	Email         string
	EmailVerified bool
}

// googleVerifier validates Google id_tokens. The discovery document is resolved
// lazily on the first verification, so the server's startup isn't coupled to
// Google's availability.
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
