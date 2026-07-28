package auth_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/usuario/commander-companion-backend/internal/auth"
	"github.com/usuario/commander-companion-backend/internal/testutil"
	"github.com/usuario/commander-companion-backend/internal/users"
)

const testPassword = "correct-horse-battery-staple"

// newAuthSvc crea un auth.Service y un users.Service sobre el mismo pool, con
// una configuración de test razonable (TTLs cortos, sin Google configurado).
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

// registerUser crea un usuario de test vía el servicio real de users (no INSERT
// crudo), así los tests de auth ejercitan el mismo camino que produce el backend, y lo
// deja verificado: estos tests ejercitan auth.Login (rotación de tokens, TTLs, etc.),
// no el flujo de verificación de email en sí (ver internal/users/service_test.go para
// ese).
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

	// El refresh token original ya fue revocado (rotación); reusarlo debe fallar.
	if _, err := authSvc.Refresh(context.Background(), login.RefreshToken); !errors.Is(err, auth.ErrInvalidToken) {
		t.Fatalf("Refresh() con token ya usado: error = %v, want ErrInvalidToken", err)
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
	expiredCfg.RefreshTokenTTL = -1 * time.Minute // ya vencido en el momento de emitirlo

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
	cfg.GoogleClientID = "" // servidor sin GOOGLE_CLIENT_ID (ver .env.example)
	authSvc, _ := newAuthSvc(t, pool, cfg)

	_, err := authSvc.GoogleLogin(context.Background(), "cualquier-id-token")
	if !errors.Is(err, auth.ErrGoogleAuthNotConfigured) {
		t.Fatalf("GoogleLogin() error = %v, want ErrGoogleAuthNotConfigured", err)
	}
}
