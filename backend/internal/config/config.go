// Package config centraliza la lectura de la configuración del servidor a
// partir de variables de entorno. Antes de este paquete, cmd/api/main.go leía
// os.Getenv suelto en 4 puntos distintos (conexión a BD, puerto, auth, CORS)
// sin un struct que los agrupara.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/usuario/commander-companion-backend/internal/auth"
	"github.com/usuario/commander-companion-backend/internal/email"
)

const (
	defaultAccessTokenTTL  = 15 * time.Minute
	defaultRefreshTokenTTL = 30 * 24 * time.Hour
	defaultPort            = "8080"
	defaultWebAppURL       = "http://localhost:3000"
)

// Config agrupa toda la configuración del servidor leída de variables de
// entorno.
type Config struct {
	DBURL              string
	Port               string
	CORSAllowedOrigins string
	// WebAppURL es la URL pública del cliente web (Nuxt), usada para armar links que
	// mandan los mails transaccionales (ej. verificación de email).
	WebAppURL string
	// RequireEmailVerification controla si el registro exige confirmar el email antes
	// de poder loguearse (ver ADR-0012). Default false: en fase alpha no tiene sentido
	// ni mandar el mail ni bloquear el login por esto, así que las cuentas nuevas
	// quedan verificadas de alta y RegisterUser ni genera el token ni llama al mailer.
	RequireEmailVerification bool
	Auth                     auth.Config
	Email                    email.Config
}

// Load lee la configuración completa desde variables de entorno, con los
// mismos defaults que usaba cmd/api/main.go antes de este paquete.
func Load() (Config, error) {
	authCfg, err := loadAuthConfig()
	if err != nil {
		return Config{}, err
	}

	return Config{
		DBURL:                    dbURL(),
		Port:                     port(),
		CORSAllowedOrigins:       corsAllowedOrigins(),
		WebAppURL:                webAppURL(),
		RequireEmailVerification: boolEnv("REQUIRE_EMAIL_VERIFICATION", false),
		Auth:                     authCfg,
		Email:                    loadEmailConfig(),
	}, nil
}

func dbURL() string {
	if v := os.Getenv("DB_URL"); v != "" {
		return v
	}
	// Credencial de desarrollo local por defecto; se sobreescribe con DB_URL en cualquier otro entorno.
	return "postgres://postgres:postgres@localhost:5432/commander?sslmode=disable"
}

func port() string {
	if v := os.Getenv("PORT"); v != "" {
		return v
	}
	return defaultPort
}

// corsAllowedOrigins lee los orígenes permitidos para CORS. Por defecto, en
// desarrollo se permite cualquier origen (no se usan cookies/credentials,
// solo Bearer tokens); en cualquier otro entorno hay que restringirlo
// explícitamente.
func corsAllowedOrigins() string {
	if v := os.Getenv("CORS_ALLOWED_ORIGINS"); v != "" {
		return v
	}
	return "*"
}

func webAppURL() string {
	if v := os.Getenv("WEB_APP_URL"); v != "" {
		return v
	}
	return defaultWebAppURL
}

// loadEmailConfig lee la configuración de Resend. Si RESEND_API_KEY está vacío (dev
// sin cuenta de Resend), email.NewResendClient usa un mailer de consola en su lugar
// (ver internal/email), así que no hace falta validar nada acá.
func loadEmailConfig() email.Config {
	return email.Config{
		APIKey:                os.Getenv("RESEND_API_KEY"),
		FromAddress:           os.Getenv("EMAIL_FROM"),
		VerifyEmailTemplateID: os.Getenv("RESEND_VERIFY_EMAIL_TEMPLATE_ID"),
	}
}

func loadAuthConfig() (auth.Config, error) {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		// Secreto de desarrollo local por defecto; se sobreescribe con JWT_SECRET en cualquier otro entorno.
		//nolint:gosec // dev-only default, not a real secret
		secret = "dev-insecure-jwt-secret-change-me"
	}

	accessTTL, err := parseDurationEnv("ACCESS_TOKEN_TTL", defaultAccessTokenTTL)
	if err != nil {
		return auth.Config{}, err
	}

	refreshTTL, err := parseDurationEnv("REFRESH_TOKEN_TTL", defaultRefreshTokenTTL)
	if err != nil {
		return auth.Config{}, err
	}

	return auth.Config{
		JWTSecret:       []byte(secret),
		AccessTokenTTL:  accessTTL,
		RefreshTokenTTL: refreshTTL,
		GoogleClientID:  os.Getenv("GOOGLE_CLIENT_ID"),
	}, nil
}

// boolEnv lee una variable de entorno booleana ("true"/"false", case-insensitive vía
// strconv.ParseBool que también acepta "1"/"0"). Cualquier valor que no parsee cae al
// default en vez de fallar el arranque: no vale la pena bloquear el servidor por un
// typo en un flag de este tipo.
func boolEnv(name string, fallback bool) bool {
	raw := os.Getenv(name)
	if raw == "" {
		return fallback
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		return fallback
	}
	return v
}

func parseDurationEnv(name string, fallback time.Duration) (time.Duration, error) {
	raw := os.Getenv(name)
	if raw == "" {
		return fallback, nil
	}

	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("parsing %s: %w", name, err)
	}
	return d, nil
}
