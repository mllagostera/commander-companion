// Package config centraliza la lectura de la configuración del servidor a
// partir de variables de entorno. Antes de este paquete, cmd/api/main.go leía
// os.Getenv suelto en 4 puntos distintos (conexión a BD, puerto, auth, CORS)
// sin un struct que los agrupara.
package config

import (
	"fmt"
	"os"
	"time"

	"github.com/usuario/commander-companion-backend/internal/auth"
)

const (
	defaultAccessTokenTTL  = 15 * time.Minute
	defaultRefreshTokenTTL = 30 * 24 * time.Hour
	defaultPort            = "8080"
)

// Config agrupa toda la configuración del servidor leída de variables de
// entorno.
type Config struct {
	DBURL              string
	Port               string
	CORSAllowedOrigins string
	Auth               auth.Config
}

// Load lee la configuración completa desde variables de entorno, con los
// mismos defaults que usaba cmd/api/main.go antes de este paquete.
func Load() (Config, error) {
	authCfg, err := loadAuthConfig()
	if err != nil {
		return Config{}, err
	}

	return Config{
		DBURL:              dbURL(),
		Port:               port(),
		CORSAllowedOrigins: corsAllowedOrigins(),
		Auth:               authCfg,
	}, nil
}

func dbURL() string {
	if v := os.Getenv("DB_URL"); v != "" {
		return v
	}
	// Credencial de desarrollo local por defecto; se sobreescribe con DB_URL en cualquier otro entorno.
	//nolint:gosec // dev-only default, not a real secret
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
