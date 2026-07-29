package users

import (
	"context"
	"errors"
	"fmt"
	"log"
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

	// emailVerificationTokenBytes es la longitud en bytes del token opaco de
	// verificación de email (antes de codificar), igual que el refresh token
	// (ver internal/auth/token.go).
	emailVerificationTokenBytes = 32
	// emailVerificationTokenTTL es cuánto tarda en vencer un link de verificación
	// mandado por mail antes de que haya que pedir un reenvío.
	emailVerificationTokenTTL = 24 * time.Hour

	// minSearchQueryLength evita búsquedas de 1 carácter que devolverían medio
	// directorio de usuarios (y facilitarían enumeración por fuerza bruta de a un
	// carácter genérico por vez).
	minSearchQueryLength = 2
	// searchResultLimit acota la respuesta de SearchUsers — no hay paginación acá,
	// es un autocomplete, no un listado.
	searchResultLimit = 10
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
	// ErrEmailNotConfirmed indica que la cuenta es válida (password correcto) pero
	// todavía no confirmó su propio email — distinto de ErrEmailNotVerified (que es
	// sobre lo que afirma Google), y distinto de ErrInvalidCredentials (acá ya se sabe
	// quién es). El cliente puede ofrecer reenviar el mail de verificación.
	ErrEmailNotConfirmed = common.Forbidden("email not confirmed, check your inbox")
	// ErrInvalidVerificationToken indica que el token de verificación de email no
	// existe, ya se usó o venció.
	ErrInvalidVerificationToken = common.InvalidInput("invalid or expired verification token")
	// ErrUserAlreadyExists indica que el username o el email ya están tomados.
	ErrUserAlreadyExists = common.Conflict("User already exists")
	// ErrUsernameExhausted indica que no se pudo generar un username único para una cuenta de Google.
	// No es un error de dominio traducible: sale como 500, porque implica un problema del servidor.
	ErrUsernameExhausted = errors.New("could not allocate a unique username for google user")
	// ErrInvalidCurrentPassword indica que el password actual mandado en ChangePassword no coincide.
	ErrInvalidCurrentPassword = common.Unauthorized("current password is incorrect")
	// ErrPasswordTooShort indica que el password nuevo no cumple el largo mínimo.
	ErrPasswordTooShort = common.InvalidInput("password must be at least 8 characters long")
	// ErrSearchQueryTooShort indica que el query de búsqueda es demasiado corto.
	ErrSearchQueryTooShort = common.InvalidInput("search query must be at least 2 characters long")
)

// minPasswordLength es el largo mínimo del password nuevo en ChangePassword, igual al
// mínimo que ya exige el form de registro del lado del cliente (ver web/app/pages/register.vue).
const minPasswordLength = 8

// Mailer es lo que users necesita para mandar el mail de verificación de cuenta
// (permite mockearlo en tests; ver decks.MoxfieldClient para el mismo patrón).
type Mailer interface {
	SendVerificationEmail(ctx context.Context, to, username, verifyURL string) error
}

// Service interface para la lógica de usuarios.
type Service interface {
	RegisterUser(ctx context.Context, req RegisterRequest) (*UserResponse, error)
	GetUser(ctx context.Context, id string) (*UserResponse, error)
	VerifyCredentials(ctx context.Context, email, password string) (*UserResponse, error)
	FindOrCreateGoogleUser(ctx context.Context, googleID, email string, emailVerified bool) (*UserResponse, error)
	UpdateMoxfieldUsername(ctx context.Context, id, moxfieldUsername string) (*UserResponse, error)
	// ChangePassword valida el password actual y, si coincide, lo reemplaza por el nuevo.
	ChangePassword(ctx context.Context, id, currentPassword, newPassword string) error
	// VerifyEmail confirma la cuenta asociada al token de verificación mandado por mail.
	VerifyEmail(ctx context.Context, token string) error
	// ResendVerification manda un nuevo mail de verificación si corresponde. Nunca
	// revela si el email existe, ya está verificado, o es una cuenta de Google: siempre
	// "tiene éxito" desde la perspectiva del caller (mismo criterio anti-enumeración
	// que VerifyCredentials).
	ResendVerification(ctx context.Context, email string) error
	// SearchUsers busca por username (contiene, case-insensitive) o email (exacto, ver
	// query.sql sobre por qué no es parcial). Excluye al propio requesterID y nunca
	// expone el email en el resultado (ver UserSearchResult).
	SearchUsers(ctx context.Context, requesterID, query string) ([]UserSearchResult, error)
}

type service struct {
	repo                     *Queries
	mailer                   Mailer
	webAppURL                string
	requireEmailVerification bool
}

// NewService crea un nuevo servicio de usuarios. requireEmailVerification controla si
// el alta por email/password exige confirmar el email antes de poder loguearse (ver
// ADR-0012); en false (fase alpha) las cuentas nuevas quedan verificadas de entrada y
// RegisterUser ni genera el token ni manda mail — no tiene sentido gastar ese envío si
// nadie va a exigir el click.
func NewService(db *pgxpool.Pool, mailer Mailer, webAppURL string, requireEmailVerification bool) Service {
	return &service{
		repo:                     New(db),
		mailer:                   mailer,
		webAppURL:                webAppURL,
		requireEmailVerification: requireEmailVerification,
	}
}

// RegisterUser crea un nuevo usuario con email y contraseña. Si requireEmailVerification
// está activo, queda sin confirmar y dispara el mail (ver sendVerificationEmail); si no,
// queda verificada de entrada y no se manda nada.
func (s *service) RegisterUser(ctx context.Context, req RegisterRequest) (*UserResponse, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hashing password: %w", err)
	}

	user, err := s.repo.CreateUser(ctx, CreateUserParams{
		Username:      req.Username,
		Email:         req.Email,
		PasswordHash:  pgtype.Text{String: string(hash), Valid: true},
		EmailVerified: !s.requireEmailVerification,
	})
	if err != nil {
		//nolint:godox // Deferido a la fase de refinamiento: distinguir username vs. email duplicado.
		// TODO: inspeccionar pgErr.ConstraintName para devolver un mensaje más preciso.
		return nil, ErrUserAlreadyExists
	}

	if !s.requireEmailVerification {
		return toUserResponse(&user), nil
	}

	// La cuenta ya quedó creada: un mail que no sale no revierte el registro, el
	// usuario puede pedir el reenvío desde /login.
	if err := s.sendVerificationEmail(ctx, &user); err != nil {
		log.Printf("no se pudo mandar el mail de verificación a %s: %v", user.Email, err)
	}

	return toUserResponse(&user), nil
}

// sendVerificationEmail genera un token de verificación nuevo, lo persiste hasheado y
// dispara el mail con el link armado sobre webAppURL.
func (s *service) sendVerificationEmail(ctx context.Context, user *User) error {
	plain, err := common.NewOpaqueToken(emailVerificationTokenBytes)
	if err != nil {
		return err
	}

	if _, err := s.repo.CreateEmailVerificationToken(ctx, CreateEmailVerificationTokenParams{
		UserID:    user.ID,
		TokenHash: common.HashToken(plain),
		ExpiresAt: pgtype.Timestamp{Time: time.Now().Add(emailVerificationTokenTTL), Valid: true},
	}); err != nil {
		return fmt.Errorf("storing verification token: %w", err)
	}

	verifyURL := s.webAppURL + "/verify-email?token=" + plain
	if err := s.mailer.SendVerificationEmail(ctx, user.Email, user.Username, verifyURL); err != nil {
		return fmt.Errorf("sending verification email: %w", err)
	}
	return nil
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

// UpdateMoxfieldUsername vincula (o cambia) el username de Moxfield del perfil. No
// valida contra la API de Moxfield que el username exista de verdad — eso queda para
// cuando se use (ver internal/moxfieldimport).
func (s *service) UpdateMoxfieldUsername(ctx context.Context, id, moxfieldUsername string) (*UserResponse, error) {
	uid, err := common.ParseUUID(id)
	if err != nil {
		return nil, ErrUserNotFound
	}

	user, err := s.repo.UpdateMoxfieldUsername(ctx, UpdateMoxfieldUsernameParams{
		ID:               uid,
		MoxfieldUsername: pgtype.Text{String: moxfieldUsername, Valid: moxfieldUsername != ""},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("updating moxfield username: %w", err)
	}

	return toUserResponse(&user), nil
}

// ChangePassword valida currentPassword contra el hash guardado y, si coincide, lo
// reemplaza por el hash de newPassword. Las cuentas de Google (sin password_hash)
// no pueden usar este camino: deben seguir entrando por Google Sign-In.
func (s *service) ChangePassword(ctx context.Context, id, currentPassword, newPassword string) error {
	if len(newPassword) < minPasswordLength {
		return ErrPasswordTooShort
	}

	uid, err := common.ParseUUID(id)
	if err != nil {
		return ErrUserNotFound
	}

	user, err := s.repo.GetUserByID(ctx, uid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrUserNotFound
		}
		return fmt.Errorf("looking up user by id: %w", err)
	}

	if !user.PasswordHash.Valid {
		return ErrGoogleOnlyAccount
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash.String), []byte(currentPassword)); err != nil {
		return ErrInvalidCurrentPassword
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hashing new password: %w", err)
	}

	if _, err := s.repo.UpdatePasswordHash(ctx, UpdatePasswordHashParams{
		ID:           uid,
		PasswordHash: pgtype.Text{String: string(hash), Valid: true},
	}); err != nil {
		return fmt.Errorf("updating password hash: %w", err)
	}

	return nil
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

	if !user.EmailVerified {
		return nil, ErrEmailNotConfirmed
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

// VerifyEmail confirma la cuenta asociada al token de verificación mandado por mail.
// Un token inexistente, ya usado o vencido devuelve el mismo error, para no filtrar
// cuál de los tres casos es.
func (s *service) VerifyEmail(ctx context.Context, token string) error {
	record, err := s.repo.GetEmailVerificationTokenByHash(ctx, common.HashToken(token))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrInvalidVerificationToken
		}
		return fmt.Errorf("looking up verification token: %w", err)
	}
	if record.UsedAt.Valid || !record.ExpiresAt.Valid || record.ExpiresAt.Time.Before(time.Now()) {
		return ErrInvalidVerificationToken
	}

	if err := s.repo.MarkEmailVerificationTokenUsed(ctx, record.ID); err != nil {
		return fmt.Errorf("marking verification token used: %w", err)
	}

	if _, err := s.repo.SetUserEmailVerified(ctx, record.UserID); err != nil {
		return fmt.Errorf("marking user verified: %w", err)
	}

	return nil
}

// ResendVerification manda un token de verificación nuevo si corresponde. Nunca revela
// nada al caller (siempre "éxito"): si el email no existe, ya está verificado, o es una
// cuenta sin password (Google-only, nunca hace login por password y por lo tanto nunca
// se bloquea por esto), no hace nada.
func (s *service) ResendVerification(ctx context.Context, email string) error {
	user, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return fmt.Errorf("looking up user by email: %w", err)
	}

	if user.EmailVerified || !user.PasswordHash.Valid {
		return nil
	}

	if err := s.sendVerificationEmail(ctx, &user); err != nil {
		log.Printf("no se pudo reenviar el mail de verificación a %s: %v", user.Email, err)
	}
	return nil
}

// SearchUsers busca por username (ILIKE, contiene) y/o email (exacto). Los dos caminos
// se combinan y deduplican; el propio requesterID nunca aparece en el resultado (no
// tiene sentido invitarte a vos mismo a un playgroup) ni tampoco su email (ver
// UserSearchResult).
func (s *service) SearchUsers(ctx context.Context, requesterID, query string) ([]UserSearchResult, error) {
	trimmed := strings.TrimSpace(query)
	if len([]rune(trimmed)) < minSearchQueryLength {
		return nil, ErrSearchQueryTooShort
	}

	seen := map[string]bool{requesterID: true}
	results := []UserSearchResult{} // nunca nil: sin esto, un resultado vacío serializa a JSON `null` en vez de `[]`

	byEmail, err := s.repo.GetUserByEmail(ctx, trimmed)
	switch {
	case err == nil:
		id := byEmail.ID.String()
		if !seen[id] {
			seen[id] = true
			results = append(results, UserSearchResult{ID: id, Username: byEmail.Username})
		}
	case !errors.Is(err, pgx.ErrNoRows):
		return nil, fmt.Errorf("searching user by email: %w", err)
	}

	byUsername, err := s.repo.SearchUsersByUsername(
		ctx,
		SearchUsersByUsernameParams{
			Pattern:     pgtype.Text{String: trimmed, Valid: true},
			ResultLimit: searchResultLimit,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("searching users by username: %w", err)
	}
	for i := range byUsername {
		id := byUsername[i].ID.String()
		if seen[id] {
			continue
		}
		seen[id] = true
		results = append(results, UserSearchResult{ID: id, Username: byUsername[i].Username})
	}

	return results, nil
}

func toUserResponse(user *User) *UserResponse {
	var createdAt time.Time
	if user.CreatedAt.Valid {
		createdAt = user.CreatedAt.Time
	}

	res := &UserResponse{
		ID:        user.ID.String(),
		Username:  user.Username,
		Email:     user.Email,
		CreatedAt: createdAt,
	}
	if user.MoxfieldUsername.Valid {
		res.MoxfieldUsername = &user.MoxfieldUsername.String
	}
	return res
}
