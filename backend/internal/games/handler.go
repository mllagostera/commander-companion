package games

import (
	"github.com/gofiber/fiber/v2"

	"github.com/usuario/commander-companion-backend/internal/common"
)

// Handler contiene las dependencias del transporte HTTP para games.
type Handler struct {
	svc Service
}

// NewHandler inicializa y retorna un nuevo Handler.
func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes registra todos los endpoints del módulo games.
func (h *Handler) RegisterRoutes(router fiber.Router) {
	router.Get("/games", h.ListGames)
	router.Post("/games", h.CreateGame)
	router.Get("/games/:id", h.GetGame)
	router.Post("/games/:id/join", h.JoinGame)
	router.Post("/games/:id/leave", h.LeaveGame)
	router.Post("/games/:id/start", h.StartGame)
	router.Post("/games/:id/finish", h.FinishGame)
}

// CreateGame maneja la creación de una nueva partida en estado pending.
func (h *Handler) CreateGame(c *fiber.Ctx) error {
	var req CreateGameRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	res, err := h.svc.CreateGame(c.Context(), req)
	if err != nil {
		return common.MapError(err)
	}
	return c.Status(fiber.StatusCreated).JSON(res)
}

// ListGames devuelve una página del historial de partidas. Acepta los query params
// `cursor` y `limit` (ver internal/common/pagination.go).
func (h *Handler) ListGames(c *fiber.Ctx) error {
	page, err := common.ParsePageRequest(c)
	if err != nil {
		return common.MapError(err)
	}

	res, err := h.svc.ListGames(c.Context(), page)
	if err != nil {
		return common.MapError(err)
	}
	return c.JSON(res)
}

// GetGame devuelve el detalle de una partida.
func (h *Handler) GetGame(c *fiber.Ctx) error {
	res, err := h.svc.GetGame(c.Context(), c.Params("id"))
	if err != nil {
		return common.MapError(err)
	}
	return c.JSON(res)
}

// JoinGame añade al usuario autenticado a una partida en estado pending.
func (h *Handler) JoinGame(c *fiber.Ctx) error {
	var req JoinGameRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	userID, _ := c.Locals(common.UserIDKey).(string)
	res, err := h.svc.JoinGame(c.Context(), c.Params("id"), userID, req)
	if err != nil {
		return common.MapError(err)
	}
	return c.JSON(res)
}

// LeaveGame remueve al usuario autenticado de una partida en estado pending.
func (h *Handler) LeaveGame(c *fiber.Ctx) error {
	userID, _ := c.Locals(common.UserIDKey).(string)
	if err := h.svc.LeaveGame(c.Context(), c.Params("id"), userID); err != nil {
		return common.MapError(err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// StartGame inicia una partida, pasándola a estado active.
func (h *Handler) StartGame(c *fiber.Ctx) error {
	res, err := h.svc.StartGame(c.Context(), c.Params("id"))
	if err != nil {
		return common.MapError(err)
	}
	return c.JSON(res)
}

// FinishGame finaliza una partida, pasándola a estado finished.
func (h *Handler) FinishGame(c *fiber.Ctx) error {
	res, err := h.svc.FinishGame(c.Context(), c.Params("id"))
	if err != nil {
		return common.MapError(err)
	}
	return c.JSON(res)
}
