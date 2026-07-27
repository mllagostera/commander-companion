package playgroups

import (
	"github.com/gofiber/fiber/v2"

	"github.com/usuario/commander-companion-backend/internal/common"
)

// Handler contiene las dependencias del transporte HTTP para playgroups.
type Handler struct {
	svc Service
}

// NewHandler inicializa y retorna un nuevo Handler.
func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes registra todos los endpoints del módulo playgroups.
func (h *Handler) RegisterRoutes(router fiber.Router) {
	router.Get("/playgroups", h.ListPlaygroups)
	router.Post("/playgroups", h.CreatePlaygroup)
	router.Get("/playgroups/:id", h.GetPlaygroup)
	router.Post("/playgroups/:id/members", h.AddMember)
}

// CreatePlaygroup maneja la creación de un nuevo grupo de juego.
func (h *Handler) CreatePlaygroup(c *fiber.Ctx) error {
	var req CreatePlaygroupRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	userID, _ := c.Locals(common.UserIDKey).(string)
	res, err := h.svc.CreatePlaygroup(c.Context(), userID, req)
	if err != nil {
		return common.MapError(err)
	}
	return c.Status(fiber.StatusCreated).JSON(res)
}

// ListPlaygroups devuelve los grupos del usuario autenticado.
func (h *Handler) ListPlaygroups(c *fiber.Ctx) error {
	userID, _ := c.Locals(common.UserIDKey).(string)
	res, err := h.svc.ListPlaygroups(c.Context(), userID)
	if err != nil {
		return common.MapError(err)
	}
	return c.JSON(res)
}

// GetPlaygroup devuelve el detalle de un grupo, si el usuario autenticado es miembro.
func (h *Handler) GetPlaygroup(c *fiber.Ctx) error {
	userID, _ := c.Locals(common.UserIDKey).(string)
	res, err := h.svc.GetPlaygroup(c.Context(), userID, c.Params("id"))
	if err != nil {
		return common.MapError(err)
	}
	return c.JSON(res)
}

// AddMember añade un miembro al grupo de juego indicado.
func (h *Handler) AddMember(c *fiber.Ctx) error {
	var req AddMemberRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	userID, _ := c.Locals(common.UserIDKey).(string)
	res, err := h.svc.AddMember(c.Context(), c.Params("id"), userID, req)
	if err != nil {
		return common.MapError(err)
	}
	return c.Status(fiber.StatusCreated).JSON(res)
}
