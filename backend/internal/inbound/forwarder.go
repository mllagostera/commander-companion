// Package inbound forwards the mail received on the site's domain (for example
// the privacy contact address the privacy policy gives) to a personal inbox.
//
// Resend receives the mail (an MX record on the domain points to it) and calls
// the webhook in handler.go with the email's metadata only. The Forwarder then
// fetches the full email from Resend's Receiving API and sends a copy through the
// regular Send Email API. There is no forwarding endpoint in Resend's REST API:
// the Node SDK's receiving.forward() does these same calls client-side.
//
// The copy carries the original HTML or text with a short header (sender,
// recipient, date, SPF/DKIM/DMARC results), reply-to set to the original sender
// so replying from the personal inbox answers them, and the untouched original
// as an .eml attachment, which keeps every attachment without parsing MIME here.
package inbound

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	defaultAPIBaseURL = "https://api.resend.com"
	httpTimeout       = 10 * time.Second
	// maxRawBytes caps the original message attached to the copy. Resend accepts
	// up to 40 MB of attachments per email after base64 (which adds a third), so
	// 25 MB of raw message still fits. Anything larger is forwarded without the
	// .eml, with a note pointing to the Resend dashboard, where it stays.
	maxRawBytes = 25 << 20
	// maxErrorBodyBytes is how much of a Resend error response goes into the error.
	maxErrorBodyBytes = 2048

	missingOriginalNote = "El original no se adjunta: no estaba disponible o pasa del tamaño máximo. " +
		"Sigue en el panel de Resend (Emails, Receiving)."
)

// ErrResendRequestFailed means one of the Resend calls answered with an error
// status (bad API key, unknown email ID, rejected send...).
var ErrResendRequestFailed = errors.New("resend request failed")

// Config configures the forwarding. The feature is off unless WebhookSecret and
// ForwardTo are both set (see internal/config).
type Config struct {
	// WebhookSecret is the "whsec_..." signing secret of the Resend webhook.
	WebhookSecret string
	// APIKey must be a Resend key with full access: reading received emails is
	// not allowed to sending-only keys.
	APIKey string
	// ForwardTo is the personal inbox that gets the copies.
	ForwardTo string
	// ForwardFrom is the sender of the copies, on a domain verified in Resend.
	ForwardFrom string
}

// Enabled reports whether forwarding is configured.
func (c Config) Enabled() bool {
	return c.WebhookSecret != "" && c.ForwardTo != ""
}

// Forwarder sends a copy of a received email to the configured inbox.
type Forwarder interface {
	Forward(ctx context.Context, emailID string) error
}

// NewResendForwarder builds the Forwarder backed by Resend's API.
func NewResendForwarder(cfg Config) Forwarder {
	return &resendForwarder{
		cfg:        cfg,
		baseURL:    defaultAPIBaseURL,
		httpClient: &http.Client{Timeout: httpTimeout},
	}
}

type resendForwarder struct {
	cfg        Config
	baseURL    string
	httpClient *http.Client
}

// receivedEmail is the part of GET /emails/receiving/{id} the copy needs.
type receivedEmail struct {
	From      string         `json:"from"`
	To        []string       `json:"to"`
	Subject   string         `json:"subject"`
	CreatedAt string         `json:"created_at"`
	HTML      *string        `json:"html"`
	Text      *string        `json:"text"`
	Headers   map[string]any `json:"headers"`
	Auth      *struct {
		SPF   string `json:"spf"`
		DKIM  string `json:"dkim"`
		DMARC string `json:"dmarc"`
	} `json:"authentication"`
	Raw *struct {
		DownloadURL string `json:"download_url"`
	} `json:"raw"`
}

type sendAttachment struct {
	Filename    string `json:"filename"`
	Content     string `json:"content"`
	ContentType string `json:"content_type"`
}

type sendRequest struct {
	From        string           `json:"from"`
	To          []string         `json:"to"`
	ReplyTo     string           `json:"reply_to,omitempty"`
	Subject     string           `json:"subject"`
	HTML        string           `json:"html"`
	Text        string           `json:"text"`
	Attachments []sendAttachment `json:"attachments,omitempty"`
}

// Forward fetches the received email and sends the copy. Sending uses the email
// ID as idempotency key, so a webhook Resend retries after the copy already went
// out (a timeout on our answer, a manual replay) does not send it twice.
func (f *resendForwarder) Forward(ctx context.Context, emailID string) error {
	email, err := f.fetch(ctx, emailID)
	if err != nil {
		return err
	}

	var raw []byte
	if email.Raw != nil && email.Raw.DownloadURL != "" {
		raw, err = f.download(ctx, email.Raw.DownloadURL)
		if err != nil {
			return err
		}
	}

	copyReq := buildCopy(email, raw, f.cfg)
	return f.send(ctx, emailID, &copyReq)
}

func (f *resendForwarder) fetch(ctx context.Context, emailID string) (*receivedEmail, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		f.baseURL+"/emails/receiving/"+url.PathEscape(emailID), http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("building receiving request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+f.cfg.APIKey)

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching received email: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if statusErr := checkStatus(resp); statusErr != nil {
		return nil, fmt.Errorf("fetching received email: %w", statusErr)
	}

	var email receivedEmail
	if decodeErr := json.NewDecoder(resp.Body).Decode(&email); decodeErr != nil {
		return nil, fmt.Errorf("decoding received email: %w", decodeErr)
	}
	return &email, nil
}

// download returns the raw message, or nil when it is larger than maxRawBytes
// (the copy then goes out without it rather than not at all).
func (f *resendForwarder) download(ctx context.Context, downloadURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("building raw download request: %w", err)
	}
	resp, err := f.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("downloading raw email: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if statusErr := checkStatus(resp); statusErr != nil {
		return nil, fmt.Errorf("downloading raw email: %w", statusErr)
	}

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxRawBytes+1))
	if err != nil {
		return nil, fmt.Errorf("reading raw email: %w", err)
	}
	if len(raw) > maxRawBytes {
		return nil, nil
	}
	return raw, nil
}

func (f *resendForwarder) send(ctx context.Context, emailID string, copyReq *sendRequest) error {
	payload, err := json.Marshal(copyReq)
	if err != nil {
		return fmt.Errorf("marshaling forward: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, f.baseURL+"/emails", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("building send request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+f.cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "inbound-forward/"+emailID)

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("sending forward: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if statusErr := checkStatus(resp); statusErr != nil {
		return fmt.Errorf("sending forward: %w", statusErr)
	}
	return nil
}

func checkStatus(resp *http.Response) error {
	if resp.StatusCode < http.StatusMultipleChoices {
		return nil
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyBytes))
	return fmt.Errorf("%w: status %d: %s", ErrResendRequestFailed, resp.StatusCode, body)
}

// buildCopy composes the email sent to the personal inbox.
func buildCopy(email *receivedEmail, raw []byte, cfg Config) sendRequest {
	subject := email.Subject
	if subject == "" {
		subject = "(sin asunto)"
	}

	copyReq := sendRequest{
		From:    cfg.ForwardFrom,
		To:      []string{cfg.ForwardTo},
		ReplyTo: email.From,
		Subject: "Fwd: " + subject,
	}
	if raw != nil {
		copyReq.Attachments = []sendAttachment{{
			Filename:    "mensaje-original.eml",
			Content:     base64.StdEncoding.EncodeToString(raw),
			ContentType: "message/rfc822",
		}}
	}

	htmlHeader, textHeader := copyHeader(email, raw != nil)
	text := ""
	if email.Text != nil {
		text = *email.Text
	}
	if email.HTML != nil && *email.HTML != "" {
		copyReq.HTML = htmlHeader + *email.HTML
	} else {
		copyReq.HTML = htmlHeader +
			`<pre style="white-space:pre-wrap;font:14px/1.5 sans-serif">` + html.EscapeString(text) + "</pre>"
	}
	copyReq.Text = textHeader + text
	return copyReq
}

// copyHeader renders the block that opens the copy, as HTML and as plain text.
// Every value comes from the received email, so the HTML version escapes them;
// the original HTML body that follows is passed through as the sender wrote it,
// which is what any mail client shows anyway.
func copyHeader(email *receivedEmail, hasOriginal bool) (htmlHeader, textHeader string) {
	sender := email.From
	if display, ok := email.Headers["from"].(string); ok && display != "" {
		sender = display
	}

	fields := [][2]string{
		{"De", sender},
		{"Para", strings.Join(email.To, ", ")},
		{"Fecha", email.CreatedAt},
	}
	if email.Auth != nil {
		fields = append(fields, [2]string{"Autenticación", fmt.Sprintf("SPF %s, DKIM %s, DMARC %s",
			email.Auth.SPF, email.Auth.DKIM, email.Auth.DMARC)})
	}

	var h, t strings.Builder
	h.WriteString(`<div style="font:13px/1.5 sans-serif;color:#52525b;` +
		`border-bottom:1px solid #e4e4e7;padding-bottom:8px;margin-bottom:16px">`)
	h.WriteString("<strong>Correo recibido en tapeandocartones.es</strong><br>")
	t.WriteString("Correo recibido en tapeandocartones.es\n")
	for _, f := range fields {
		fmt.Fprintf(&h, "%s: %s<br>", f[0], html.EscapeString(f[1]))
		fmt.Fprintf(&t, "%s: %s\n", f[0], f[1])
	}
	if !hasOriginal {
		h.WriteString(html.EscapeString(missingOriginalNote) + "<br>")
		t.WriteString(missingOriginalNote + "\n")
	}
	h.WriteString("</div>")
	t.WriteString("\n")
	return h.String(), t.String()
}
