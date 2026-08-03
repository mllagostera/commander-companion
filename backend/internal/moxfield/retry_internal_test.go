package moxfield

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
)

const testDeckA = "deck-a"

func testClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return &Client{httpClient: srv.Client(), baseURL: srv.URL}
}

func validDeckBody() string {
	return `{
		"name": "Test Deck",
		"publicId": "abc123",
		"boards": {"commanders": {"cards": {"1": {"card": {"id": "card-1", "name": "Atraxa"}}}}},
		"main": {"id": "card-1", "name": "Atraxa"}
	}`
}

func TestGetDeck_RetriesOn500ThenSucceeds(t *testing.T) {
	var calls int32
	client := testClient(t, func(w http.ResponseWriter, _ *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(validDeckBody()))
	})

	deck, err := client.GetDeck(context.Background(), "abc123")
	if err != nil {
		t.Fatalf("GetDeck() unexpected error = %v", err)
	}
	if deck.Commander != "Atraxa" {
		t.Fatalf("GetDeck() commander = %q, want Atraxa", deck.Commander)
	}
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("calls = %d, want 2 (1 failure + 1 retry)", got)
	}
}

func TestGetDeck_404NeverRetries(t *testing.T) {
	var calls int32
	client := testClient(t, func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusNotFound)
	})

	_, err := client.GetDeck(context.Background(), "abc123")
	if !errors.Is(err, ErrDeckNotFound) {
		t.Fatalf("GetDeck() error = %v, want ErrDeckNotFound", err)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("calls = %d, want 1 (no retry on 404)", got)
	}
}

func TestGetDeck_ExhaustsRetriesAsUpstreamUnavailable(t *testing.T) {
	var calls int32
	client := testClient(t, func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusInternalServerError)
	})

	_, err := client.GetDeck(context.Background(), "abc123")
	if !errors.Is(err, ErrUpstreamUnavailable) {
		t.Fatalf("GetDeck() error = %v, want ErrUpstreamUnavailable", err)
	}
	if got := atomic.LoadInt32(&calls); got != maxAttempts {
		t.Fatalf("calls = %d, want %d (all attempts exhausted)", got, maxAttempts)
	}
}

func TestGetDeck_ImageURL_SingleFacedCard_UsesMainID(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(validDeckBody()))
	})

	deck, err := client.GetDeck(context.Background(), "abc123")
	if err != nil {
		t.Fatalf("GetDeck() unexpected error = %v", err)
	}
	want := "https://assets.moxfield.net/cards/card-card-1-art_crop.jpg"
	if deck.ImageURL != want {
		t.Fatalf("ImageURL = %q, want %q", deck.ImageURL, want)
	}
}

// Transform/MDFC cards: Moxfield only caches the art crop under each face's
// id, never under the combined id that main.id carries — see mainImageURL.
func TestGetDeck_ImageURL_TwoFacedCard_UsesFrontFaceID(t *testing.T) {
	body := `{
		"name": "Test Deck",
		"publicId": "abc123",
		"boards": {"commanders": {"cards": {"1": {"card": {"id": "combined-id", "name": "Front // Back"}}}}},
		"main": {
			"id": "combined-id",
			"name": "Front // Back",
			"card_faces": [
				{"id": "front-face-id", "name": "Front"},
				{"id": "back-face-id", "name": "Back"}
			]
		}
	}`
	client := testClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	})

	deck, err := client.GetDeck(context.Background(), "abc123")
	if err != nil {
		t.Fatalf("GetDeck() unexpected error = %v", err)
	}
	want := "https://assets.moxfield.net/cards/card-face-front-face-id-art_crop.jpg"
	if deck.ImageURL != want {
		t.Fatalf("ImageURL = %q, want %q (should use the front face id with the \"card-face-\" prefix, not main.id)",
			deck.ImageURL, want)
	}
}

func TestListDecksByUsername_SinglePage_ReturnsPublicIDs(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("authorUserNames"); got != "vansid" {
			t.Errorf("authorUserNames = %q, want vansid", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data": [{"publicId": "deck-a"}, {"publicId": "deck-b"}], "totalPages": 1}`))
	})

	got, err := client.ListDecksByUsername(context.Background(), "vansid")
	if err != nil {
		t.Fatalf("ListDecksByUsername() unexpected error = %v", err)
	}
	want := []string{testDeckA, "deck-b"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("ListDecksByUsername() = %v, want %v", got, want)
	}
}

func TestListDecksByUsername_PaginatesUntilLastPage(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Query().Get("pageNumber") {
		case "1":
			_, _ = w.Write([]byte(`{"data": [{"publicId": "deck-a"}], "totalPages": 2}`))
		case "2":
			_, _ = w.Write([]byte(`{"data": [{"publicId": "deck-b"}], "totalPages": 2}`))
		default:
			t.Fatalf("unexpected pageNumber %q", r.URL.Query().Get("pageNumber"))
		}
	})

	got, err := client.ListDecksByUsername(context.Background(), "vansid")
	if err != nil {
		t.Fatalf("ListDecksByUsername() unexpected error = %v", err)
	}
	want := []string{testDeckA, "deck-b"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("ListDecksByUsername() = %v, want %v", got, want)
	}
}

func TestListDecksByUsername_NoDecks_ReturnsEmpty(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data": [], "totalPages": 0}`))
	})

	got, err := client.ListDecksByUsername(context.Background(), "nobody")
	if err != nil {
		t.Fatalf("ListDecksByUsername() unexpected error = %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("ListDecksByUsername() = %v, want empty", got)
	}
}

func TestListDecksByUsername_RetriesOn500ThenSucceeds(t *testing.T) {
	var calls int32
	client := testClient(t, func(w http.ResponseWriter, _ *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data": [{"publicId": "deck-a"}], "totalPages": 1}`))
	})

	got, err := client.ListDecksByUsername(context.Background(), "vansid")
	if err != nil {
		t.Fatalf("ListDecksByUsername() unexpected error = %v", err)
	}
	if len(got) != 1 || got[0] != testDeckA {
		t.Fatalf("ListDecksByUsername() = %v, want [%s]", got, testDeckA)
	}
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("calls = %d, want 2 (1 failure + 1 retry)", got)
	}
}

func TestListDecksByUsername_ExhaustsRetriesAsUpstreamUnavailable(t *testing.T) {
	var calls int32
	client := testClient(t, func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusInternalServerError)
	})

	_, err := client.ListDecksByUsername(context.Background(), "vansid")
	if !errors.Is(err, ErrUpstreamUnavailable) {
		t.Fatalf("ListDecksByUsername() error = %v, want ErrUpstreamUnavailable", err)
	}
	if got := atomic.LoadInt32(&calls); got != maxAttempts {
		t.Fatalf("calls = %d, want %d (all attempts exhausted)", got, maxAttempts)
	}
}

func TestGetDeck_RespectsRetryAfterOn429(t *testing.T) {
	var calls int32
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			// Retry-After of 0s: we check that the header is respected (instead of
			// the fixed 200ms backoff) without unnecessarily lengthening the test.
			w.Header().Set("Retry-After", strconv.Itoa(0))
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(validDeckBody()))
	})

	deck, err := client.GetDeck(context.Background(), "abc123")
	if err != nil {
		t.Fatalf("GetDeck() unexpected error = %v", err)
	}
	if deck.Commander != "Atraxa" {
		t.Fatalf("GetDeck() commander = %q, want Atraxa", deck.Commander)
	}
}
