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
	resp, _, err := s.issueTokenPair(ctx, user)
	return resp, err
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

	resp, _, err := s.issueTokenPair(ctx, user)
	return resp, err
}

// refreshReuseGrace is how long after a rotation the rotated-away token is still
// accepted once more without being read as theft, see Refresh and ADR-0027.
const refreshReuseGrace = 30 * time.Second

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
//
// One exception (ADR-0027): a token rotated less than refreshReuseGrace ago, whose
// replacement is still active, gets a fresh pair instead. That is what one client
// refreshing twice in parallel looks like (two web BFF instances, two app requests
// racing on the same expired access token), and treating it as theft logged real users
// out of every device. Tokens revoked by logout, reuse detection or a password reset
// have no replacement and never qualify.
func (s *service) Refresh(ctx context.Context, refreshToken string) (*TokenResponse, error) {
	record, err := s.repo.GetRefreshTokenByHash(ctx, hashRefreshToken(refreshToken))
	if err != nil {
		return nil, ErrInvalidToken
	}

	if record.RevokedAt.Valid {
		withinGrace, graceErr := s.repo.HasActiveSuccessorWithinGrace(ctx, HasActiveSuccessorWithinGraceParams{
			ID:           record.ID,
			GraceSeconds: refreshReuseGrace.Seconds(),
		})
		if graceErr != nil {
			return nil, fmt.Errorf("checking refresh token reuse grace: %w", graceErr)
		}
		if withinGrace {
			resp, _, issueErr := s.issueTokensForRefresh(ctx, &record)
			return resp, issueErr
		}

		if revokeErr := s.repo.RevokeAllRefreshTokensForUser(ctx, record.UserID); revokeErr != nil {
			return nil, fmt.Errorf("revoking refresh token family after reuse: %w", revokeErr)
		}
		return nil, ErrInvalidToken
	}

	resp, successorID, err := s.issueTokensForRefresh(ctx, &record)
	if err != nil {
		return nil, err
	}

	// Zero rows means a concurrent Refresh with this same token rotated it first; the
	// pair just issued is still legitimate (the token was active when read), it simply
	// isn't recorded as the successor.
	if _, err = s.repo.RotateRefreshToken(ctx, RotateRefreshTokenParams{
		ID:         record.ID,
		ReplacedBy: successorID,
	}); err != nil {
		return nil, fmt.Errorf("rotating used refresh token: %w", err)
	}

	return resp, nil
}

// issueTokensForRefresh runs the checks every refresh needs — the token hasn't
// expired, the account is still active — and issues a new pair for its user.
func (s *service) issueTokensForRefresh(
	ctx context.Context, record *RefreshToken,
) (*TokenResponse, pgtype.UUID, error) {
	if !record.ExpiresAt.Valid || record.ExpiresAt.Time.Before(time.Now()) {
		return nil, pgtype.UUID{}, ErrInvalidToken
	}

	user, err := s.users.GetUser(ctx, record.UserID.String())
	if err != nil {
		return nil, pgtype.UUID{}, err
	}

	// An admin may have deactivated this account after the refresh token was issued;
	// checking here (rather than trusting the still-valid token) bounds how long a
	// deactivated account stays usable to at most one access-token TTL — see ADR-0018.
	if !user.IsActive {
		return nil, pgtype.UUID{}, users.ErrAccountDeactivated
	}

	return s.issueTokenPair(ctx, user)
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

// issueTokenPair issues a new access token and refresh token for user, and returns
// the stored refresh token's ID alongside them.
func (s *service) issueTokenPair(ctx context.Context, user *users.UserResponse) (*TokenResponse, pgtype.UUID, error) {
	accessToken, expiresAt, err := generateAccessToken(s.cfg.JWTSecret, user.ID, s.cfg.AccessTokenTTL)
	if err != nil {
		return nil, pgtype.UUID{}, err
	}

	refreshPlain, err := newRefreshTokenPlain()
	if err != nil {
		return nil, pgtype.UUID{}, err
	}

	userUUID, err := common.ParseUUID(user.ID)
	if err != nil {
		return nil, pgtype.UUID{}, fmt.Errorf("parsing user id: %w", err)
	}

	stored, err := s.repo.CreateRefreshToken(ctx, CreateRefreshTokenParams{
		UserID:    userUUID,
		TokenHash: hashRefreshToken(refreshPlain),
		ExpiresAt: pgtype.Timestamp{Time: time.Now().Add(s.cfg.RefreshTokenTTL), Valid: true},
	})
	if err != nil {
		return nil, pgtype.UUID{}, fmt.Errorf("storing refresh token: %w", err)
	}

	return &TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshPlain,
		TokenType:    "Bearer",
		ExpiresIn:    int64(time.Until(expiresAt).Seconds()),
		User:         user,
	}, stored.ID, nil
}
