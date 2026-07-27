package users

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/usuario/commander-companion-backend/internal/common"
)

const (
	maxUsernameAttempts = 3
	googleIDSuffixLen   = 6
	usernameConstraint  = "users_username_key"
)

var (
	// ErrInvalidCredentials indica que el email no existe o el password no coincide.
	ErrInvalidCredentials = common.Unauthorized("invalid email or password")
	// ErrGoogleOnlyAccount indica que la cuenta no tiene password (se registró con Google).
	ErrGoogleOnlyAccount = common.Unauthorized("account has no password set, sign in with google instead")
	// ErrUserNotFound indica que no existe un usuario con el ID indicado.
	ErrUserNotFound = common.NotFound("user not found")
	// ErrEmailNotVerified indica que Google no confirma el email asociado a la cuenta.
	ErrEmailNotVerified = common.InvalidInput("google email not verified")
	// ErrUserAlreadyExists indica que el username o el email ya están tomados.
	ErrUserAlreadyExists = common.Conflict("User already exists")
	// ErrUsernameExhausted indica que no se pudo generar un username único para una cuenta de Google.
	// No es un error de dominio traducible: sale como 500, porque implica un problema del servidor.
	ErrUsernameExhausted = errors.New("could not allocate a unique username for google user")
)

// Service interface para la lógica de usuarios.
type Service interface {
	RegisterUser(ctx context.Context, req RegisterRequest) (*UserResponse, error)
	GetUser(ctx context.Context, id string) (*UserResponse, error)
	VerifyCredentials(ctx context.Context, email, password string) (*UserResponse, error)
	FindOrCreateGoogleUser(ctx context.Context, googleID, email string, emailVerified bool) (*UserResponse, error)
}

type service struct {
	repo *Queries
}

// NewService crea un nuevo servicio de usuarios.
func NewService(db *pgxpool.Pool) Service {
	return &service{
		repo: New(db),
	}
}

// RegisterUser crea un nuevo usuario con email y contraseña.
func (s *service) RegisterUser(ctx context.Context, req RegisterRequest) (*UserResponse, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hashing password: %w", err)
	}

	user, err := s.repo.CreateUser(ctx, CreateUserParams{
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: pgtype.Text{String: string(hash), Valid: true},
	})
	if err != nil {
		//nolint:godox // Deferido a la fase de refinamiento: distinguir username vs. email duplicado.
		// TODO: inspeccionar pgErr.ConstraintName para devolver un mensaje más preciso.
		return nil, ErrUserAlreadyExists
	}

	return toUserResponse(&user), nil
}

// GetUser devuelve un usuario por su ID.
func (s *service) GetUser(ctx context.Context, id string) (*UserResponse, error) {
	uid, err := common.ParseUUID(id)
	if err != nil {
		return nil, ErrUserNotFound
	}

	user, err := s.repo.GetUserByID(ctx, uid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("looking up user by id: %w", err)
	}

	return toUserResponse(&user), nil
}

// VerifyCredentials valida el email/password y devuelve el usuario si son correctos.
func (s *service) VerifyCredentials(ctx context.Context, email, password string) (*UserResponse, error) {
	user, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInvalidCredentials
		}
		return nil, fmt.Errorf("looking up user by email: %w", err)
	}

	if !user.PasswordHash.Valid {
		return nil, ErrGoogleOnlyAccount
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash.String), []byte(password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	return toUserResponse(&user), nil
}

// FindOrCreateGoogleUser busca un usuario por su Google ID. Si no existe pero el
// email ya está registrado, vincula la cuenta de Google; si tampoco existe,
// crea un usuario nuevo sin password.
func (s *service) FindOrCreateGoogleUser(
	ctx context.Context, googleID, email string, emailVerified bool,
) (*UserResponse, error) {
	googleIDText := pgtype.Text{String: googleID, Valid: true}

	user, err := s.repo.GetUserByGoogleID(ctx, googleIDText)
	switch {
	case err == nil:
		return toUserResponse(&user), nil
	case !errors.Is(err, pgx.ErrNoRows):
		return nil, fmt.Errorf("looking up user by google id: %w", err)
	}

	if !emailVerified {
		return nil, ErrEmailNotVerified
	}

	existing, err := s.repo.GetUserByEmail(ctx, email)
	switch {
	case err == nil:
		var linked User
		linked, err = s.repo.LinkGoogleID(ctx, LinkGoogleIDParams{ID: existing.ID, GoogleID: googleIDText})
		if err != nil {
			return nil, fmt.Errorf("linking google account: %w", err)
		}
		return toUserResponse(&linked), nil
	case !errors.Is(err, pgx.ErrNoRows):
		return nil, fmt.Errorf("looking up user by email: %w", err)
	}

	return s.createGoogleUser(ctx, googleID, googleIDText, email)
}

func (s *service) createGoogleUser(
	ctx context.Context, googleID string, googleIDText pgtype.Text, email string,
) (*UserResponse, error) {
	base := usernameFromEmail(email)

	for attempt := range maxUsernameAttempts {
		candidate := base
		if attempt > 0 {
			candidate = fmt.Sprintf("%s-%s", base, googleID[:min(googleIDSuffixLen, len(googleID))])
		}

		created, err := s.repo.CreateUserWithGoogle(ctx, CreateUserWithGoogleParams{
			Username: candidate,
			Email:    email,
			GoogleID: googleIDText,
		})
		if err == nil {
			return toUserResponse(&created), nil
		}

		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.ConstraintName != usernameConstraint {
			return nil, fmt.Errorf("creating google user: %w", err)
		}
	}

	return nil, ErrUsernameExhausted
}

func usernameFromEmail(email string) string {
	local, _, found := strings.Cut(email, "@")
	if !found || local == "" {
		return "user"
	}
	return local
}

func toUserResponse(user *User) *UserResponse {
	var createdAt time.Time
	if user.CreatedAt.Valid {
		createdAt = user.CreatedAt.Time
	}

	return &UserResponse{
		ID:        user.ID.String(),
		Username:  user.Username,
		Email:     user.Email,
		CreatedAt: createdAt,
	}
}
