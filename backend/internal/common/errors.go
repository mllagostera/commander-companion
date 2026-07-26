package common

import (
	"errors"

	"github.com/gofiber/fiber/v2"
)

// ErrorResponse representa la estructura estándar de errores.
type ErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ErrorHandler es el middleware global para manejar errores en Fiber.
func ErrorHandler(c *fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	message := "Internal Server Error"

	// Si es un error de Fiber, extraer código y mensaje
	var fiberErr *fiber.Error
	if errors.As(err, &fiberErr) {
		code = fiberErr.Code
		message = fiberErr.Message
	} else if err != nil {
		// Loguear el error real internamente
		// log.Printf("Error no controlado: %v", err)
		message = err.Error() // Para desarrollo, mostramos el error
	}

	return c.Status(code).JSON(ErrorResponse{
		Code:    code,
		Message: message,
	})
}
