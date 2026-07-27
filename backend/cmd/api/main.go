package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"

	"github.com/usuario/commander-companion-backend/internal/auth"
	"github.com/usuario/commander-companion-backend/internal/common"
	"github.com/usuario/commander-companion-backend/internal/decks"
	gameactions "github.com/usuario/commander-companion-backend/internal/game-actions"
	"github.com/usuario/commander-companion-backend/internal/games"
	"github.com/usuario/commander-companion-backend/internal/moxfield"
	"github.com/usuario/commander-companion-backend/internal/playgroups"
	"github.com/usuario/commander-companion-backend/internal/statistics"
	"github.com/usuario/commander-companion-backend/internal/sync"
	"github.com/usuario/commander-companion-backend/internal/users"
	"github.com/usuario/commander-companion-backend/internal/websocket"
)

const (
	defaultAccessTokenTTL  = 15 * time.Minute
	defaultRefreshTokenTTL = 30 * 24 * time.Hour
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	// 1. Conexión a BD
	dbURL := os.Getenv("DB_URL")
	if dbURL == "" {
		// Credencial de desarrollo local por defecto; se sobreescribe con DB_URL en cualquier otro entorno.
		//nolint:gosec // dev-only default, not a real secret
		dbURL = "postgres://postgres:postgres@localhost:5432/commander?sslmode=disable"
	}

	db, err := common.NewDB(dbURL)
	if err != nil {
		return fmt.Errorf("no se pudo conectar a la base de datos: %w", err)
	}
	defer db.Close()
	log.Println("Conectado a PostgreSQL exitosamente.")

	authCfg, err := loadAuthConfig()
	if err != nil {
		return err
	}

	// 2. Inicializar Fiber
	app := fiber.New(fiber.Config{
		ErrorHandler: common.ErrorHandler,
		AppName:      "Commander Companion API v0.1",
	})

	// Middlewares
	app.Use(logger.New())
	app.Use(recover.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins: corsAllowedOrigins(),
		AllowHeaders: "Origin, Content-Type, Accept, Authorization",
		AllowMethods: "GET,POST,PUT,PATCH,DELETE,OPTIONS",
	}))

	registerModules(app, db, authCfg)

	// 4. Arrancar Servidor
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	//nolint:gosec // port viene de una env var del operador, no de un usuario final
	log.Printf("Iniciando servidor en el puerto %s...", port)
	if err := app.Listen(":" + port); err != nil {
		return fmt.Errorf("error al arrancar el servidor: %w", err)
	}
	return nil
}

// loadAuthConfig lee la configuración de auth desde variables de entorno.
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

// corsAllowedOrigins lee los orígenes permitidos para CORS. Por defecto, en
// desarrollo se permite cualquier origen (no se usan cookies/credentials, solo
// Bearer tokens); en cualquier otro entorno hay que restringirlo explícitamente.
func corsAllowedOrigins() string {
	origins := os.Getenv("CORS_ALLOWED_ORIGINS")
	if origins == "" {
		return "*"
	}
	return origins
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

// registerModules instancia repositorios, servicios y handlers, y registra las
// rutas de todos los módulos bajo /api/v1 (públicas y protegidas por JWT).
func registerModules(app *fiber.App, db *common.DB, authCfg auth.Config) {
	api := app.Group("/api/v1")

	usersService := users.NewService(db.Pool)
	usersHandler := users.NewHandler(usersService)
	usersHandler.RegisterRoutes(api) // POST /auth/register

	authService := auth.NewService(db.Pool, usersService, authCfg)
	authHandler := auth.NewHandler(authService)
	authHandler.RegisterPublicRoutes(api) // login, google, refresh, logout

	protected := api.Group("", auth.RequireAuth(authCfg.JWTSecret))
	authHandler.RegisterProtectedRoutes(protected) // GET /auth/me

	decksService := decks.NewService(db.Pool, moxfield.NewClient())
	decks.NewHandler(decksService).RegisterRoutes(protected)

	playgroupsService := playgroups.NewService(db.Pool)
	playgroups.NewHandler(playgroupsService).RegisterRoutes(protected)

	statisticsService := statistics.NewService(db.Pool)
	statistics.NewHandler(statisticsService).RegisterRoutes(protected)

	// wsHub retransmite en vivo los game_actions de una partida a todos los clientes
	// conectados a ella (ver ADR-0005). Se inyecta en games/game-actions como
	// Broadcaster, sin que esos paquetes dependan de internal/websocket (mismo patrón
	// que statisticsService como StatisticsRecalculator). La ruta de WebSocket es
	// pública (sin auth.RequireAuth): autentica por mensaje inicial, no por header.
	wsHub := websocket.NewHub()
	websocket.RegisterRoutes(api, wsHub, authCfg.JWTSecret)

	gamesService := games.NewService(db.Pool, statisticsService, wsHub)
	games.NewHandler(gamesService).RegisterRoutes(protected)

	gameActionsService := gameactions.NewService(db.Pool, wsHub)
	gameactions.NewHandler(gameActionsService).RegisterRoutes(protected)

	syncService := sync.NewService(db.Pool)
	sync.NewHandler(syncService).RegisterRoutes(protected)
}
