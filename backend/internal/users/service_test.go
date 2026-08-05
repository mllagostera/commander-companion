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

// fakeMailer records the last verification link sent to each email, so
// VerifyEmail/ResendVerification can be exercised without depending on real Resend.
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

// tokenFor extracts the token from the last link sent to that email.
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
	// "users" drags along decks, refresh_tokens, email_verification_tokens, and others via CASCADE.
	testutil.Truncate(t, pool, "users")
	mailer := newFakeMailer()
	return users.NewService(pool, mailer, testWebAppURL, true), pool, mailer
}

// newUsersSvcVerificationOff instantiates the service with requireEmailVerification=false
// (the production default in alpha phase, see ADR-0012): registration should leave the
// account already verified and the mailer shouldn't receive any send.
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

// asFiberError translates the domain error returned by the service to its HTTP
// equivalent with common.MapError (services no longer depend on fiber, see
// internal/common/errors.go), so we can keep verifying the status code the
// client sees.
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
	if !user.HasPassword {
		t.Fatalf("RegisterUser() HasPassword = false, want true (cuenta email/password)")
	}
}

// The password never comes out in the DTO, but it does need to end up hashed in the DB:
// storing it in plaintext would go unnoticed just by looking at the response.
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

// A malformed ID isn't different from a nonexistent one for the client: both are 404.
func TestGetUser_MalformedID(t *testing.T) {
	svc, _ := newUsersSvc(t)

	_, err := svc.GetUser(context.Background(), "not-a-uuid")
	if !errors.Is(err, users.ErrUserNotFound) {
		t.Fatalf("GetUser() con id malformado: error = %v, want ErrUserNotFound", err)
	}
}

// VerifyCredentials only succeeds with the email already confirmed (see
// TestRegisterUser_LeavesEmailUnconfirmed): here it's verified first with the token
// captured by fakeMailer, the same path as a real user clicking the link.
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

// Registration leaves the email unconfirmed: it can't log in until clicking the
// link sent by mail (see ADR-0012 — block login until verified).
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

// An already-used token isn't valid a second time: otherwise someone who intercepted the
// link once could reuse it indefinitely.
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

// ResendVerification never reveals whether the email exists: same anti-enumeration
// criteria as VerifyCredentials with ErrInvalidCredentials.
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

// Already verified, ResendVerification does nothing (but doesn't fail either): there's
// no point sending another link.
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

// With requireEmailVerification=false (production default in alpha phase, see
// ADR-0012) registration leaves the account verified upfront and it can log in
// immediately, without going through any link.
func TestRegisterUser_VerificationOff_LeavesAccountVerified(t *testing.T) {
	svc, _ := newUsersSvcVerificationOff(t)

	registerUser(t, svc, "alpha@example.com")

	if _, err := svc.VerifyCredentials(context.Background(), "alpha@example.com", testPassword); err != nil {
		t.Fatalf("VerifyCredentials() con verificación desactivada: error = %v, want nil", err)
	}
}

// With requireEmailVerification=false there's no need to spend a send that nobody's
// going to require: RegisterUser shouldn't even call the mailer.
func TestRegisterUser_VerificationOff_DoesNotSendMail(t *testing.T) {
	svc, mailer := newUsersSvcVerificationOff(t)

	registerUser(t, svc, "alpha-no-mail@example.com")

	if _, sent := mailer.verifyURLByEmail["alpha-no-mail@example.com"]; sent {
		t.Fatal("RegisterUser() mandó un mail de verificación con requireEmailVerification=false")
	}
}

// A nonexistent email returns the SAME error as an incorrect password: it doesn't
// leak which emails are registered.
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
	// The username is derived from the local part of the email.
	if user.Username != "nuevo" {
		t.Fatalf("FindOrCreateGoogleUser() username = %q, want %q", user.Username, "nuevo")
	}
}

// The account created by Google has no password: trying to log in with
// email/password has to say so explicitly, not give "invalid credentials".
func TestFindOrCreateGoogleUser_CreatedAccountHasNoPassword(t *testing.T) {
	svc, _ := newUsersSvc(t)

	created, err := svc.FindOrCreateGoogleUser(context.Background(), "google-sub-2", "sinpass@example.com", true)
	if err != nil {
		t.Fatalf("FindOrCreateGoogleUser() error = %v, want nil", err)
	}
	if created.HasPassword {
		t.Fatalf("FindOrCreateGoogleUser() HasPassword = true, want false (cuenta sin password propio)")
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

// If the email already exists as an email/password account, Google links to that
// account instead of creating a duplicate, and the original password keeps working.
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

// Without an email verified by Google nothing is created or linked: otherwise anyone
// with an id_token from a lax provider could claim someone else's email.
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

// Two different Google accounts whose email local part matches can't clash: the
// second one's username is disambiguated with a suffix from the google id.
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

	// This is reflected in a subsequent read, not just in the write's own
	// response.
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

// The HTTP handler responds 404 if :id isn't the authenticated user's, without
// calling the service with someone else's id. The same ErrUserNotFound that backs
// that 404 is also what the service returns for any nonexistent id, so this test
// exercises that common path.
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

func TestUpdateUsername_Success(t *testing.T) {
	svc, _ := newUsersSvc(t)
	user := registerUser(t, svc, "update-username@example.com")

	updated, err := svc.UpdateUsername(context.Background(), user.ID, "NuevoUsername")
	if err != nil {
		t.Fatalf("UpdateUsername() error = %v, want nil", err)
	}
	if updated.Username != "NuevoUsername" {
		t.Fatalf("UpdateUsername() Username = %q, want %q", updated.Username, "NuevoUsername")
	}

	// This is reflected in a subsequent read, not just in the write's own response.
	fetched, err := svc.GetUser(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("GetUser() error = %v", err)
	}
	if fetched.Username != "NuevoUsername" {
		t.Fatalf("GetUser() Username = %q, want %q", fetched.Username, "NuevoUsername")
	}
}

func TestUpdateUsername_TrimsWhitespace(t *testing.T) {
	svc, _ := newUsersSvc(t)
	user := registerUser(t, svc, "update-username-trim@example.com")

	updated, err := svc.UpdateUsername(context.Background(), user.ID, "  ConEspacios  ")
	if err != nil {
		t.Fatalf("UpdateUsername() error = %v, want nil", err)
	}
	if updated.Username != "ConEspacios" {
		t.Fatalf("UpdateUsername() Username = %q, want %q (trimmed)", updated.Username, "ConEspacios")
	}
}

func TestUpdateUsername_Empty_ReturnsInvalidInput(t *testing.T) {
	svc, _ := newUsersSvc(t)
	user := registerUser(t, svc, "update-username-empty@example.com")

	_, err := svc.UpdateUsername(context.Background(), user.ID, "   ")
	if !errors.Is(err, users.ErrUsernameEmpty) {
		t.Fatalf("UpdateUsername(\"   \") error = %v, want ErrUsernameEmpty", err)
	}
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusBadRequest {
		t.Fatalf("UpdateUsername(\"   \") code = %d, want %d", fiberErr.Code, fiber.StatusBadRequest)
	}
}

func TestUpdateUsername_AlreadyTaken_ReturnsConflict(t *testing.T) {
	svc, _ := newUsersSvc(t)
	registerUser(t, svc, "username-owner@example.com")
	other := registerUser(t, svc, "username-challenger@example.com")

	_, err := svc.UpdateUsername(context.Background(), other.ID, "user-username-owner@example.com")
	if !errors.Is(err, users.ErrUsernameTaken) {
		t.Fatalf("UpdateUsername() con username tomado: error = %v, want ErrUsernameTaken", err)
	}
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusConflict {
		t.Fatalf("UpdateUsername() con username tomado: code = %d, want %d", fiberErr.Code, fiber.StatusConflict)
	}
}

func TestUpdateUsername_UnknownUser_ReturnsNotFound(t *testing.T) {
	svc, _ := newUsersSvc(t)

	_, err := svc.UpdateUsername(context.Background(), "00000000-0000-0000-0000-000000000000", "Handle")
	if !errors.Is(err, users.ErrUserNotFound) {
		t.Fatalf("UpdateUsername() con usuario inexistente: error = %v, want ErrUserNotFound", err)
	}
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusNotFound {
		t.Fatalf("UpdateUsername() con usuario inexistente: code = %d, want %d", fiberErr.Code, fiber.StatusNotFound)
	}
}

func TestIsUsernameAvailable_Free(t *testing.T) {
	svc, _ := newUsersSvc(t)

	available, err := svc.IsUsernameAvailable(context.Background(), "nunca-usado")
	if err != nil {
		t.Fatalf("IsUsernameAvailable() error = %v, want nil", err)
	}
	if !available {
		t.Fatal("IsUsernameAvailable() = false, want true (username never registered)")
	}
}

func TestIsUsernameAvailable_Taken(t *testing.T) {
	svc, _ := newUsersSvc(t)
	user := registerUser(t, svc, "availability-taken@example.com")

	available, err := svc.IsUsernameAvailable(context.Background(), user.Username)
	if err != nil {
		t.Fatalf("IsUsernameAvailable() error = %v, want nil", err)
	}
	if available {
		t.Fatal("IsUsernameAvailable() = true, want false (username already registered)")
	}
}

func TestIsUsernameAvailable_TrimsWhitespace(t *testing.T) {
	svc, _ := newUsersSvc(t)
	user := registerUser(t, svc, "availability-trim@example.com")

	available, err := svc.IsUsernameAvailable(context.Background(), "  "+user.Username+"  ")
	if err != nil {
		t.Fatalf("IsUsernameAvailable() error = %v, want nil", err)
	}
	if available {
		t.Fatal("IsUsernameAvailable() with surrounding whitespace = true, want false (same collision as UpdateUsername)")
	}
}

func TestIsUsernameAvailable_Empty_ReturnsInvalidInput(t *testing.T) {
	svc, _ := newUsersSvc(t)

	_, err := svc.IsUsernameAvailable(context.Background(), "   ")
	if !errors.Is(err, users.ErrUsernameEmpty) {
		t.Fatalf("IsUsernameAvailable(\"   \") error = %v, want ErrUsernameEmpty", err)
	}
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusBadRequest {
		t.Fatalf("IsUsernameAvailable(\"   \") code = %d, want %d", fiberErr.Code, fiber.StatusBadRequest)
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

	// The original password keeps working: the failed attempt didn't touch it.
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

// A Google account has no password of its own: ChangePassword must reject it with the
// same error as VerifyCredentials, not with a 500 due to a null password_hash.
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

// The email is deliberately NOT searched partially: it would allow enumerating other
// people's addresses by prefix/substring (see comment in query.sql). The username is
// deliberately different from the searched substring, so as not to confuse "matches by
// username" with "matches by partial email" (which is exactly what this test verifies does NOT happen).
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

// A nil slice serializes to JSON `null` instead of `[]`, which breaks clients that always
// expect an array (e.g. `results.length`/`.filter()` in JS/TS). SearchUsers must return
// an initialized empty slice, not nil, when there are no matches.
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
