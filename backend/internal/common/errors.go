package common

import (
	"errors"
	"log"

	"github.com/gofiber/fiber/v2"
)

// Domain error kinds. Services do NOT import fiber: they return business
// errors built with the helpers below (or their own module-specific
// sentinels that wrap them), and the HTTP transport translates them to a
// status code with MapError. This way the domain-error → HTTP mapping lives
// in a single place and every module returns the same codes for the same
// cases.
var (
	// ErrInvalidInput groups input validation errors → HTTP 400.
	ErrInvalidInput = errors.New("invalid input")
	// ErrUnauthorized groups authentication errors → HTTP 401.
	ErrUnauthorized = errors.New("unauthorized")
	// ErrNotFound groups errors for a nonexistent or foreign resource → HTTP 404.
	ErrNotFound = errors.New("not found")
	// ErrConflict groups errors for a state incompatible with the operation → HTTP 409.
	ErrConflict = errors.New("conflict")
	// ErrForbidden groups errors for an action not allowed for an already
	// authenticated/identified subject (unlike ErrUnauthorized: here we know
	// who they are, but they can't do this yet) → HTTP 403.
	ErrForbidden = errors.New("forbidden")
	// ErrNotImplemented groups what's not configured or not available → HTTP 501.
	ErrNotImplemented = errors.New("not implemented")
	// ErrUpstreamUnavailable groups failures of an external dependency (e.g.
	// Moxfield) after exhausting retries → HTTP 503.
	ErrUpstreamUnavailable = errors.New("upstream unavailable")
)

// ErrInvalidUser indicates that the user ID propagated by the auth
// middleware isn't a valid UUID (corrupt token or one issued against
// another schema). It's common to all modules that read common.UserIDKey,
// which is why it lives here.
var ErrInvalidUser = Unauthorized("invalid user")

// DomainError is a business error: it carries its own message (the one the
// client sees) and a kind that determines which HTTP status it translates
// to. It implements Unwrap, so errors.Is(err, common.ErrNotFound) and
// errors.Is(err, ErrDeckNotFound) both work on the same value.
type DomainError struct {
	kind error
	msg  string
}

// Error implements the error interface, returning the business message.
func (e *DomainError) Error() string { return e.msg }

// Unwrap exposes the error's kind, so errors.Is can compare it against the
// sentinels above.
func (e *DomainError) Unwrap() error { return e.kind }

// InvalidInput creates an input validation error (→ HTTP 400).
func InvalidInput(msg string) *DomainError { return &DomainError{kind: ErrInvalidInput, msg: msg} }

// Unauthorized creates an authentication error (→ HTTP 401).
func Unauthorized(msg string) *DomainError { return &DomainError{kind: ErrUnauthorized, msg: msg} }

// NotFound creates an error for a resource that doesn't exist or doesn't belong to the user (→ HTTP 404).
func NotFound(msg string) *DomainError { return &DomainError{kind: ErrNotFound, msg: msg} }

// Conflict creates an error for a state incompatible with the requested operation (→ HTTP 409).
func Conflict(msg string) *DomainError { return &DomainError{kind: ErrConflict, msg: msg} }

// Forbidden creates an error for an action not allowed for an identified subject (→ HTTP 403).
func Forbidden(msg string) *DomainError { return &DomainError{kind: ErrForbidden, msg: msg} }

// NotImplemented creates an error for functionality not configured or not available (→ HTTP 501).
func NotImplemented(msg string) *DomainError {
	return &DomainError{kind: ErrNotImplemented, msg: msg}
}

// UpstreamUnavailable creates an error for an external dependency unavailable
// after exhausting retries (→ HTTP 503).
func UpstreamUnavailable(msg string) *DomainError {
	return &DomainError{kind: ErrUpstreamUnavailable, msg: msg}
}

// MapError translates a domain error to the *fiber.Error with the
// corresponding HTTP status. Errors that are already *fiber.Error (the ones
// produced by the transport itself, e.g. a malformed body) and unexpected
// ones —which the global ErrorHandler turns into a 500— pass through
// unchanged, so applying it twice to the same error is harmless.
//
// It walks a table instead of a switch/case per kind: with 7 kinds, the
// switch exceeds the linter's (cyclop) cyclomatic complexity limit without
// adding anything — it's the same 1-to-1 kind → status mapping either way.
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

// ErrorResponse represents the standard error structure.
type ErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ErrorHandler is the global middleware for handling errors in Fiber. It
// applies the same domain → HTTP mapping as MapError, so a business error
// that arrives without going through the handler doesn't degrade to a 500.
func ErrorHandler(c *fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	message := "Internal Server Error"

	var fiberErr *fiber.Error
	switch {
	case errors.As(MapError(err), &fiberErr):
		code = fiberErr.Code
		message = fiberErr.Message
	case err != nil:
		// Error not mapped to a DomainError: the internal detail isn't leaked
		// to the client (it could be a DB driver error, an external
		// dependency error, etc.), but it is logged server-side so as not to
		// lose visibility.
		log.Printf("error no controlado: %v", err)
	}

	return c.Status(code).JSON(ErrorResponse{
		Code:    code,
		Message: message,
	})
}
