package users_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/usuario/commander-companion-backend/internal/common"
	"github.com/usuario/commander-companion-backend/internal/testutil"
	"github.com/usuario/commander-companion-backend/internal/users"
)

const (
	testPassword  = "correct-horse-battery-staple"
	testWebAppURL = "http://localhost:3000"
)

func newUsersSvc(t *testing.T) (users.Service, *pgxpool.Pool) {
	t.Helper()
	svc, pool, _ := newUsersSvcWithMailer(t)
	return svc, pool
}

// fakeMailer registra el último link de verificación mandado a cada email, para poder
// ejercitar VerifyEmail/ResendVerification sin depender de Resend real.
type fakeMailer struct {
	verifyURLByEmail map[string]string
}

func newFakeMailer() *fakeMailer {
	return &fakeMailer{verifyURLByEmail: make(map[string]string)}
}

func (m *fakeMailer) SendVerificationEmail(_ context.Context, to, _, verifyURL string) error {
	m.verifyURLByEmail[to] = verifyURL
	return nil
}

// tokenFor extrae el token del último link mandado a ese email.
func (m *fakeMailer) tokenFor(t *testing.T, email string) string {
	t.Helper()
	verifyURL, ok := m.verifyURLByEmail[email]
	if !ok {
		t.Fatalf("no se mandó ningún mail de verificación a %s", email)
	}
	_, token, found := strings.Cut(verifyURL, "token=")
	if !found {
		t.Fatalf("verifyURL sin token: %q", verifyURL)
	}
	return token
}

func newUsersSvcWithMailer(t *testing.T) (users.Service, *pgxpool.Pool, *fakeMailer) {
	t.Helper()
	pool := testutil.DB(t)
	// "users" arrastra decks, refresh_tokens, email_verification_tokens y demás por CASCADE.
	testutil.Truncate(t, pool, "users")
	mailer := newFakeMailer()
	return users.NewService(pool, mailer, testWebAppURL, true), pool, mailer
}

// newUsersSvcVerificationOff instancia el servicio con requireEmailVerification=false
// (el default de producción en fase alpha, ver ADR-0012): el registro debe dejar la
// cuenta ya verificada y el mailer no debe recibir ningún envío.
func newUsersSvcVerificationOff(t *testing.T) (users.Service, *fakeMailer) {
	t.Helper()
	pool := testutil.DB(t)
	testutil.Truncate(t, pool, "users")
	mailer := newFakeMailer()
	return users.NewService(pool, mailer, testWebAppURL, false), mailer
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

// VerifyCredentials solo tiene éxito con el email ya confirmado (ver
// TestRegisterUser_LeavesEmailUnconfirmed): acá se verifica primero con el token que
// capturó el fakeMailer, mismo camino que un usuario real haciendo click en el link.
func TestVerifyCredentials_Success(t *testing.T) {
	svc, _, mailer := newUsersSvcWithMailer(t)

	created := registerUser(t, svc, "verify-ok@example.com")
	token := mailer.tokenFor(t, "verify-ok@example.com")
	if err := svc.VerifyEmail(context.Background(), token); err != nil {
		t.Fatalf("VerifyEmail() error = %v, want nil", err)
	}

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

// El registro deja el email sin confirmar: no puede loguearse hasta hacer click en el
// link mandado por mail (ver ADR-0012 — bloquear login hasta verificar).
func TestVerifyCredentials_BlocksUnconfirmedEmail(t *testing.T) {
	svc, _ := newUsersSvc(t)

	registerUser(t, svc, "unconfirmed@example.com")

	_, err := svc.VerifyCredentials(context.Background(), "unconfirmed@example.com", testPassword)
	if !errors.Is(err, users.ErrEmailNotConfirmed) {
		t.Fatalf("VerifyCredentials() con email sin confirmar: error = %v, want ErrEmailNotConfirmed", err)
	}
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusForbidden {
		t.Fatalf("VerifyCredentials() con email sin confirmar: code = %d, want %d", fiberErr.Code, fiber.StatusForbidden)
	}
}

func TestRegisterUser_SendsVerificationEmail(t *testing.T) {
	svc, _, mailer := newUsersSvcWithMailer(t)

	registerUser(t, svc, "sends-mail@example.com")

	token := mailer.tokenFor(t, "sends-mail@example.com")
	if token == "" {
		t.Fatal("RegisterUser() no mandó un token de verificación")
	}
}

func TestVerifyEmail_Success(t *testing.T) {
	svc, _, mailer := newUsersSvcWithMailer(t)

	registerUser(t, svc, "verify-email-ok@example.com")
	token := mailer.tokenFor(t, "verify-email-ok@example.com")

	if err := svc.VerifyEmail(context.Background(), token); err != nil {
		t.Fatalf("VerifyEmail() error = %v, want nil", err)
	}

	if _, err := svc.VerifyCredentials(context.Background(), "verify-email-ok@example.com", testPassword); err != nil {
		t.Fatalf("VerifyCredentials() tras verificar: error = %v, want nil", err)
	}
}

func TestVerifyEmail_InvalidToken(t *testing.T) {
	svc, _ := newUsersSvc(t)

	err := svc.VerifyEmail(context.Background(), "no-existe")
	if !errors.Is(err, users.ErrInvalidVerificationToken) {
		t.Fatalf("VerifyEmail() con token inexistente: error = %v, want ErrInvalidVerificationToken", err)
	}
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusBadRequest {
		t.Fatalf("VerifyEmail() con token inexistente: code = %d, want %d", fiberErr.Code, fiber.StatusBadRequest)
	}
}

// Un token ya usado no sirve una segunda vez: si no, alguien que interceptó el link una
// vez podría reusarlo indefinidamente.
func TestVerifyEmail_TokenAlreadyUsed(t *testing.T) {
	svc, _, mailer := newUsersSvcWithMailer(t)

	registerUser(t, svc, "verify-email-reuse@example.com")
	token := mailer.tokenFor(t, "verify-email-reuse@example.com")

	if err := svc.VerifyEmail(context.Background(), token); err != nil {
		t.Fatalf("VerifyEmail() primera vez: error = %v, want nil", err)
	}

	err := svc.VerifyEmail(context.Background(), token)
	if !errors.Is(err, users.ErrInvalidVerificationToken) {
		t.Fatalf("VerifyEmail() reusando el token: error = %v, want ErrInvalidVerificationToken", err)
	}
}

// ResendVerification nunca revela si el email existe: mismo criterio anti-enumeración
// que VerifyCredentials con ErrInvalidCredentials.
func TestResendVerification_UnknownEmail_DoesNotError(t *testing.T) {
	svc, _ := newUsersSvc(t)

	if err := svc.ResendVerification(context.Background(), "nadie@example.com"); err != nil {
		t.Fatalf("ResendVerification() con email inexistente: error = %v, want nil", err)
	}
}

func TestResendVerification_SendsNewToken(t *testing.T) {
	svc, _, mailer := newUsersSvcWithMailer(t)

	registerUser(t, svc, "resend@example.com")
	firstToken := mailer.tokenFor(t, "resend@example.com")

	if err := svc.ResendVerification(context.Background(), "resend@example.com"); err != nil {
		t.Fatalf("ResendVerification() error = %v, want nil", err)
	}
	secondToken := mailer.tokenFor(t, "resend@example.com")

	if secondToken == firstToken {
		t.Fatal("ResendVerification() no generó un token nuevo")
	}
	if err := svc.VerifyEmail(context.Background(), secondToken); err != nil {
		t.Fatalf("VerifyEmail() con el token reenviado: error = %v, want nil", err)
	}
}

// Ya verificado, ResendVerification no hace nada (pero tampoco falla): no tiene sentido
// mandar otro link.
func TestResendVerification_AlreadyVerified_NoOp(t *testing.T) {
	svc, _, mailer := newUsersSvcWithMailer(t)

	registerUser(t, svc, "already-verified@example.com")
	token := mailer.tokenFor(t, "already-verified@example.com")
	if err := svc.VerifyEmail(context.Background(), token); err != nil {
		t.Fatalf("VerifyEmail() error = %v, want nil", err)
	}

	if err := svc.ResendVerification(context.Background(), "already-verified@example.com"); err != nil {
		t.Fatalf("ResendVerification() con email ya verificado: error = %v, want nil", err)
	}
}

// Con requireEmailVerification=false (default de producción en fase alpha, ver
// ADR-0012) el registro deja la cuenta verificada de entrada y puede loguearse
// inmediatamente, sin pasar por ningún link.
func TestRegisterUser_VerificationOff_LeavesAccountVerified(t *testing.T) {
	svc, _ := newUsersSvcVerificationOff(t)

	registerUser(t, svc, "alpha@example.com")

	if _, err := svc.VerifyCredentials(context.Background(), "alpha@example.com", testPassword); err != nil {
		t.Fatalf("VerifyCredentials() con verificación desactivada: error = %v, want nil", err)
	}
}

// Con requireEmailVerification=false no hay que gastar un envío que nadie va a exigir:
// RegisterUser ni siquiera debe llamar al mailer.
func TestRegisterUser_VerificationOff_DoesNotSendMail(t *testing.T) {
	svc, mailer := newUsersSvcVerificationOff(t)

	registerUser(t, svc, "alpha-no-mail@example.com")

	if _, sent := mailer.verifyURLByEmail["alpha-no-mail@example.com"]; sent {
		t.Fatal("RegisterUser() mandó un mail de verificación con requireEmailVerification=false")
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

func TestChangePassword_Success(t *testing.T) {
	svc, _ := newUsersSvcVerificationOff(t)
	user := registerUser(t, svc, "change-password-ok@example.com")

	if err := svc.ChangePassword(context.Background(), user.ID, testPassword, "new-correct-horse-battery"); err != nil {
		t.Fatalf("ChangePassword() error = %v, want nil", err)
	}

	_, err := svc.VerifyCredentials(context.Background(), "change-password-ok@example.com", "new-correct-horse-battery")
	if err != nil {
		t.Fatalf("VerifyCredentials() con el password nuevo: error = %v, want nil", err)
	}
	_, err = svc.VerifyCredentials(context.Background(), "change-password-ok@example.com", testPassword)
	if !errors.Is(err, users.ErrInvalidCredentials) {
		t.Fatalf("VerifyCredentials() con el password viejo: error = %v, want ErrInvalidCredentials", err)
	}
}

func TestChangePassword_WrongCurrentPassword(t *testing.T) {
	svc, _ := newUsersSvcVerificationOff(t)
	user := registerUser(t, svc, "change-password-wrong@example.com")

	err := svc.ChangePassword(context.Background(), user.ID, "not-the-current-password", "new-correct-horse-battery")
	if !errors.Is(err, users.ErrInvalidCurrentPassword) {
		t.Fatalf("ChangePassword() con password actual incorrecta: error = %v, want ErrInvalidCurrentPassword", err)
	}
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusUnauthorized {
		t.Fatalf("ChangePassword() con password actual incorrecta: code = %d, want %d",
			fiberErr.Code, fiber.StatusUnauthorized)
	}

	// La password original sigue funcionando: el intento fallido no la tocó.
	_, err = svc.VerifyCredentials(context.Background(), "change-password-wrong@example.com", testPassword)
	if err != nil {
		t.Fatalf("VerifyCredentials() con el password original tras el intento fallido: error = %v, want nil", err)
	}
}

func TestChangePassword_TooShort(t *testing.T) {
	svc, _ := newUsersSvc(t)
	user := registerUser(t, svc, "change-password-short@example.com")

	err := svc.ChangePassword(context.Background(), user.ID, testPassword, "short")
	if !errors.Is(err, users.ErrPasswordTooShort) {
		t.Fatalf("ChangePassword() con password nuevo corto: error = %v, want ErrPasswordTooShort", err)
	}
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusBadRequest {
		t.Fatalf("ChangePassword() con password nuevo corto: code = %d, want %d", fiberErr.Code, fiber.StatusBadRequest)
	}
}

// Una cuenta de Google no tiene password propio: ChangePassword debe rechazarla con el
// mismo error que VerifyCredentials, no con un 500 por password_hash nulo.
func TestChangePassword_GoogleOnlyAccount(t *testing.T) {
	svc, _ := newUsersSvc(t)

	user, err := svc.FindOrCreateGoogleUser(
		context.Background(), "google-sub-change-password", "google-change-password@example.com", true,
	)
	if err != nil {
		t.Fatalf("FindOrCreateGoogleUser() error = %v", err)
	}

	err = svc.ChangePassword(context.Background(), user.ID, "anything", "new-correct-horse-battery")
	if !errors.Is(err, users.ErrGoogleOnlyAccount) {
		t.Fatalf("ChangePassword() sobre cuenta de Google: error = %v, want ErrGoogleOnlyAccount", err)
	}
}

func TestChangePassword_UnknownUser_ReturnsNotFound(t *testing.T) {
	svc, _ := newUsersSvc(t)

	err := svc.ChangePassword(
		context.Background(), "00000000-0000-0000-0000-000000000000", testPassword, "new-correct-horse-battery",
	)
	if !errors.Is(err, users.ErrUserNotFound) {
		t.Fatalf("ChangePassword() con usuario inexistente: error = %v, want ErrUserNotFound", err)
	}
}
func TestSearchUsers_ByUsernamePartial_IsCaseInsensitive(t *testing.T) {
	svc, _ := newUsersSvc(t)
	ctx := context.Background()

	if _, err := svc.RegisterUser(ctx, users.RegisterRequest{
		Username: "AtraxaPlayer", Email: "atraxa@example.com", Password: testPassword,
	}); err != nil {
		t.Fatalf("registrando usuario: %v", err)
	}
	if _, err := svc.RegisterUser(ctx, users.RegisterRequest{
		Username: "someoneelse", Email: "someoneelse@example.com", Password: testPassword,
	}); err != nil {
		t.Fatalf("registrando usuario: %v", err)
	}

	results, err := svc.SearchUsers(ctx, "00000000-0000-0000-0000-000000000000", "atraxa")
	if err != nil {
		t.Fatalf("SearchUsers() error = %v, want nil", err)
	}
	if len(results) != 1 || results[0].Username != "AtraxaPlayer" {
		t.Fatalf("SearchUsers(\"atraxa\") = %+v, want solo AtraxaPlayer", results)
	}
}

func TestSearchUsers_ByExactEmail(t *testing.T) {
	svc, _ := newUsersSvc(t)
	ctx := context.Background()
	user := registerUser(t, svc, "buscame@example.com")

	results, err := svc.SearchUsers(ctx, "00000000-0000-0000-0000-000000000000", "buscame@example.com")
	if err != nil {
		t.Fatalf("SearchUsers() error = %v, want nil", err)
	}
	if len(results) != 1 || results[0].ID != user.ID {
		t.Fatalf("SearchUsers(email exacto) = %+v, want %s", results, user.ID)
	}
}

// El email NO se busca parcial a propósito: permitiría enumerar direcciones ajenas por
// prefijo/substring (ver comentario en query.sql). El username es deliberadamente
// distinto del substring buscado, para no confundir "matchea por username" con "matchea
// por email parcial" (que es justo lo que este test verifica que NO pasa).
func TestSearchUsers_EmailParcialNoMatchea(t *testing.T) {
	svc, _ := newUsersSvc(t)
	ctx := context.Background()
	if _, err := svc.RegisterUser(ctx, users.RegisterRequest{
		Username: "unrelated-username", Email: "completo@example.com", Password: testPassword,
	}); err != nil {
		t.Fatalf("registrando usuario: %v", err)
	}

	results, err := svc.SearchUsers(ctx, "00000000-0000-0000-0000-000000000000", "completo")
	if err != nil {
		t.Fatalf("SearchUsers() error = %v, want nil", err)
	}
	if len(results) != 0 {
		t.Fatalf("SearchUsers() con substring de email = %+v, want vacío (el email exige match exacto)", results)
	}
}

func TestSearchUsers_ExcludeSelf(t *testing.T) {
	svc, _ := newUsersSvc(t)
	ctx := context.Background()
	self, err := svc.RegisterUser(ctx, users.RegisterRequest{
		Username: "buscandome", Email: "buscandome@example.com", Password: testPassword,
	})
	if err != nil {
		t.Fatalf("registrando usuario: %v", err)
	}

	results, err := svc.SearchUsers(ctx, self.ID, "buscandome")
	if err != nil {
		t.Fatalf("SearchUsers() error = %v, want nil", err)
	}
	if len(results) != 0 {
		t.Fatalf("SearchUsers() no debería incluir al propio requester: %+v", results)
	}
}

func TestSearchUsers_QueryTooShort_ReturnsBadRequest(t *testing.T) {
	svc, _ := newUsersSvc(t)

	_, err := svc.SearchUsers(context.Background(), "00000000-0000-0000-0000-000000000000", "a")
	if !errors.Is(err, users.ErrSearchQueryTooShort) {
		t.Fatalf("SearchUsers() con query de 1 char: error = %v, want ErrSearchQueryTooShort", err)
	}
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusBadRequest {
		t.Fatalf("SearchUsers() con query corto: code = %d, want %d", fiberErr.Code, fiber.StatusBadRequest)
	}
}

// Un slice nil serializa a JSON `null` en vez de `[]`, lo que rompe clientes que esperan
// siempre un array (ej. `results.length`/`.filter()` en JS/TS). SearchUsers debe devolver
// slice vacío inicializado, no nil, cuando no hay matches.
func TestSearchUsers_NoResults_ReturnsEmptySliceNotNil(t *testing.T) {
	svc, _ := newUsersSvc(t)

	results, err := svc.SearchUsers(context.Background(), "00000000-0000-0000-0000-000000000000", "no-existe-nadie-asi")
	if err != nil {
		t.Fatalf("SearchUsers() error = %v, want nil", err)
	}
	if results == nil {
		t.Fatalf("SearchUsers() sin resultados devolvió nil, want slice vacío (serializa a JSON null en vez de [])")
	}
	if len(results) != 0 {
		t.Fatalf("SearchUsers() = %+v, want vacío", results)
	}
}

func TestSearchUsers_ResultsAreCapped(t *testing.T) {
	svc, _ := newUsersSvc(t)
	ctx := context.Background()
	for i := 0; i < 12; i++ {
		if _, err := svc.RegisterUser(ctx, users.RegisterRequest{
			Username: fmt.Sprintf("capped-%02d", i),
			Email:    fmt.Sprintf("capped-%02d@example.com", i),
			Password: testPassword,
		}); err != nil {
			t.Fatalf("registrando usuario %d: %v", i, err)
		}
	}

	results, err := svc.SearchUsers(ctx, "00000000-0000-0000-0000-000000000000", "capped-")
	if err != nil {
		t.Fatalf("SearchUsers() error = %v, want nil", err)
	}
	if len(results) != 10 {
		t.Fatalf("SearchUsers() con 12 matches = %d resultados, want 10 (limit)", len(results))
	}
}
