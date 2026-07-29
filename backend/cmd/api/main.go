package main

import (
	"fmt"
	"log"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"

	"github.com/usuario/commander-companion-backend/internal/auth"
	"github.com/usuario/commander-companion-backend/internal/common"
	"github.com/usuario/commander-companion-backend/internal/config"
	"github.com/usuario/commander-companion-backend/internal/decks"
	"github.com/usuario/commander-companion-backend/internal/email"
	gameactions "github.com/usuario/commander-companion-backend/internal/game-actions"
	"github.com/usuario/commander-companion-backend/internal/games"
	"github.com/usuario/commander-companion-backend/internal/moxfield"
	"github.com/usuario/commander-companion-backend/internal/moxfieldimport"
	"github.com/usuario/commander-companion-backend/internal/playgroups"
	"github.com/usuario/commander-companion-backend/internal/statistics"
	"github.com/usuario/commander-companion-backend/internal/sync"
	"github.com/usuario/commander-companion-backend/internal/users"
	"github.com/usuario/commander-companion-backend/internal/websocket"
)

const (
	// Rate limit de los endpoints públicos de auth. 20 req/min por IP deja
	// holgura de sobra para un humano equivocándose de contraseña o para el
	// refresh automático de varios dispositivos detrás de un mismo NAT, pero
	// corta en seco el credential stuffing.
	authRateLimitMax    = 20
	authRateLimitWindow = time.Minute

	migrationsDir = "migrations"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	// 1. Cargar configuración y conectar a BD
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// Migraciones antes de abrir el pool de la app (ver common.RunMigrations):
	// deja el schema al día en cualquier entorno, incluidos los que no ofrecen
	// un hook de "release/pre-deploy command" separado.
	if err := common.RunMigrations(cfg.DBURL, migrationsDir); err != nil {
		return err
	}
	log.Println("Migraciones aplicadas correctamente.")

	db, err := common.NewDB(cfg.DBURL)
	if err != nil {
		return fmt.Errorf("no se pudo conectar a la base de datos: %w", err)
	}
	defer db.Close()
	log.Println("Conectado a PostgreSQL exitosamente.")

	// 2. Inicializar Fiber
	app := fiber.New(fiber.Config{
		ErrorHandler: common.ErrorHandler,
		AppName:      "Commander Companion API v0.1",
	})

	// Middlewares
	app.Use(logger.New())
	app.Use(recover.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins: cfg.CORSAllowedOrigins,
		AllowHeaders: "Origin, Content-Type, Accept, Authorization",
		AllowMethods: "GET,POST,PUT,PATCH,DELETE,OPTIONS",
	}))

	registerModules(app, db, &cfg)

	// 4. Arrancar Servidor
	log.Printf("Iniciando servidor en el puerto %s...", cfg.Port)
	if err := app.Listen(":" + cfg.Port); err != nil {
		return fmt.Errorf("error al arrancar el servidor: %w", err)
	}
	return nil
}

// newAuthRateLimiter construye el middleware de rate limiting por IP de los
// endpoints públicos de auth. El contador vive en memoria del proceso: alcanza
// para una instancia única (el despliegue actual, ver docker-compose.yml); con
// varias réplicas habría que moverlo a un Storage compartido (Redis).
//
// Nota: la IP sale de fiber.Ctx.IP(), que detrás de un proxy/ingress devuelve la
// del proxy salvo que se configure ProxyHeader en fiber.Config. Cuando se
// despliegue detrás de uno, hay que setearlo o el límite sería global.
func newAuthRateLimiter() fiber.Handler {
	return limiter.New(limiter.Config{
		Max:        authRateLimitMax,
		Expiration: authRateLimitWindow,
		LimitReached: func(_ *fiber.Ctx) error {
			// Devolver el error (en vez de escribir la respuesta acá) lo hace pasar
			// por common.ErrorHandler, así un 429 tiene el mismo cuerpo que
			// cualquier otro error de la API.
			return fiber.NewError(fiber.StatusTooManyRequests, "too many authentication requests, try again later")
		},
	})
}

// registerModules instancia repositorios, servicios y handlers, y registra las
// rutas de todos los módulos bajo /api/v1 (públicas y protegidas por JWT).
func registerModules(app *fiber.App, db *common.DB, cfg *config.Config) {
	api := app.Group("/api/v1")

	// El rate limit se pasa a cada endpoint público de auth en vez de montarlo como
	// middleware del grupo: en Fiber, un Group con middleware aplica a todo lo que
	// comparta el prefijo (incluido /auth/me y el resto de la API), y acá solo
	// queremos acotar los endpoints sin JWT.
	authRateLimit := newAuthRateLimiter()

	// emailClient manda el mail de verificación de cuenta. Sin RESEND_API_KEY (dev sin
	// cuenta de Resend) loguea el link por consola en vez de mandarlo (ver internal/email).
	emailClient := email.NewResendClient(cfg.Email)
	usersService := users.NewService(db.Pool, emailClient, cfg.WebAppURL, cfg.RequireEmailVerification)
	usersHandler := users.NewHandler(usersService)
	usersHandler.RegisterRoutes(api, authRateLimit) // POST /auth/register, verify-email, resend-verification

	authService := auth.NewService(db.Pool, usersService, cfg.Auth)
	authHandler := auth.NewHandler(authService)
	authHandler.RegisterPublicRoutes(api, authRateLimit) // login, google, refresh, logout

	protected := api.Group("", auth.RequireAuth(cfg.Auth.JWTSecret))
	authHandler.RegisterProtectedRoutes(protected)  // GET /auth/me
	usersHandler.RegisterProtectedRoutes(protected) // PATCH /users/:id

	moxfieldClient := moxfield.NewClient()
	decksService := decks.NewService(db.Pool, moxfieldClient)
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
	websocket.RegisterRoutes(api, wsHub, cfg.Auth.JWTSecret)

	gamesService := games.NewService(db.Pool, statisticsService, wsHub, playgroupsService)
	games.NewHandler(gamesService).RegisterRoutes(protected)

	gameActionsService := gameactions.NewService(db.Pool, wsHub)
	gameactions.NewHandler(gameActionsService).RegisterRoutes(protected)

	// sync no habla con la BD ni con Moxfield por su cuenta: delega en decks, que
	// es el dueño de la tabla y del cliente (ver internal/sync/service.go).
	syncService := sync.NewService(decksService)
	sync.NewHandler(syncService).RegisterRoutes(protected)

	// moxfieldimport: import masivo en background, scaffold completo pero con
	// ListDecksByUsername todavía stubbeado (ver internal/moxfieldimport).
	moxfieldImportService := moxfieldimport.NewService(db.Pool, usersService, decksService, moxfieldClient)
	moxfieldimport.NewHandler(moxfieldImportService).RegisterRoutes(protected)
}
