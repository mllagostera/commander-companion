package common

import (
	"errors"
	"log"

	"github.com/gofiber/fiber/v2"
)

// Kinds de error de dominio. Los servicios NO importan fiber: devuelven errores
// de negocio construidos con los helpers de abajo (o sentinelas propios del
// módulo que los envuelven), y el transporte HTTP los traduce a un status code
// con MapError. Así el mapeo error-de-dominio → HTTP vive en un solo lugar y
// todos los módulos devuelven los mismos códigos para los mismos casos.
var (
	// ErrInvalidInput agrupa los errores de validación de entrada → HTTP 400.
	ErrInvalidInput = errors.New("invalid input")
	// ErrUnauthorized agrupa los errores de autenticación → HTTP 401.
	ErrUnauthorized = errors.New("unauthorized")
	// ErrNotFound agrupa los errores de recurso inexistente o ajeno → HTTP 404.
	ErrNotFound = errors.New("not found")
	// ErrConflict agrupa los errores de estado incompatible con la operación → HTTP 409.
	ErrConflict = errors.New("conflict")
	// ErrForbidden agrupa los errores de acción no permitida para un sujeto ya
	// autenticado/identificado (distinto de ErrUnauthorized: acá se sabe quién es, pero
	// no puede hacer esto todavía) → HTTP 403.
	ErrForbidden = errors.New("forbidden")
	// ErrNotImplemented agrupa lo no configurado o no disponible → HTTP 501.
	ErrNotImplemented = errors.New("not implemented")
	// ErrUpstreamUnavailable agrupa los fallos de una dependencia externa (p. ej.
	// Moxfield) tras agotar los reintentos → HTTP 503.
	ErrUpstreamUnavailable = errors.New("upstream unavailable")
)

// ErrInvalidUser indica que el ID de usuario propagado por el middleware de auth
// no es un UUID válido (token corrupto o emitido contra otro esquema). Es común a
// todos los módulos que leen common.UserIDKey, por eso vive acá.
var ErrInvalidUser = Unauthorized("invalid user")

// DomainError es un error de negocio: lleva su propio mensaje (el que ve el
// cliente) y un kind que determina a qué status HTTP se traduce. Implementa
// Unwrap, así que errors.Is(err, common.ErrNotFound) y errors.Is(err, ErrDeckNotFound)
// funcionan sobre el mismo valor.
type DomainError struct {
	kind error
	msg  string
}

// Error implementa la interfaz error devolviendo el mensaje de negocio.
func (e *DomainError) Error() string { return e.msg }

// Unwrap expone el kind del error, para que errors.Is lo compare contra los
// sentinelas de arriba.
func (e *DomainError) Unwrap() error { return e.kind }

// InvalidInput crea un error de validación de entrada (→ HTTP 400).
func InvalidInput(msg string) *DomainError { return &DomainError{kind: ErrInvalidInput, msg: msg} }

// Unauthorized crea un error de autenticación (→ HTTP 401).
func Unauthorized(msg string) *DomainError { return &DomainError{kind: ErrUnauthorized, msg: msg} }

// NotFound crea un error de recurso inexistente o ajeno al usuario (→ HTTP 404).
func NotFound(msg string) *DomainError { return &DomainError{kind: ErrNotFound, msg: msg} }

// Conflict crea un error de estado incompatible con la operación pedida (→ HTTP 409).
func Conflict(msg string) *DomainError { return &DomainError{kind: ErrConflict, msg: msg} }

// Forbidden crea un error de acción no permitida para un sujeto identificado (→ HTTP 403).
func Forbidden(msg string) *DomainError { return &DomainError{kind: ErrForbidden, msg: msg} }

// NotImplemented crea un error de funcionalidad no configurada o no disponible (→ HTTP 501).
func NotImplemented(msg string) *DomainError {
	return &DomainError{kind: ErrNotImplemented, msg: msg}
}

// UpstreamUnavailable crea un error de dependencia externa no disponible tras
// agotar los reintentos (→ HTTP 503).
func UpstreamUnavailable(msg string) *DomainError {
	return &DomainError{kind: ErrUpstreamUnavailable, msg: msg}
}

// MapError traduce un error de dominio al *fiber.Error con el status HTTP que le
// corresponde. Los errores que ya son *fiber.Error (los que produce el propio
// transporte, p. ej. un body mal formado) y los inesperados —que el ErrorHandler
// global convierte en 500— pasan tal cual, así que aplicarla dos veces sobre el
// mismo error es inocuo.
//
// Recorre una tabla en vez de un switch/case por kind: con 7 kinds, el switch supera
// el límite de complejidad ciclomática del linter (cyclop) sin aportar nada — es el
// mismo mapeo 1 a 1 kind → status en cualquiera de las dos formas.
func MapError(err error) error {
	if err == nil {
		return nil
	}

	var fiberErr *fiber.Error
	if errors.As(err, &fiberErr) {
		return err
	}

	statusByKind := []struct {
		kind   error
		status int
	}{
		{ErrInvalidInput, fiber.StatusBadRequest},
		{ErrUnauthorized, fiber.StatusUnauthorized},
		{ErrNotFound, fiber.StatusNotFound},
		{ErrConflict, fiber.StatusConflict},
		{ErrForbidden, fiber.StatusForbidden},
		{ErrNotImplemented, fiber.StatusNotImplemented},
		{ErrUpstreamUnavailable, fiber.StatusServiceUnavailable},
	}

	for _, m := range statusByKind {
		if errors.Is(err, m.kind) {
			return fiber.NewError(m.status, err.Error())
		}
	}

	return err
}

// ErrorResponse representa la estructura estándar de errores.
type ErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ErrorHandler es el middleware global para manejar errores en Fiber. Aplica el
// mismo mapeo dominio → HTTP que MapError, para que un error de negocio que
// llegue sin haber pasado por el handler no se degrade a un 500.
func ErrorHandler(c *fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	message := "Internal Server Error"

	var fiberErr *fiber.Error
	switch {
	case errors.As(MapError(err), &fiberErr):
		code = fiberErr.Code
		message = fiberErr.Message
	case err != nil:
		// Error no mapeado a un DomainError: no se filtra el detalle interno al
		// cliente (podría ser un error de driver de BD, de una dependencia
		// externa, etc.), pero sí se loguea del lado servidor para no perder
		// visibilidad.
		log.Printf("error no controlado: %v", err)
	}

	return c.Status(code).JSON(ErrorResponse{
		Code:    code,
		Message: message,
	})
}
