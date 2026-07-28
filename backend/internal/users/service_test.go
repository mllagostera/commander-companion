package users_test

import (
	"context"
	"errors"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/usuario/commander-companion-backend/internal/common"
	"github.com/usuario/commander-companion-backend/internal/testutil"
	"github.com/usuario/commander-companion-backend/internal/users"
)

const testPassword = "correct-horse-battery-staple"

func newUsersSvc(t *testing.T) (users.Service, *pgxpool.Pool) {
	t.Helper()
	pool := testutil.DB(t)
	// "users" arrastra decks, refresh_tokens y demás por CASCADE.
	testutil.Truncate(t, pool, "users")
	return users.NewService(pool), pool
}

func registerUser(t *testing.T, svc users.Service, email string) *users.UserResponse {
	t.Helper()
	user, err := svc.RegisterUser(context.Background(), users.RegisterRequest{
		Username: "user-" + email,
		Email:    email,
		Password: testPassword,
	})
	if err != nil {
		t.Fatalf("registrando usuario de test: %v", err)
	}
	return user
}

// asFiberError traduce el error de dominio que devuelve el service a su equivalente
// HTTP con common.MapError (los services ya no dependen de fiber, ver
// internal/common/errors.go), para poder seguir verificando el status code que ve
// el cliente.
func asFiberError(t *testing.T, err error) *fiber.Error {
	t.Helper()
	var fiberErr *fiber.Error
	if !errors.As(common.MapError(err), &fiberErr) {
		t.Fatalf("error = %v (%T), want *fiber.Error", err, err)
	}
	return fiberErr
}

func TestRegisterUser_Success(t *testing.T) {
	svc, _ := newUsersSvc(t)

	user := registerUser(t, svc, "register-ok@example.com")

	if user.ID == "" {
		t.Fatalf("RegisterUser() devolvió un ID vacío: %+v", user)
	}
	if user.Email != "register-ok@example.com" {
		t.Fatalf("RegisterUser() email = %q, want %q", user.Email, "register-ok@example.com")
	}
	if user.CreatedAt.IsZero() {
		t.Fatalf("RegisterUser() created_at sin setear: %+v", user)
	}
}

// El password nunca sale en el DTO, pero sí tiene que quedar hasheado en la BD:
// guardarlo en claro pasaría desapercibido mirando solo la respuesta.
func TestRegisterUser_StoresHashedPassword(t *testing.T) {
	svc, pool := newUsersSvc(t)

	user := registerUser(t, svc, "register-hash@example.com")

	var stored string
	err := pool.QueryRow(context.Background(), "SELECT password_hash FROM users WHERE id = $1", user.ID).Scan(&stored)
	if err != nil {
		t.Fatalf("leyendo password_hash: %v", err)
	}
	if stored == testPassword {
		t.Fatal("RegisterUser() guardó el password en claro")
	}
	if stored == "" {
		t.Fatal("RegisterUser() guardó un password_hash vacío")
	}
}

func TestRegisterUser_DuplicateEmail(t *testing.T) {
	svc, _ := newUsersSvc(t)

	registerUser(t, svc, "duplicate@example.com")

	_, err := svc.RegisterUser(context.Background(), users.RegisterRequest{
		Username: "otro-username",
		Email:    "duplicate@example.com",
		Password: testPassword,
	})
	if !errors.Is(err, users.ErrUserAlreadyExists) {
		t.Fatalf("RegisterUser() con email repetido: error = %v, want ErrUserAlreadyExists", err)
	}
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusConflict {
		t.Fatalf("RegisterUser() con email repetido: code = %d, want %d", fiberErr.Code, fiber.StatusConflict)
	}
}

func TestRegisterUser_DuplicateUsername(t *testing.T) {
	svc, _ := newUsersSvc(t)

	first := registerUser(t, svc, "username-clash@example.com")

	_, err := svc.RegisterUser(context.Background(), users.RegisterRequest{
		Username: first.Username,
		Email:    "otro-email@example.com",
		Password: testPassword,
	})
	if !errors.Is(err, users.ErrUserAlreadyExists) {
		t.Fatalf("RegisterUser() con username repetido: error = %v, want ErrUserAlreadyExists", err)
	}
}

func TestGetUser_Success(t *testing.T) {
	svc, _ := newUsersSvc(t)

	created := registerUser(t, svc, "get-user@example.com")

	got, err := svc.GetUser(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("GetUser() error = %v, want nil", err)
	}
	if got.ID != created.ID || got.Email != created.Email || got.Username != created.Username {
		t.Fatalf("GetUser() = %+v, want %+v", got, created)
	}
}

func TestGetUser_NotFound(t *testing.T) {
	svc, _ := newUsersSvc(t)

	_, err := svc.GetUser(context.Background(), "3f2504e0-4f89-11d3-9a0c-0305e82c3301")
	if !errors.Is(err, users.ErrUserNotFound) {
		t.Fatalf("GetUser() con id inexistente: error = %v, want ErrUserNotFound", err)
	}
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusNotFound {
		t.Fatalf("GetUser() con id inexistente: code = %d, want %d", fiberErr.Code, fiber.StatusNotFound)
	}
}

// Un ID malformado no es distinto de uno inexistente para el cliente: ambos son 404.
func TestGetUser_MalformedID(t *testing.T) {
	svc, _ := newUsersSvc(t)

	_, err := svc.GetUser(context.Background(), "not-a-uuid")
	if !errors.Is(err, users.ErrUserNotFound) {
		t.Fatalf("GetUser() con id malformado: error = %v, want ErrUserNotFound", err)
	}
}

func TestVerifyCredentials_Success(t *testing.T) {
	svc, _ := newUsersSvc(t)

	created := registerUser(t, svc, "verify-ok@example.com")

	got, err := svc.VerifyCredentials(context.Background(), "verify-ok@example.com", testPassword)
	if err != nil {
		t.Fatalf("VerifyCredentials() error = %v, want nil", err)
	}
	if got.ID != created.ID {
		t.Fatalf("VerifyCredentials() id = %q, want %q", got.ID, created.ID)
	}
}

func TestVerifyCredentials_WrongPassword(t *testing.T) {
	svc, _ := newUsersSvc(t)

	registerUser(t, svc, "verify-wrong@example.com")

	_, err := svc.VerifyCredentials(context.Background(), "verify-wrong@example.com", "otra-password")
	if !errors.Is(err, users.ErrInvalidCredentials) {
		t.Fatalf("VerifyCredentials() con password incorrecta: error = %v, want ErrInvalidCredentials", err)
	}
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusUnauthorized {
		t.Fatalf("VerifyCredentials() con password incorrecta: code = %d, want %d", fiberErr.Code, fiber.StatusUnauthorized)
	}
}

// Un email inexistente devuelve el MISMO error que una password incorrecta: no se
// filtra qué emails están registrados.
func TestVerifyCredentials_UnknownEmail(t *testing.T) {
	svc, _ := newUsersSvc(t)

	_, err := svc.VerifyCredentials(context.Background(), "nadie@example.com", testPassword)
	if !errors.Is(err, users.ErrInvalidCredentials) {
		t.Fatalf("VerifyCredentials() con email inexistente: error = %v, want ErrInvalidCredentials", err)
	}
}

func TestFindOrCreateGoogleUser_CreatesNewUser(t *testing.T) {
	svc, _ := newUsersSvc(t)

	user, err := svc.FindOrCreateGoogleUser(context.Background(), "google-sub-1", "nuevo@example.com", true)
	if err != nil {
		t.Fatalf("FindOrCreateGoogleUser() error = %v, want nil", err)
	}
	if user.Email != "nuevo@example.com" {
		t.Fatalf("FindOrCreateGoogleUser() email = %q, want %q", user.Email, "nuevo@example.com")
	}
	// El username se deriva de la parte local del email.
	if user.Username != "nuevo" {
		t.Fatalf("FindOrCreateGoogleUser() username = %q, want %q", user.Username, "nuevo")
	}
}

// La cuenta creada por Google no tiene password: intentar loguearse con
// email/password tiene que decirlo explícitamente, no dar "credenciales inválidas".
func TestFindOrCreateGoogleUser_CreatedAccountHasNoPassword(t *testing.T) {
	svc, _ := newUsersSvc(t)

	_, err := svc.FindOrCreateGoogleUser(context.Background(), "google-sub-2", "sinpass@example.com", true)
	if err != nil {
		t.Fatalf("FindOrCreateGoogleUser() error = %v, want nil", err)
	}

	_, err = svc.VerifyCredentials(context.Background(), "sinpass@example.com", testPassword)
	if !errors.Is(err, users.ErrGoogleOnlyAccount) {
		t.Fatalf("VerifyCredentials() sobre cuenta de Google: error = %v, want ErrGoogleOnlyAccount", err)
	}
}

func TestFindOrCreateGoogleUser_IsIdempotent(t *testing.T) {
	svc, _ := newUsersSvc(t)

	first, err := svc.FindOrCreateGoogleUser(context.Background(), "google-sub-3", "repetido@example.com", true)
	if err != nil {
		t.Fatalf("FindOrCreateGoogleUser() primera llamada: error = %v", err)
	}

	second, err := svc.FindOrCreateGoogleUser(context.Background(), "google-sub-3", "repetido@example.com", true)
	if err != nil {
		t.Fatalf("FindOrCreateGoogleUser() segunda llamada: error = %v", err)
	}
	if first.ID != second.ID {
		t.Fatalf("FindOrCreateGoogleUser() creó un usuario nuevo (%q) en vez de reusar %q", second.ID, first.ID)
	}
}

// Si el email ya existe como cuenta email/password, Google se vincula a esa cuenta
// en vez de crear un duplicado, y la password original sigue funcionando.
func TestFindOrCreateGoogleUser_LinksExistingEmailAccount(t *testing.T) {
	svc, _ := newUsersSvc(t)

	existing := registerUser(t, svc, "vinculado@example.com")

	linked, err := svc.FindOrCreateGoogleUser(context.Background(), "google-sub-4", "vinculado@example.com", true)
	if err != nil {
		t.Fatalf("FindOrCreateGoogleUser() error = %v, want nil", err)
	}
	if linked.ID != existing.ID {
		t.Fatalf("FindOrCreateGoogleUser() id = %q, want %q (debería vincular, no duplicar)", linked.ID, existing.ID)
	}

	if _, err := svc.VerifyCredentials(context.Background(), "vinculado@example.com", testPassword); err != nil {
		t.Fatalf("VerifyCredentials() tras vincular Google: error = %v, want nil", err)
	}
}

// Sin email verificado por Google no se crea ni se vincula nada: si no, cualquiera
// con un id_token de un proveedor laxo podría reclamar el email de otro.
func TestFindOrCreateGoogleUser_RejectsUnverifiedEmail(t *testing.T) {
	svc, _ := newUsersSvc(t)

	_, err := svc.FindOrCreateGoogleUser(context.Background(), "google-sub-5", "noverificado@example.com", false)
	if !errors.Is(err, users.ErrEmailNotVerified) {
		t.Fatalf("FindOrCreateGoogleUser() sin email verificado: error = %v, want ErrEmailNotVerified", err)
	}
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusBadRequest {
		t.Fatalf("FindOrCreateGoogleUser() sin email verificado: code = %d, want %d", fiberErr.Code, fiber.StatusBadRequest)
	}
}

// Dos cuentas de Google distintas cuya parte local del email coincide no pueden
// chocar: el username del segundo se desambigua con un sufijo del google id.
func TestFindOrCreateGoogleUser_ResolvesUsernameCollision(t *testing.T) {
	svc, _ := newUsersSvc(t)

	first, err := svc.FindOrCreateGoogleUser(context.Background(), "google-sub-6", "colision@example.com", true)
	if err != nil {
		t.Fatalf("FindOrCreateGoogleUser() primera cuenta: error = %v", err)
	}

	second, err := svc.FindOrCreateGoogleUser(context.Background(), "google-sub-7", "colision@otrodominio.com", true)
	if err != nil {
		t.Fatalf("FindOrCreateGoogleUser() segunda cuenta: error = %v", err)
	}

	if second.ID == first.ID {
		t.Fatal("FindOrCreateGoogleUser() reusó la cuenta de otro email")
	}
	if second.Username == first.Username {
		t.Fatalf("FindOrCreateGoogleUser() username = %q, colisiona con el de la primera cuenta", second.Username)
	}
}

func TestUpdateMoxfieldUsername_Success(t *testing.T) {
	svc, _ := newUsersSvc(t)
	user := registerUser(t, svc, "moxfield-profile@example.com")

	updated, err := svc.UpdateMoxfieldUsername(context.Background(), user.ID, "MyMoxfieldHandle")
	if err != nil {
		t.Fatalf("UpdateMoxfieldUsername() error = %v, want nil", err)
	}
	if updated.MoxfieldUsername == nil || *updated.MoxfieldUsername != "MyMoxfieldHandle" {
		t.Fatalf("UpdateMoxfieldUsername() MoxfieldUsername = %v, want %q", updated.MoxfieldUsername, "MyMoxfieldHandle")
	}

	// Se refleja en una lectura posterior, no solo en la respuesta de la propia
	// escritura.
	fetched, err := svc.GetUser(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("GetUser() error = %v", err)
	}
	if fetched.MoxfieldUsername == nil || *fetched.MoxfieldUsername != "MyMoxfieldHandle" {
		t.Fatalf("GetUser() MoxfieldUsername = %v, want %q", fetched.MoxfieldUsername, "MyMoxfieldHandle")
	}
}

func TestUpdateMoxfieldUsername_Idempotent(t *testing.T) {
	svc, _ := newUsersSvc(t)
	user := registerUser(t, svc, "moxfield-idempotent@example.com")

	for range 2 {
		_, err := svc.UpdateMoxfieldUsername(context.Background(), user.ID, "SameHandle")
		if err != nil {
			t.Fatalf("UpdateMoxfieldUsername() error = %v, want nil", err)
		}
	}
}

// El handler HTTP responde 404 si :id no es el del usuario autenticado, sin llamar
// al service con un id ajeno. El mismo ErrUserNotFound que backea ese 404 también
// es lo que devuelve el service ante cualquier id inexistente, así que este test
// ejercita ese camino común.
func TestUpdateMoxfieldUsername_UnknownUser_ReturnsNotFound(t *testing.T) {
	svc, _ := newUsersSvc(t)

	_, err := svc.UpdateMoxfieldUsername(context.Background(), "00000000-0000-0000-0000-000000000000", "Handle")
	if !errors.Is(err, users.ErrUserNotFound) {
		t.Fatalf("UpdateMoxfieldUsername() con usuario inexistente: error = %v, want ErrUserNotFound", err)
	}
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusNotFound {
		t.Fatalf("UpdateMoxfieldUsername() con usuario inexistente: code = %d, want %d", fiberErr.Code, fiber.StatusNotFound)
	}
}
