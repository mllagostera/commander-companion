package notify_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/usuario/commander-companion-backend/internal/notify"
)

func TestSlackNotifierPostsEscapedUsername(t *testing.T) {
	received := make(chan string, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var msg struct {
			Text string `json:"text"`
		}
		if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
			t.Errorf("decoding body: %v", err)
		}
		received <- msg.Text
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	notify.NewSlackNotifier(srv.URL).NotifySignup("<!channel> & co", notify.SignupMethodGoogle)

	select {
	case text := <-received:
		if !strings.Contains(text, "&lt;!channel&gt; &amp; co") {
			t.Errorf("username not escaped in %q", text)
		}
		if !strings.Contains(text, notify.SignupMethodGoogle) {
			t.Errorf("signup method missing from %q", text)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the webhook was never called")
	}
}

func TestSlackNotifierWithoutURLDoesNothing(t *testing.T) {
	// Must not panic nor try to reach any URL.
	notify.NewSlackNotifier("").NotifySignup("someone", notify.SignupMethodPassword)
}
