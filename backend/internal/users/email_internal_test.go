package users

import (
	"errors"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestNormalizeEmail(t *testing.T) {
	const plain = "user@example.com"
	valid := map[string]string{
		plain:                        plain,
		"  User@Example.COM  ":       plain,
		"first.last+tag@mail.co.uk":  "first.last+tag@mail.co.uk",
		"\tMixed.Case@Sub.Domain.io": "mixed.case@sub.domain.io",
	}
	for in, want := range valid {
		got, err := normalizeEmail(in)
		if err != nil || got != want {
			t.Errorf("normalizeEmail(%q) = %q, %v; want %q, nil", in, got, err, want)
		}
	}

	invalid := []string{
		"vansidgg",       // the incident: a bare username stored as the email
		"",               // empty
		"   ",            // whitespace only
		"user@",          // no domain
		"@example.com",   // no local part
		"user@localhost", // no dot in the domain
		"a b@example.com",
		"a@b@example.com",
		"Name <user@example.com>",
	}
	for _, in := range invalid {
		if got, err := normalizeEmail(in); !errors.Is(err, ErrInvalidEmail) {
			t.Errorf("normalizeEmail(%q) = %q, %v; want ErrInvalidEmail", in, got, err)
		}
	}
}

// The equalizer only hides which emails exist if it costs the same as a real comparison.
func TestTimingEqualizerHash_UsesDefaultCost(t *testing.T) {
	cost, err := bcrypt.Cost([]byte(timingEqualizerHash))
	if err != nil {
		t.Fatalf("timingEqualizerHash is not a bcrypt hash: %v", err)
	}
	if cost != bcrypt.DefaultCost {
		t.Fatalf("timingEqualizerHash cost = %d, want bcrypt.DefaultCost (%d)", cost, bcrypt.DefaultCost)
	}
}
