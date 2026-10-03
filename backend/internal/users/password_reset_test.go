package users_test

import (
	"context"
	"errors"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/usuario/commander-companion-backend/internal/testutil"
	"github.com/usuario/commander-companion-backend/internal/users"
)

const newTestPassword = "a-brand-new-password"

// requestReset asks for a reset link for email and returns the token it carried.
func requestReset(t *testing.T, svc users.Service, mailer *fakeMailer, email string) string {
	t.Helper()
	if err := svc.RequestPasswordReset(context.Background(), email, ""); err != nil {
		t.Fatalf("RequestPasswordReset() error = %v, want nil", err)
	}
	return mailer.resetTokenFor(t, email)
}

// newVerifiedUsersSvc is newUsersSvcWithMailer with email verification off, so registered
// accounts can log in right away and the tests only exercise the reset flow.
func newVerifiedUsersSvc(t *testing.T) (users.Service, *pgxpool.Pool, *fakeMailer) {
	t.Helper()
	pool := testutil.DB(t)
	// "users" drags along refresh_tokens and password_reset_tokens via CASCADE.
	testutil.Truncate(t, pool, "users")
	mailer := newFakeMailer()
	return users.NewService(pool, mailer, &fakeNotifier{}, testWebAppURL, false), pool, mailer
}

func TestResetPassword_Success(t *testing.T) {
	svc, _, mailer := newVerifiedUsersSvc(t)
	ctx := context.Background()

	registerUser(t, svc, "reset-ok@example.com")
	token := requestReset(t, svc, mailer, "reset-ok@example.com")

	if err := svc.ResetPassword(ctx, token, newTestPassword); err != nil {
		t.Fatalf("ResetPassword() error = %v, want nil", err)
	}

	if _, err := svc.VerifyCredentials(ctx, "reset-ok@example.com", newTestPassword); err != nil {
		t.Fatalf("VerifyCredentials() with the new password: error = %v, want nil", err)
	}
	_, err := svc.VerifyCredentials(ctx, "reset-ok@example.com", testPassword)
	if !errors.Is(err, users.ErrInvalidCredentials) {
		t.Fatalf("VerifyCredentials() with the old password: error = %v, want ErrInvalidCredentials", err)
	}
}

// RequestPasswordReset never reveals whether the email exists (same anti-enumeration
// contract as ResendVerification), and of course sends nothing.
func TestRequestPasswordReset_UnknownEmail_DoesNotError(t *testing.T) {
	svc, _, mailer := newVerifiedUsersSvc(t)

	if err := svc.RequestPasswordReset(context.Background(), "nobody@example.com", ""); err != nil {
		t.Fatalf("RequestPasswordReset() with unknown email: error = %v, want nil", err)
	}
	if len(mailer.resetURLByEmail) != 0 {
		t.Fatalf("RequestPasswordReset() with unknown email sent %v, want nothing", mailer.resetURLByEmail)
	}
}

func TestRequestPasswordReset_IsCaseInsensitive(t *testing.T) {
	svc, _, mailer := newVerifiedUsersSvc(t)

	registerUser(t, svc, "reset-case@example.com")
	if err := svc.RequestPasswordReset(context.Background(), "  Reset-Case@Example.com ", ""); err != nil {
		t.Fatalf("RequestPasswordReset() error = %v, want nil", err)
	}
	mailer.resetTokenFor(t, "reset-case@example.com")
}

func TestRequestPasswordReset_PassesLocaleToMailer(t *testing.T) {
	svc, _, mailer := newVerifiedUsersSvc(t)

	registerUser(t, svc, "reset-locale@example.com")
	if err := svc.RequestPasswordReset(context.Background(), "reset-locale@example.com", "ca"); err != nil {
		t.Fatalf("RequestPasswordReset() error = %v, want nil", err)
	}
	if got := mailer.resetLocaleByEmail["reset-locale@example.com"]; got != "ca" {
		t.Fatalf("locale = %q, want %q", got, "ca")
	}
}

// A deactivated account couldn't log in after resetting anyway, so no link is sent.
func TestRequestPasswordReset_DeactivatedAccount_NoOp(t *testing.T) {
	svc, pool, mailer := newVerifiedUsersSvc(t)

	created := registerUser(t, svc, "reset-deactivated@example.com")
	const deactivateQuery = "UPDATE users SET is_active = false WHERE id = $1"
	if _, err := pool.Exec(context.Background(), deactivateQuery, created.ID); err != nil {
		t.Fatalf("deactivating test account: %v", err)
	}

	if err := svc.RequestPasswordReset(context.Background(), "reset-deactivated@example.com", ""); err != nil {
		t.Fatalf("RequestPasswordReset() error = %v, want nil", err)
	}
	if _, sent := mailer.resetURLByEmail["reset-deactivated@example.com"]; sent {
		t.Fatal("RequestPasswordReset() sent a link to a deactivated account")
	}
}

func TestResetPassword_InvalidToken(t *testing.T) {
	svc, _, _ := newVerifiedUsersSvc(t)

	err := svc.ResetPassword(context.Background(), "does-not-exist", newTestPassword)
	if !errors.Is(err, users.ErrInvalidPasswordResetToken) {
		t.Fatalf("ResetPassword() with unknown token: error = %v, want ErrInvalidPasswordResetToken", err)
	}
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusBadRequest {
		t.Fatalf("ResetPassword() with unknown token: code = %d, want %d", fiberErr.Code, fiber.StatusBadRequest)
	}
}

func TestResetPassword_TokenAlreadyUsed(t *testing.T) {
	svc, _, mailer := newVerifiedUsersSvc(t)
	ctx := context.Background()

	registerUser(t, svc, "reset-reuse@example.com")
	token := requestReset(t, svc, mailer, "reset-reuse@example.com")

	if err := svc.ResetPassword(ctx, token, newTestPassword); err != nil {
		t.Fatalf("ResetPassword() first time: error = %v, want nil", err)
	}
	if err := svc.ResetPassword(ctx, token, "yet-another-password"); !errors.Is(err, users.ErrInvalidPasswordResetToken) {
		t.Fatalf("ResetPassword() reusing the token: error = %v, want ErrInvalidPasswordResetToken", err)
	}
}

// Redeeming one link burns every other pending one: an older email still sitting in the
// inbox must not be able to change the password again.
func TestResetPassword_BurnsOtherPendingTokens(t *testing.T) {
	svc, _, mailer := newVerifiedUsersSvc(t)
	ctx := context.Background()

	registerUser(t, svc, "reset-burn@example.com")
	older := requestReset(t, svc, mailer, "reset-burn@example.com")
	newer := requestReset(t, svc, mailer, "reset-burn@example.com")

	if err := svc.ResetPassword(ctx, newer, newTestPassword); err != nil {
		t.Fatalf("ResetPassword() with the newer token: error = %v, want nil", err)
	}
	if err := svc.ResetPassword(ctx, older, "yet-another-password"); !errors.Is(err, users.ErrInvalidPasswordResetToken) {
		t.Fatalf("ResetPassword() with the older token: error = %v, want ErrInvalidPasswordResetToken", err)
	}
}

func TestResetPassword_ExpiredToken(t *testing.T) {
	svc, pool, mailer := newVerifiedUsersSvc(t)
	ctx := context.Background()

	registerUser(t, svc, "reset-expired@example.com")
	token := requestReset(t, svc, mailer, "reset-expired@example.com")
	const expireQuery = "UPDATE password_reset_tokens SET expires_at = now() - interval '1 minute'"
	if _, err := pool.Exec(ctx, expireQuery); err != nil {
		t.Fatalf("expiring test token: %v", err)
	}

	if err := svc.ResetPassword(ctx, token, newTestPassword); !errors.Is(err, users.ErrInvalidPasswordResetToken) {
		t.Fatalf("ResetPassword() with expired token: error = %v, want ErrInvalidPasswordResetToken", err)
	}
}

// A too-short password is rejected before touching the token, so the user can retry
// with the same link.
func TestResetPassword_ShortPassword_KeepsTokenUsable(t *testing.T) {
	svc, _, mailer := newVerifiedUsersSvc(t)
	ctx := context.Background()

	registerUser(t, svc, "reset-short@example.com")
	token := requestReset(t, svc, mailer, "reset-short@example.com")

	err := svc.ResetPassword(ctx, token, "short")
	if !errors.Is(err, users.ErrPasswordTooShort) {
		t.Fatalf("ResetPassword() with short password: error = %v, want ErrPasswordTooShort", err)
	}
	if err := svc.ResetPassword(ctx, token, newTestPassword); err != nil {
		t.Fatalf("ResetPassword() retrying with a valid password: error = %v, want nil", err)
	}
}

// A reset signs the account out everywhere: whoever had the old password may still
// hold a session.
func TestResetPassword_RevokesRefreshTokens(t *testing.T) {
	svc, pool, mailer := newVerifiedUsersSvc(t)
	ctx := context.Background()

	created := registerUser(t, svc, "reset-revoke@example.com")
	const insertRefresh = `INSERT INTO refresh_tokens (user_id, token_hash, expires_at)
		VALUES ($1, 'reset-revoke-hash', now() + interval '1 day')`
	if _, err := pool.Exec(ctx, insertRefresh, created.ID); err != nil {
		t.Fatalf("inserting test refresh token: %v", err)
	}

	token := requestReset(t, svc, mailer, "reset-revoke@example.com")
	if err := svc.ResetPassword(ctx, token, newTestPassword); err != nil {
		t.Fatalf("ResetPassword() error = %v, want nil", err)
	}

	var active int
	const countActive = "SELECT count(*) FROM refresh_tokens WHERE user_id = $1 AND revoked_at IS NULL"
	if err := pool.QueryRow(ctx, countActive, created.ID).Scan(&active); err != nil {
		t.Fatalf("counting refresh tokens: %v", err)
	}
	if active != 0 {
		t.Fatalf("active refresh tokens after reset = %d, want 0", active)
	}
}

// Redeeming a link sent to the address proves the user owns it, so an account that never
// confirmed its email can log in after resetting.
func TestResetPassword_ConfirmsEmail(t *testing.T) {
	svc, _, mailer := newUsersSvcWithMailer(t)
	ctx := context.Background()

	registerUser(t, svc, "reset-unconfirmed@example.com")
	token := requestReset(t, svc, mailer, "reset-unconfirmed@example.com")

	if err := svc.ResetPassword(ctx, token, newTestPassword); err != nil {
		t.Fatalf("ResetPassword() error = %v, want nil", err)
	}
	if _, err := svc.VerifyCredentials(ctx, "reset-unconfirmed@example.com", newTestPassword); err != nil {
		t.Fatalf("VerifyCredentials() after reset: error = %v, want nil", err)
	}
}

// A Google-only account can use the reset flow to add a password of its own.
func TestResetPassword_GoogleOnlyAccountGetsPassword(t *testing.T) {
	svc, _, mailer := newVerifiedUsersSvc(t)
	ctx := context.Background()

	if _, err := svc.FindOrCreateGoogleUser(ctx, "google-sub-reset", "reset-google@example.com", true); err != nil {
		t.Fatalf("FindOrCreateGoogleUser() error = %v", err)
	}
	token := requestReset(t, svc, mailer, "reset-google@example.com")

	if err := svc.ResetPassword(ctx, token, newTestPassword); err != nil {
		t.Fatalf("ResetPassword() error = %v, want nil", err)
	}
	got, err := svc.VerifyCredentials(ctx, "reset-google@example.com", newTestPassword)
	if err != nil {
		t.Fatalf("VerifyCredentials() after reset: error = %v, want nil", err)
	}
	if !got.HasPassword {
		t.Fatal("HasPassword = false after reset, want true")
	}
}
