package common_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/usuario/commander-companion-backend/internal/common"
)

// errConnRefused stands in for an unmapped infrastructure error (e.g. from the DB driver).
var errConnRefused = errors.New("connection refused")

func TestIsUnexpected(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"domain error", common.NotFound("deck not found"), false},
		{"wrapped domain error", fmt.Errorf("loading deck: %w", common.Conflict("already exists")), false},
		{"fiber error", fiber.NewError(fiber.StatusBadRequest, "bad body"), false},
		{"upstream unavailable", common.UpstreamUnavailable("moxfield down"), false},
		{"plain error", errConnRefused, true},
		{"wrapped plain error", fmt.Errorf("querying decks: %w", errConnRefused), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := common.IsUnexpected(tt.err); got != tt.want {
				t.Fatalf("IsUnexpected(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}
