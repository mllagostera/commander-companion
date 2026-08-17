package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/usuario/commander-companion-backend/internal/common"
	"github.com/usuario/commander-companion-backend/internal/users"
)

// Config groups the configuration parameters of the auth module.
type Config struct {
	JWTSecret       []byte
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
	GoogleClientID  string
}

// Service defines the authentication business logic.
type Service interface {
	Login(ctx context.Context, email, password string) (*TokenResponse, error)
	GoogleLogin(ctx context.Context, idToken string) (*TokenResponse, error)
	Refresh(ctx context.Context, refreshToken string) (*TokenResponse, error)
	Logout(ctx context.Context, refreshToken string) error
	Me(ctx context.Context, userID string) (*users.UserResponse, error)
}

type service struct {
	repo   *Queries
	users  users.Service
	cfg    Config
	google *googleVerifier
}

// NewService creates a new auth service.
func NewService(db *pgxpool.Pool, usersSvc users.Service, cfg Config) Service {
	return &service{
		repo:   New(db),
		users:  usersSvc,
		cfg:    cfg,
		google: newGoogleVerifier(cfg.GoogleClientID),
	}
}

// Login authenticates a user by email/password and issues a new token pair.
func (s *service) Login(ctx context.Context, email, password string) (*TokenResponse, error) {
	user, err := s.users.VerifyCredentials(ctx, email, password)
	if err != nil {
		return nil, err
	}
	return s.issueTokens(ctx, user)
}

// GoogleLogin verifies a Google id_token, resolves (or creates) the corresponding
// user, and issues a new token pair.
func (s *service) GoogleLogin(ctx context.Context, idToken string) (*TokenResponse, error) {
	claims, err := s.google.verify(ctx, idToken)
	if err != nil {
		return nil, err
	}

	user, err := s.users.FindOrCreateGoogleUser(ctx, claims.Subject, claims.Email, claims.EmailVerified)
	if err != nil {
		return nil, err
	}

	return s.issueTokens(ctx, user)
}

// Refresh validates a valid refresh token, revokes it (rotation), and issues a new token pair.
//
// Reuse of an already-revoked token — one that was already rotated away by an earlier
// Refresh call, or explicitly revoked by Logout — is treated as a signal of theft, not
// just an expired session: rotation makes every refresh token single-use, so a second
// presentation of the same one means either the legitimate client and an attacker both
// have it (replay), or a stolen token is being used after the real client already moved
// on. Either way, the whole family of that user's active refresh tokens is revoked, so a
// leaked token can't be replayed indefinitely to keep a session alive on every device —
// the account owner has to log in again everywhere, not just lose this one token.
func (s *service) Refresh(ctx context.Context, refreshToken string) (*TokenResponse, error) {
	record, err := s.repo.GetRefreshTokenByHash(ctx, hashRefreshToken(refreshToken))
	if err != nil {
		return nil, ErrInvalidToken
	}

	if record.RevokedAt.Valid {
		if revokeErr := s.repo.RevokeAllRefreshTokensForUser(ctx, record.UserID); revokeErr != nil {
			return nil, fmt.Errorf("revoking refresh token family after reuse: %w", revokeErr)
		}
		return nil, ErrInvalidToken
	}
	if !record.ExpiresAt.Valid || record.ExpiresAt.Time.Before(time.Now()) {
		return nil, ErrInvalidToken
	}

	if err = s.repo.RevokeRefreshToken(ctx, record.ID); err != nil {
		return nil, fmt.Errorf("revoking used refresh token: %w", err)
	}

	user, err := s.users.GetUser(ctx, record.UserID.String())
	if err != nil {
		return nil, err
	}

	// An admin may have deactivated this account after the refresh token was issued;
	// checking here (rather than trusting the still-valid token) bounds how long a
	// deactivated account stays usable to at most one access-token TTL — see ADR-0018.
	if !user.IsActive {
		return nil, users.ErrAccountDeactivated
	}

	return s.issueTokens(ctx, user)
}

// Logout revokes the given refresh token, ending the session.
func (s *service) Logout(ctx context.Context, refreshToken string) error {
	if err := s.repo.RevokeRefreshTokenByHash(ctx, hashRefreshToken(refreshToken)); err != nil {
		return fmt.Errorf("revoking refresh token: %w", err)
	}
	return nil
}

// Me returns the authenticated user's profile.
func (s *service) Me(ctx context.Context, userID string) (*users.UserResponse, error) {
	return s.users.GetUser(ctx, userID)
}

func (s *service) issueTokens(ctx context.Context, user *users.UserResponse) (*TokenResponse, error) {
	accessToken, expiresAt, err := generateAccessToken(s.cfg.JWTSecret, user.ID, s.cfg.AccessTokenTTL)
	if err != nil {
		return nil, err
	}

	refreshPlain, err := newRefreshTokenPlain()
	if err != nil {
		return nil, err
	}

	userUUID, err := common.ParseUUID(user.ID)
	if err != nil {
		return nil, fmt.Errorf("parsing user id: %w", err)
	}

	_, err = s.repo.CreateRefreshToken(ctx, CreateRefreshTokenParams{
		UserID:    userUUID,
		TokenHash: hashRefreshToken(refreshPlain),
		ExpiresAt: pgtype.Timestamp{Time: time.Now().Add(s.cfg.RefreshTokenTTL), Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("storing refresh token: %w", err)
	}

	return &TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshPlain,
		TokenType:    "Bearer",
		ExpiresIn:    int64(time.Until(expiresAt).Seconds()),
		User:         user,
	}, nil
}
