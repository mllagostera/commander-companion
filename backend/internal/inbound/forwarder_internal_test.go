package inbound

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const rawMessage = "From: Ana <ana@example.com>\r\nSubject: Borrar mi cuenta\r\n\r\nHola, quiero borrar mi cuenta.\r\n"

// fakeResend serves the three calls a forward makes and records the send.
type fakeResend struct {
	srv          *httptest.Server
	receivedJSON string
	rawStatus    int
	sendStatus   int
	sent         sendRequest
	sentHeaders  http.Header
	sendCalls    int
}

func newFakeResend(t *testing.T) *fakeResend {
	t.Helper()
	f := &fakeResend{rawStatus: http.StatusOK, sendStatus: http.StatusOK}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/emails/receiving/em_123":
			if r.Header.Get("Authorization") != "Bearer re_full" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			_, _ = w.Write([]byte(strings.ReplaceAll(f.receivedJSON, "RAW_URL", f.srv.URL+"/raw/em_123")))
		case r.Method == http.MethodGet && r.URL.Path == "/raw/em_123":
			w.WriteHeader(f.rawStatus)
			_, _ = w.Write([]byte(rawMessage))
		case r.Method == http.MethodPost && r.URL.Path == "/emails":
			f.sendCalls++
			f.sentHeaders = r.Header.Clone()
			if err := json.NewDecoder(r.Body).Decode(&f.sent); err != nil {
				t.Errorf("decoding send body: %v", err)
			}
			w.WriteHeader(f.sendStatus)
			_, _ = w.Write([]byte(`{"id":"sent_1"}`))
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeResend) forwarder() *resendForwarder {
	return &resendForwarder{
		cfg: Config{
			APIKey:      "re_full",
			ForwardTo:   "me@gmail.example",
			ForwardFrom: "TapeandoCartones <privacidad@tapeandocartones.es>",
		},
		baseURL:    f.srv.URL,
		httpClient: f.srv.Client(),
	}
}

const receivedWithHTML = `{
  "object": "email",
  "id": "em_123",
  "from": "ana@example.com",
  "to": ["privacidad@tapeandocartones.es"],
  "subject": "Borrar mi cuenta",
  "created_at": "2026-10-18T10:00:00.000Z",
  "html": "<p>Hola, quiero borrar mi cuenta.</p>",
  "text": "Hola, quiero borrar mi cuenta.",
  "headers": {"from": "Ana <script>x</script> <ana@example.com>"},
  "authentication": {"spf": "pass", "dkim": "pass", "dmarc": "pass"},
  "raw": {"download_url": "RAW_URL", "expires_at": "2026-10-18T11:00:00.000Z"}
}`

func TestForwardSendsCopyWithOriginalAttached(t *testing.T) {
	f := newFakeResend(t)
	f.receivedJSON = receivedWithHTML

	if err := f.forwarder().Forward(context.Background(), "em_123"); err != nil {
		t.Fatalf("Forward() error = %v", err)
	}

	assertEnvelope(t, &f.sent)
	assertBody(t, &f.sent)
	assertOriginalAttached(t, &f.sent)
	if key := f.sentHeaders.Get("Idempotency-Key"); key != "inbound-forward/em_123" {
		t.Errorf("Idempotency-Key = %q", key)
	}
}

func assertEnvelope(t *testing.T, got *sendRequest) {
	t.Helper()
	if got.From != "TapeandoCartones <privacidad@tapeandocartones.es>" {
		t.Errorf("From = %q", got.From)
	}
	if len(got.To) != 1 || got.To[0] != "me@gmail.example" {
		t.Errorf("To = %v", got.To)
	}
	if got.ReplyTo != "ana@example.com" {
		t.Errorf("ReplyTo = %q, want the original sender", got.ReplyTo)
	}
	if got.Subject != "Fwd: Borrar mi cuenta" {
		t.Errorf("Subject = %q", got.Subject)
	}
}

func assertBody(t *testing.T, got *sendRequest) {
	t.Helper()
	if !strings.HasSuffix(got.HTML, "<p>Hola, quiero borrar mi cuenta.</p>") {
		t.Errorf("HTML does not end with the original body: %q", got.HTML)
	}
	if strings.Contains(got.HTML, "<script>") {
		t.Errorf("header values were not escaped: %q", got.HTML)
	}
	if !strings.Contains(got.HTML, "SPF pass, DKIM pass, DMARC pass") {
		t.Errorf("authentication results missing from %q", got.HTML)
	}
	if !strings.Contains(got.Text, "Para: privacidad@tapeandocartones.es") {
		t.Errorf("recipient missing from text: %q", got.Text)
	}
}

func assertOriginalAttached(t *testing.T, got *sendRequest) {
	t.Helper()
	if len(got.Attachments) != 1 {
		t.Fatalf("Attachments = %d, want the original .eml", len(got.Attachments))
	}
	att := got.Attachments[0]
	decoded, err := base64.StdEncoding.DecodeString(att.Content)
	if err != nil || string(decoded) != rawMessage {
		t.Errorf("attachment content = %q (err %v), want the raw message", decoded, err)
	}
	if att.ContentType != "message/rfc822" {
		t.Errorf("attachment content type = %q", att.ContentType)
	}
}

func TestForwardTextOnlyEmailIsEscapedIntoHTML(t *testing.T) {
	f := newFakeResend(t)
	f.receivedJSON = `{"from":"bob@example.com","to":["hola@tapeandocartones.es"],"subject":"",
		"html":null,"text":"1 < 2 & <b>no</b>","raw":{"download_url":"RAW_URL"}}`

	if err := f.forwarder().Forward(context.Background(), "em_123"); err != nil {
		t.Fatalf("Forward() error = %v", err)
	}
	if f.sent.Subject != "Fwd: (sin asunto)" {
		t.Errorf("Subject = %q", f.sent.Subject)
	}
	if !strings.Contains(f.sent.HTML, "1 &lt; 2 &amp; &lt;b&gt;no&lt;/b&gt;") {
		t.Errorf("text body not escaped into the HTML part: %q", f.sent.HTML)
	}
	if !strings.HasSuffix(f.sent.Text, "1 < 2 & <b>no</b>") {
		t.Errorf("text part = %q", f.sent.Text)
	}
}

func TestForwardWithoutRawStillSendsWithNote(t *testing.T) {
	f := newFakeResend(t)
	f.receivedJSON = `{"from":"bob@example.com","to":["hola@tapeandocartones.es"],"subject":"Hola",
		"text":"cuerpo","raw":null}`

	if err := f.forwarder().Forward(context.Background(), "em_123"); err != nil {
		t.Fatalf("Forward() error = %v", err)
	}
	if len(f.sent.Attachments) != 0 {
		t.Errorf("Attachments = %d, want none", len(f.sent.Attachments))
	}
	if !strings.Contains(f.sent.Text, "panel de Resend") {
		t.Errorf("missing-original note absent from %q", f.sent.Text)
	}
}

func TestForwardReportsResendErrors(t *testing.T) {
	t.Run("send rejected", func(t *testing.T) {
		f := newFakeResend(t)
		f.receivedJSON = receivedWithHTML
		f.sendStatus = http.StatusUnprocessableEntity

		err := f.forwarder().Forward(context.Background(), "em_123")
		if !errors.Is(err, ErrResendRequestFailed) {
			t.Fatalf("Forward() error = %v, want ErrResendRequestFailed", err)
		}
	})

	t.Run("raw download failed", func(t *testing.T) {
		f := newFakeResend(t)
		f.receivedJSON = receivedWithHTML
		f.rawStatus = http.StatusForbidden

		err := f.forwarder().Forward(context.Background(), "em_123")
		if !errors.Is(err, ErrResendRequestFailed) {
			t.Fatalf("Forward() error = %v, want ErrResendRequestFailed", err)
		}
		if f.sendCalls != 0 {
			t.Errorf("sent %d copies after a failed download, want 0", f.sendCalls)
		}
	})

	t.Run("sending-only key cannot read", func(t *testing.T) {
		f := newFakeResend(t)
		f.receivedJSON = receivedWithHTML
		fw := f.forwarder()
		fw.cfg.APIKey = "re_sending_only"

		err := fw.Forward(context.Background(), "em_123")
		if !errors.Is(err, ErrResendRequestFailed) {
			t.Fatalf("Forward() error = %v, want ErrResendRequestFailed", err)
		}
	})
}
