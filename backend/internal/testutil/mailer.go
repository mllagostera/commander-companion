package testutil

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/usuario/commander-companion-backend/internal/users"
)

// testWebAppURL is the base used to build verification links in tests; none of the
// tests that use this helper actually resolve the link.
const testWebAppURL = "http://localhost:3000"

// noopMailer doesn't send anything: it's enough to instantiate users.Service in the
// modules that only need it as a dependency (they don't exercise the email
// verification flow itself). internal/users/service_test.go uses its own fake that
// does record the sent token, so it can test VerifyEmail.
type noopMailer struct{}

// SendVerificationEmail does nothing (see noopMailer).
func (noopMailer) SendVerificationEmail(context.Context, string, string, string) error {
	return nil
}

// NewUsersService instantiates users.Service with a no-op mailer and email
// verification required (unlike the production default — see ADR-0012 —, here
// requireEmailVerification is deliberately left true: this way registration still
// leaves the account unconfirmed and VerifyUserEmail, below, remains necessary and
// meaningful for the tests that need to log in).
func NewUsersService(pool *pgxpool.Pool) users.Service {
	return users.NewService(pool, noopMailer{}, testWebAppURL, true)
}

// VerifyUserEmail marks a user as having a confirmed email without going through the
// token/mail flow, for tests of other modules that need an account that can log in
// (VerifyCredentials now requires email_verified, see internal/users/service.go) but
// aren't testing the verification flow itself.
func VerifyUserEmail(t *testing.T, pool *pgxpool.Pool, userID string) {
	t.Helper()
	const query = "UPDATE users SET email_verified = true WHERE id = $1"
	if _, err := pool.Exec(context.Background(), query, userID); err != nil {
		t.Fatalf("marcando email verificado para %s: %v", userID, err)
	}
}
