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

func TestGetDeck_RespectsRetryAfterOn429(t *testing.T) {
	var calls int32
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			// Retry-After de 0s: comprobamos que se respeta el header (en vez del
			// backoff fijo de 200ms) sin alargar el test innecesariamente.
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
