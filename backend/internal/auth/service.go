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

// Config agrupa los parámetros de configuración del módulo auth.
type Config struct {
	JWTSecret       []byte
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
	GoogleClientID  string
}

// Service define la lógica de negocio de autenticación.
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

// NewService crea un nuevo servicio de auth.
func NewService(db *pgxpool.Pool, usersSvc users.Service, cfg Config) Service {
	return &service{
		repo:   New(db),
		users:  usersSvc,
		cfg:    cfg,
		google: newGoogleVerifier(cfg.GoogleClientID),
	}
}

// Login autentica a un usuario por email/password y emite un nuevo par de tokens.
func (s *service) Login(ctx context.Context, email, password string) (*TokenResponse, error) {
	user, err := s.users.VerifyCredentials(ctx, email, password)
	if err != nil {
		return nil, err
	}
	return s.issueTokens(ctx, user)
}

// GoogleLogin verifica un id_token de Google, resuelve (o crea) el usuario
// correspondiente y emite un nuevo par de tokens.
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

// Refresh valida un refresh token vigente, lo revoca (rotación) y emite un nuevo par de tokens.
func (s *service) Refresh(ctx context.Context, refreshToken string) (*TokenResponse, error) {
	record, err := s.repo.GetRefreshTokenByHash(ctx, hashRefreshToken(refreshToken))
	if err != nil {
		return nil, ErrInvalidToken
	}
	if record.RevokedAt.Valid || !record.ExpiresAt.Valid || record.ExpiresAt.Time.Before(time.Now()) {
		return nil, ErrInvalidToken
	}

	if err = s.repo.RevokeRefreshToken(ctx, record.ID); err != nil {
		return nil, fmt.Errorf("revoking used refresh token: %w", err)
	}

	user, err := s.users.GetUser(ctx, record.UserID.String())
	if err != nil {
		return nil, err
	}

	return s.issueTokens(ctx, user)
}

// Logout revoca el refresh token indicado, terminando la sesión.
func (s *service) Logout(ctx context.Context, refreshToken string) error {
	if err := s.repo.RevokeRefreshTokenByHash(ctx, hashRefreshToken(refreshToken)); err != nil {
		return fmt.Errorf("revoking refresh token: %w", err)
	}
	return nil
}

// Me devuelve el perfil del usuario autenticado.
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
