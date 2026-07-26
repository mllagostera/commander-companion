package main

import (
	"fmt"
	"log"
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"

	"github.com/usuario/commander-companion-backend/internal/common"
	"github.com/usuario/commander-companion-backend/internal/decks"
	gameactions "github.com/usuario/commander-companion-backend/internal/game-actions"
	"github.com/usuario/commander-companion-backend/internal/games"
	"github.com/usuario/commander-companion-backend/internal/playgroups"
	"github.com/usuario/commander-companion-backend/internal/statistics"
	"github.com/usuario/commander-companion-backend/internal/sync"
	"github.com/usuario/commander-companion-backend/internal/users"
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
		// Credencial de desarrollo local por defecto, no un secreto real; se sobreescribe con DB_URL en cualquier otro entorno.
		dbURL = "postgres://postgres:postgres@localhost:5432/commander?sslmode=disable" //nolint:gosec
	}

	db, err := common.NewDB(dbURL)
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

	// 3. Inicializar Módulos (Inyección de dependencias)
	api := app.Group("/api/v1")

	// Users Module
	usersService := users.NewService(db.Pool)
	usersHandler := users.NewHandler(usersService)
	usersHandler.RegisterRoutes(api)

	// Decks Module
	decksService := decks.NewService(db.Pool)
	decksHandler := decks.NewHandler(decksService)
	decksHandler.RegisterRoutes(api)

	// Playgroups Module
	playgroupsService := playgroups.NewService(db.Pool)
	playgroupsHandler := playgroups.NewHandler(playgroupsService)
	playgroupsHandler.RegisterRoutes(api)

	// Games Module
	gamesService := games.NewService(db.Pool)
	gamesHandler := games.NewHandler(gamesService)
	gamesHandler.RegisterRoutes(api)

	// Game Actions Module
	gameActionsService := gameactions.NewService(db.Pool)
	gameActionsHandler := gameactions.NewHandler(gameActionsService)
	gameActionsHandler.RegisterRoutes(api)

	// Statistics Module
	statisticsService := statistics.NewService(db.Pool)
	statisticsHandler := statistics.NewHandler(statisticsService)
	statisticsHandler.RegisterRoutes(api)

	// Sync Module
	syncService := sync.NewService(db.Pool)
	syncHandler := sync.NewHandler(syncService)
	syncHandler.RegisterRoutes(api)

	// 4. Arrancar Servidor
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	// El puerto proviene de una variable de entorno controlada por el operador del despliegue, no de un usuario final.
	log.Printf("Iniciando servidor en el puerto %s...", port) //nolint:gosec
	if err := app.Listen(":" + port); err != nil {
		return fmt.Errorf("error al arrancar el servidor: %w", err)
	}
	return nil
}
