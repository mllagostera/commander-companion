// Package email sends the transactional account verification mail via Resend.
//
// The content (subject, copy, layout) lives in a Template in the Resend dashboard, not
// here: this package only references the template by ID and passes it the variables
// (USERNAME, VERIFY_URL). The verification link is rendered in the template as
// plain text (not as a button/href) because Resend's REST API breaks URLs that
// go inside an href attribute when they come from a template variable (see the
// open issue at https://github.com/resend/react-email/issues/3247).
package email

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

const resendEmailsURL = "https://api.resend.com/emails"

const httpTimeout = 10 * time.Second

// ErrResendRequestFailed indicates that Resend responded with an error status when sending
// the mail (invalid API key, unpublished template, etc.).
var ErrResendRequestFailed = errors.New("resend request failed")

// Config groups the configuration parameters for sending transactional mail.
type Config struct {
	APIKey                string
	FromAddress           string
	VerifyEmailTemplateID string
}

// Sender is what the rest of the backend needs to send the account verification
// mail.
type Sender interface {
	SendVerificationEmail(ctx context.Context, to, username, verifyURL string) error
}

// NewResendClient builds the mail-sending client. If cfg.APIKey is empty
// (no Resend account configured) it returns a "console" mailer that logs the
// verification link instead of sending it, so local development doesn't depend on
// having a Resend account (see docker-compose.yml).
func NewResendClient(cfg Config) Sender {
	if cfg.APIKey == "" {
		return &consoleClient{}
	}
	return &resendClient{
		apiKey:                cfg.APIKey,
		from:                  cfg.FromAddress,
		verifyEmailTemplateID: cfg.VerifyEmailTemplateID,
		httpClient:            &http.Client{Timeout: httpTimeout},
	}
}

type consoleClient struct{}

// SendVerificationEmail logs the verification link instead of sending it (see
// NewResendClient).
func (c *consoleClient) SendVerificationEmail(_ context.Context, to, username, verifyURL string) error {
	log.Printf("[email consola] verificación para %s (%s): %s", username, to, verifyURL)
	return nil
}

type resendClient struct {
	apiKey                string
	from                  string
	verifyEmailTemplateID string
	httpClient            *http.Client
}

type resendTemplate struct {
	ID        string            `json:"id"`
	Variables map[string]string `json:"variables"`
}

type resendSendRequest struct {
	From     string         `json:"from"`
	To       []string       `json:"to"`
	Template resendTemplate `json:"template"`
}

// SendVerificationEmail sends the mail via the configured Resend Template (see
// Config.VerifyEmailTemplateID).
func (c *resendClient) SendVerificationEmail(ctx context.Context, to, username, verifyURL string) error {
	payload, err := json.Marshal(resendSendRequest{
		From: c.from,
		To:   []string{to},
		Template: resendTemplate{
			ID: c.verifyEmailTemplateID,
			Variables: map[string]string{
				"USERNAME":   username,
				"VERIFY_URL": verifyURL,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("marshaling resend request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, resendEmailsURL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("building resend request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("calling resend: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode >= http.StatusMultipleChoices {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%w: status %d: %s", ErrResendRequestFailed, resp.StatusCode, respBody)
	}

	return nil
}
