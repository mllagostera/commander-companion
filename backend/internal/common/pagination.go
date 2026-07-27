package common

import (
	"encoding/base64"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

const (
	// DefaultPageLimit es el tamaño de página cuando el cliente no pide uno.
	DefaultPageLimit = 20
	// MaxPageLimit acota lo que puede pedir un cliente; por encima se recorta en
	// silencio en vez de dar error, para no romper clientes que pidan de más.
	MaxPageLimit = 100

	cursorSeparator = "|"
)

// ErrInvalidCursor indica que el cursor recibido no es uno emitido por esta API
// (mal formado, truncado o manipulado).
var ErrInvalidCursor = InvalidInput("invalid cursor")

// ErrInvalidLimit indica que el parámetro limit no es un entero positivo.
var ErrInvalidLimit = InvalidInput("limit must be a positive integer")

// Cursor es la posición de la última fila de una página. La paginación es keyset
// sobre (created_at, id) DESC: a diferencia de OFFSET, no se saltea ni repite
// filas cuando se insertan registros nuevos mientras se pagina, y el coste no
// crece con la profundidad de la página. Se incluye el id además del created_at
// porque created_at no es único (dos filas creadas en el mismo microsegundo
// desempatan por id).
type Cursor struct {
	CreatedAt time.Time
	ID        string
}

// PageRequest son los parámetros de paginación ya validados de una request.
// Cursor vacío significa "primera página".
type PageRequest struct {
	Cursor string
	Limit  int32
}

// EncodeCursor serializa un cursor a un string opaco para el cliente. El formato
// interno (base64url de "<created_at>|<id>") es un detalle de implementación: los
// clientes deben tratarlo como opaco y devolverlo tal cual.
func EncodeCursor(c Cursor) string {
	raw := c.CreatedAt.UTC().Format(time.RFC3339Nano) + cursorSeparator + c.ID
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

// DecodeCursor deshace EncodeCursor. Devuelve ErrInvalidCursor ante cualquier
// entrada que no haya salido de esta API.
func DecodeCursor(encoded string) (Cursor, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return Cursor{}, ErrInvalidCursor
	}

	createdAt, id, found := strings.Cut(string(decoded), cursorSeparator)
	if !found || id == "" {
		return Cursor{}, ErrInvalidCursor
	}

	parsed, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return Cursor{}, ErrInvalidCursor
	}
	return Cursor{CreatedAt: parsed, ID: id}, nil
}

// ParsePageRequest lee los query params `cursor` y `limit` de una request.
func ParsePageRequest(c *fiber.Ctx) (PageRequest, error) {
	limit, err := parsePageLimit(c.Query("limit"))
	if err != nil {
		return PageRequest{}, err
	}
	return PageRequest{Cursor: c.Query("cursor"), Limit: limit}, nil
}

// parsePageLimit valida el query param `limit`: vacío usa el default y cualquier
// valor por encima del máximo se recorta.
func parsePageLimit(raw string) (int32, error) {
	if raw == "" {
		return DefaultPageLimit, nil
	}

	limit, err := strconv.Atoi(raw)
	if err != nil || limit < 1 {
		return 0, ErrInvalidLimit
	}
	if limit > MaxPageLimit {
		return MaxPageLimit, nil
	}
	//nolint:gosec // acotado a [1, MaxPageLimit] justo arriba
	return int32(limit), nil
}
