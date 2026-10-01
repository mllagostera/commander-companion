package email_test

import (
	"testing"

	"github.com/usuario/commander-companion-backend/internal/email"
)

func TestNormalizeLocale(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"es", "es"},
		{"en", "en"},
		{"ca", "ca"},
		{"CA", "ca"},
		{"en-US", "en"},
		{"ca_ES", "ca"},
		{" en ", "en"},
		{"", email.DefaultLocale},
		{"fr", email.DefaultLocale},
		{"fr-CA", email.DefaultLocale},
	}
	for _, tt := range tests {
		if got := email.NormalizeLocale(tt.in); got != tt.want {
			t.Errorf("NormalizeLocale(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
