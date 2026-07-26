package moxfield_test

import (
	"errors"
	"testing"

	"github.com/usuario/commander-companion-backend/internal/moxfield"
)

const testPublicID = "abc123"

func TestExtractPublicID(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr error
	}{
		{name: "bare id", input: testPublicID, want: testPublicID},
		{name: "full url", input: "https://moxfield.com/decks/" + testPublicID, want: testPublicID},
		{
			name:  "full url with trailing slash",
			input: "https://www.moxfield.com/decks/" + testPublicID + "/",
			want:  testPublicID,
		},
		{name: "url with whitespace", input: "  https://moxfield.com/decks/" + testPublicID + "  ", want: testPublicID},
		{name: "empty input", input: "", wantErr: moxfield.ErrMissingIdentifier},
		{name: "whitespace only", input: "   ", wantErr: moxfield.ErrMissingIdentifier},
		{
			name:    "url without decks segment",
			input:   "https://moxfield.com/users/someone",
			wantErr: moxfield.ErrIDNotFoundInURL,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := moxfield.ExtractPublicID(tt.input)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("ExtractPublicID(%q) error = %v, want %v", tt.input, err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("ExtractPublicID(%q) unexpected error = %v", tt.input, err)
			}
			if got != tt.want {
				t.Fatalf("ExtractPublicID(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
