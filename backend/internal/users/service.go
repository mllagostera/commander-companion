package users

import (
	"context"
	"errors"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
)

var errGetUserNotImplemented = errors.New("get user not fully implemented yet")

// Service interface para la lógica de usuarios.
type Service interface {
	RegisterUser(ctx context.Context, req RegisterRequest) (*UserResponse, error)
	GetUser(ctx context.Context, id string) (*UserResponse, error)
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

// RegisterUser crea un nuevo usuario.
func (s *service) RegisterUser(ctx context.Context, req RegisterRequest) (*UserResponse, error) {
	//nolint:godox // Deferido a la fase de refinamiento (auth real, ver TASKS.md).
	// TODO: Hashear la contraseña usando bcrypt
	dummyHash := "hashed_" + req.Password

	user, err := s.repo.CreateUser(ctx, CreateUserParams{
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: dummyHash,
	})
	if err != nil {
		// En un entorno real se comprobaría el constraint violation
		return nil, fiber.NewError(fiber.StatusConflict, "User already exists")
	}

	// Mapeo manual a DTO
	var createdAt time.Time
	if user.CreatedAt.Valid {
		createdAt = user.CreatedAt.Time
	}

	return &UserResponse{
		ID:        user.ID.String(),
		Username:  user.Username,
		Email:     user.Email,
		CreatedAt: createdAt,
	}, nil
}

// GetUser devuelve un usuario por su ID.
func (s *service) GetUser(ctx context.Context, id string) (*UserResponse, error) {
	// sqlc genera GetUserByID con pgtype.UUID; falta mapear el id string recibido
	// a pgtype.UUID. Se implementará junto al middleware de auth real.
	return nil, errGetUserNotImplemented
}
