package auth_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/usuario/commander-companion-backend/internal/auth"
	"github.com/usuario/commander-companion-backend/internal/testutil"
	"github.com/usuario/commander-companion-backend/internal/users"
)

const testPassword = "correct-horse-battery-staple"

// newAuthSvc creates an auth.Service and a users.Service over the same pool, with
// a reasonable test configuration (short TTLs, no Google configured).
func newAuthSvc(t *testing.T, pool *pgxpool.Pool, cfg auth.Config) (authSvc auth.Service, usersSvc users.Service) {
	t.Helper()
	usersSvc = testutil.NewUsersService(pool)
	authSvc = auth.NewService(pool, usersSvc, cfg)
	return authSvc, usersSvc
}

func defaultTestConfig() auth.Config {
	return auth.Config{
		JWTSecret:       []byte("test-secret"),
		AccessTokenTTL:  time.Minute,
		RefreshTokenTTL: time.Hour,
	}
}

// registerUser creates a test user via the real users service (not a raw
// INSERT), so the auth tests exercise the same path the backend produces, and it
// leaves it verified: these tests exercise auth.Login (token rotation, TTLs, etc.),
// not the email verification flow itself (see internal/users/service_test.go for
// that).
func registerUser(t *testing.T, pool *pgxpool.Pool, usersSvc users.Service, email string) *users.UserResponse {
	t.Helper()
	user, err := usersSvc.RegisterUser(context.Background(), users.RegisterRequest{
		Username: "user-" + email,
		Email:    email,
		Password: testPassword,
	})
	if err != nil {
		t.Fatalf("registrando usuario de test: %v", err)
	}
	testutil.VerifyUserEmail(t, pool, user.ID)
	return user
}

// ageRevocationsPastGrace moves every revocation a minute into the past, so the
// next Refresh with a rotated token sees it as older than the reuse grace window
// (ADR-0027) — the window is compared against the database clock, so this is the
// way to "wait" it out without sleeping.
func ageRevocationsPastGrace(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	if _, err := pool.Exec(context.Background(),
		"UPDATE refresh_tokens SET revoked_at = revoked_at - interval '1 minute' WHERE revoked_at IS NOT NULL"); err != nil {
		t.Fatalf("aging refresh token revocations: %v", err)
	}
}

func TestLogin_Success(t *testing.T) {
	pool := testutil.DB(t)
	testutil.Truncate(t, pool, "users")

	authSvc, usersSvc := newAuthSvc(t, pool, defaultTestConfig())
	registerUser(t, pool, usersSvc, "login-success@example.com")

	res, err := authSvc.Login(context.Background(), "login-success@example.com", testPassword)
	if err != nil {
		t.Fatalf("Login() error = %v, want nil", err)
	}
	if res.AccessToken == "" || res.RefreshToken == "" {
		t.Fatalf("Login() devolvió tokens vacíos: %+v", res)
	}
	if res.User.Email != "login-success@example.com" {
		t.Fatalf("Login() user email = %q, want %q", res.User.Email, "login-success@example.com")
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	pool := testutil.DB(t)
	testutil.Truncate(t, pool, "users")

	authSvc, usersSvc := newAuthSvc(t, pool, defaultTestConfig())
	registerUser(t, pool, usersSvc, "wrong-password@example.com")

	_, err := authSvc.Login(context.Background(), "wrong-password@example.com", "not-the-password")
	if !errors.Is(err, users.ErrInvalidCredentials) {
		t.Fatalf("Login() error = %v, want ErrInvalidCredentials", err)
	}
}

func TestLogin_UnknownEmail(t *testing.T) {
	pool := testutil.DB(t)
	testutil.Truncate(t, pool, "users")

	authSvc, _ := newAuthSvc(t, pool, defaultTestConfig())

	_, err := authSvc.Login(context.Background(), "does-not-exist@example.com", testPassword)
	if !errors.Is(err, users.ErrInvalidCredentials) {
		t.Fatalf("Login() error = %v, want ErrInvalidCredentials (no debe distinguir de wrong-password)", err)
	}
}

func TestLogin_GoogleOnlyAccount(t *testing.T) {
	pool := testutil.DB(t)
	testutil.Truncate(t, pool, "users")

	authSvc, usersSvc := newAuthSvc(t, pool, defaultTestConfig())
	_, err := usersSvc.FindOrCreateGoogleUser(context.Background(), "google-sub-123", "google-only@example.com", true)
	if err != nil {
		t.Fatalf("creando usuario de Google: %v", err)
	}

	_, err = authSvc.Login(context.Background(), "google-only@example.com", "cualquier-password")
	if !errors.Is(err, users.ErrGoogleOnlyAccount) {
		t.Fatalf("Login() error = %v, want ErrGoogleOnlyAccount", err)
	}
}

func TestRefresh_RotatesTokenAndInvalidatesThePrevious(t *testing.T) {
	pool := testutil.DB(t)
	testutil.Truncate(t, pool, "users")

	authSvc, usersSvc := newAuthSvc(t, pool, defaultTestConfig())
	registerUser(t, pool, usersSvc, "refresh-rotate@example.com")

	login, err := authSvc.Login(context.Background(), "refresh-rotate@example.com", testPassword)
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}

	refreshed, err := authSvc.Refresh(context.Background(), login.RefreshToken)
	if err != nil {
		t.Fatalf("Refresh() error = %v, want nil", err)
	}
	if refreshed.RefreshToken == login.RefreshToken {
		t.Fatalf("Refresh() no rotó el refresh token")
	}

	// The original refresh token has already been revoked (rotation); reusing it once
	// the grace window is over must fail.
	ageRevocationsPastGrace(t, pool)
	if _, err := authSvc.Refresh(context.Background(), login.RefreshToken); !errors.Is(err, auth.ErrInvalidToken) {
		t.Fatalf("Refresh() con token ya usado: error = %v, want ErrInvalidToken", err)
	}
}

func TestRefresh_ReuseOfRotatedToken_RevokesTheWholeSessionFamily(t *testing.T) {
	pool := testutil.DB(t)
	testutil.Truncate(t, pool, "users")

	authSvc, usersSvc := newAuthSvc(t, pool, defaultTestConfig())
	registerUser(t, pool, usersSvc, "refresh-reuse@example.com")

	login, err := authSvc.Login(context.Background(), "refresh-reuse@example.com", testPassword)
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}

	// Legitimate rotation: the client refreshes once, RT1 -> RT2.
	refreshed, err := authSvc.Refresh(context.Background(), login.RefreshToken)
	if err != nil {
		t.Fatalf("Refresh() error = %v, want nil", err)
	}

	// An attacker who stole RT1 (or a race between two legitimate refreshes) replays
	// it after it was already rotated away. This must not just fail for RT1 — it must
	// also kill RT2, the token that replaced it, so the thief (or whoever ends up
	// holding the surviving copy) can't keep the session alive either. Past the grace
	// window: inside it, a reuse reads as a concurrent refresh (see the tests below).
	ageRevocationsPastGrace(t, pool)
	if _, err := authSvc.Refresh(context.Background(), login.RefreshToken); !errors.Is(err, auth.ErrInvalidToken) {
		t.Fatalf("Refresh() con token reusado: error = %v, want ErrInvalidToken", err)
	}

	if _, err := authSvc.Refresh(context.Background(), refreshed.RefreshToken); !errors.Is(err, auth.ErrInvalidToken) {
		t.Fatalf(
			"Refresh() del token vigente tras detectar reuso: error = %v, want ErrInvalidToken "+
				"(la familia entera debe quedar revocada)",
			err,
		)
	}
}

// Two refreshes with the same token, the second arriving right after the first
// rotated it: what two web BFF instances (or two parallel app requests) do with one
// expired access token. Within the grace window that is not theft (ADR-0027): the
// second gets its own pair, and nothing is revoked.
func TestRefresh_ReuseWithinGrace_IssuesAFreshPairAndKeepsTheSession(t *testing.T) {
	pool := testutil.DB(t)
	testutil.Truncate(t, pool, "users")

	authSvc, usersSvc := newAuthSvc(t, pool, defaultTestConfig())
	registerUser(t, pool, usersSvc, "refresh-grace@example.com")

	login, err := authSvc.Login(context.Background(), "refresh-grace@example.com", testPassword)
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}

	first, err := authSvc.Refresh(context.Background(), login.RefreshToken)
	if err != nil {
		t.Fatalf("first Refresh() error = %v", err)
	}
	second, err := authSvc.Refresh(context.Background(), login.RefreshToken)
	if err != nil {
		t.Fatalf("second Refresh() with the same token within grace: error = %v, want nil", err)
	}
	if second.RefreshToken == first.RefreshToken || second.RefreshToken == login.RefreshToken {
		t.Fatalf("second Refresh() did not issue a new refresh token")
	}

	// Neither pair was revoked: both stay usable.
	for name, token := range map[string]string{"first": first.RefreshToken, "second": second.RefreshToken} {
		if _, err := authSvc.Refresh(context.Background(), token); err != nil {
			t.Fatalf("Refresh() with the %s pair's token: error = %v, want nil", name, err)
		}
	}
}

// The grace window only covers a token whose replacement is still active. Once the
// client logged out with that replacement, presenting the old one again is a replay,
// grace window or not: it fails and revokes every session of the account.
func TestRefresh_ReuseWithinGrace_AfterTheSuccessorWasRevoked_RevokesTheFamily(t *testing.T) {
	pool := testutil.DB(t)
	testutil.Truncate(t, pool, "users")

	authSvc, usersSvc := newAuthSvc(t, pool, defaultTestConfig())
	registerUser(t, pool, usersSvc, "refresh-grace-logout@example.com")

	login, err := authSvc.Login(context.Background(), "refresh-grace-logout@example.com", testPassword)
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	otherDevice, err := authSvc.Login(context.Background(), "refresh-grace-logout@example.com", testPassword)
	if err != nil {
		t.Fatalf("Login() on a second device error = %v", err)
	}

	refreshed, err := authSvc.Refresh(context.Background(), login.RefreshToken)
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if err := authSvc.Logout(context.Background(), refreshed.RefreshToken); err != nil {
		t.Fatalf("Logout() error = %v", err)
	}

	if _, err := authSvc.Refresh(context.Background(), login.RefreshToken); !errors.Is(err, auth.ErrInvalidToken) {
		t.Fatalf("Refresh() with a token whose successor was logged out: error = %v, want ErrInvalidToken", err)
	}
	if _, err := authSvc.Refresh(context.Background(), otherDevice.RefreshToken); !errors.Is(err, auth.ErrInvalidToken) {
		t.Fatalf("Refresh() on the other device after the replay: error = %v, want ErrInvalidToken "+
			"(the whole family must be revoked)", err)
	}
}

// A token revoked by Logout has no replacement, so it never qualifies for the grace
// window, however recent the logout.
func TestRefresh_ReuseRightAfterLogout_IsNotCoveredByGrace(t *testing.T) {
	pool := testutil.DB(t)
	testutil.Truncate(t, pool, "users")

	authSvc, usersSvc := newAuthSvc(t, pool, defaultTestConfig())
	registerUser(t, pool, usersSvc, "refresh-grace-plain-logout@example.com")

	login, err := authSvc.Login(context.Background(), "refresh-grace-plain-logout@example.com", testPassword)
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	otherDevice, err := authSvc.Login(context.Background(), "refresh-grace-plain-logout@example.com", testPassword)
	if err != nil {
		t.Fatalf("Login() on a second device error = %v", err)
	}
	if err := authSvc.Logout(context.Background(), login.RefreshToken); err != nil {
		t.Fatalf("Logout() error = %v", err)
	}

	if _, err := authSvc.Refresh(context.Background(), login.RefreshToken); !errors.Is(err, auth.ErrInvalidToken) {
		t.Fatalf("Refresh() right after Logout(): error = %v, want ErrInvalidToken", err)
	}
	if _, err := authSvc.Refresh(context.Background(), otherDevice.RefreshToken); !errors.Is(err, auth.ErrInvalidToken) {
		t.Fatalf("Refresh() on the other device: error = %v, want ErrInvalidToken (reuse of a "+
			"logged-out token still revokes the family)", err)
	}
}

// Several refreshes with one token at the same instant. Whether each one reads the
// token before or after another has rotated it depends on scheduling, so this covers
// both paths at once: the conditional rotation (only one records its successor) and
// the grace window (the late ones). Every one must succeed and none may trigger reuse
// detection. The deterministic version of the late case is
// TestRefresh_ReuseWithinGrace_IssuesAFreshPairAndKeepsTheSession.
func TestRefresh_ConcurrentRefreshesWithTheSameToken_AllSucceed(t *testing.T) {
	pool := testutil.DB(t)
	testutil.Truncate(t, pool, "users")

	authSvc, usersSvc := newAuthSvc(t, pool, defaultTestConfig())
	registerUser(t, pool, usersSvc, "refresh-concurrent@example.com")

	login, err := authSvc.Login(context.Background(), "refresh-concurrent@example.com", testPassword)
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	otherDevice, err := authSvc.Login(context.Background(), "refresh-concurrent@example.com", testPassword)
	if err != nil {
		t.Fatalf("Login() on a second device error = %v", err)
	}

	const parallel = 5
	errs := make(chan error, parallel)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for range parallel {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := authSvc.Refresh(context.Background(), login.RefreshToken)
			errs <- err
		}()
	}
	close(start)
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent Refresh() error = %v, want nil", err)
		}
	}
	if _, err := authSvc.Refresh(context.Background(), otherDevice.RefreshToken); err != nil {
		t.Fatalf("Refresh() on the other device after the race: error = %v, want nil (no family revocation)", err)
	}
}

// Deactivation (see ADR-0018) is done via internal/admin, not internal/auth/internal/users
// themselves — this test flips the column directly with SQL, same as a real admin action
// would leave it. The refresh token was issued while the account was still active; Refresh
// must still reject it once is_active flips, bounding a deactivated account's exposure to
// at most one access-token TTL (see ADR-0018).
func TestRefresh_BlocksDeactivatedAccount(t *testing.T) {
	pool := testutil.DB(t)
	testutil.Truncate(t, pool, "users")

	authSvc, usersSvc := newAuthSvc(t, pool, defaultTestConfig())
	registerUser(t, pool, usersSvc, "refresh-deactivated@example.com")

	login, err := authSvc.Login(context.Background(), "refresh-deactivated@example.com", testPassword)
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}

	if _, err := pool.Exec(context.Background(), "UPDATE users SET is_active = false WHERE email = $1",
		"refresh-deactivated@example.com"); err != nil {
		t.Fatalf("desactivando cuenta de test: %v", err)
	}

	if _, err := authSvc.Refresh(context.Background(), login.RefreshToken); !errors.Is(err, users.ErrAccountDeactivated) {
		t.Fatalf("Refresh() con cuenta desactivada: error = %v, want ErrAccountDeactivated", err)
	}
}

func TestRefresh_InvalidToken(t *testing.T) {
	pool := testutil.DB(t)
	testutil.Truncate(t, pool, "users")

	authSvc, _ := newAuthSvc(t, pool, defaultTestConfig())

	_, err := authSvc.Refresh(context.Background(), "un-token-que-nunca-existio")
	if !errors.Is(err, auth.ErrInvalidToken) {
		t.Fatalf("Refresh() error = %v, want ErrInvalidToken", err)
	}
}

func TestRefresh_ExpiredToken(t *testing.T) {
	pool := testutil.DB(t)
	testutil.Truncate(t, pool, "users")

	expiredCfg := defaultTestConfig()
	expiredCfg.RefreshTokenTTL = -1 * time.Minute // already expired at the moment it's issued

	expiredSvc, usersSvc := newAuthSvc(t, pool, expiredCfg)
	registerUser(t, pool, usersSvc, "refresh-expired@example.com")

	login, err := expiredSvc.Login(context.Background(), "refresh-expired@example.com", testPassword)
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}

	authSvc, _ := newAuthSvc(t, pool, defaultTestConfig())
	if _, err := authSvc.Refresh(context.Background(), login.RefreshToken); !errors.Is(err, auth.ErrInvalidToken) {
		t.Fatalf("Refresh() con token vencido: error = %v, want ErrInvalidToken", err)
	}
}

func TestLogout_RevokesRefreshToken(t *testing.T) {
	pool := testutil.DB(t)
	testutil.Truncate(t, pool, "users")

	authSvc, usersSvc := newAuthSvc(t, pool, defaultTestConfig())
	registerUser(t, pool, usersSvc, "logout@example.com")

	login, err := authSvc.Login(context.Background(), "logout@example.com", testPassword)
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}

	if err := authSvc.Logout(context.Background(), login.RefreshToken); err != nil {
		t.Fatalf("Logout() error = %v, want nil", err)
	}

	if _, err := authSvc.Refresh(context.Background(), login.RefreshToken); !errors.Is(err, auth.ErrInvalidToken) {
		t.Fatalf("Refresh() tras Logout(): error = %v, want ErrInvalidToken", err)
	}
}

func TestMe_ReturnsAuthenticatedUser(t *testing.T) {
	pool := testutil.DB(t)
	testutil.Truncate(t, pool, "users")

	authSvc, usersSvc := newAuthSvc(t, pool, defaultTestConfig())
	created := registerUser(t, pool, usersSvc, "me@example.com")

	res, err := authSvc.Me(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("Me() error = %v, want nil", err)
	}
	if res.Email != "me@example.com" {
		t.Fatalf("Me() email = %q, want %q", res.Email, "me@example.com")
	}
}

func TestMe_UnknownUser(t *testing.T) {
	pool := testutil.DB(t)
	testutil.Truncate(t, pool, "users")

	authSvc, _ := newAuthSvc(t, pool, defaultTestConfig())

	_, err := authSvc.Me(context.Background(), "00000000-0000-0000-0000-000000000000")
	if !errors.Is(err, users.ErrUserNotFound) {
		t.Fatalf("Me() error = %v, want ErrUserNotFound", err)
	}
}

func TestGoogleLogin_NotConfigured(t *testing.T) {
	pool := testutil.DB(t)
	testutil.Truncate(t, pool, "users")

	cfg := defaultTestConfig()
	cfg.GoogleClientID = "" // server without GOOGLE_CLIENT_ID (see .env.example)
	authSvc, _ := newAuthSvc(t, pool, cfg)

	_, err := authSvc.GoogleLogin(context.Background(), "cualquier-id-token")
	if !errors.Is(err, auth.ErrGoogleAuthNotConfigured) {
		t.Fatalf("GoogleLogin() error = %v, want ErrGoogleAuthNotConfigured", err)
	}
}
