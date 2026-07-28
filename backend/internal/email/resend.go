// Package email manda el mail transaccional de verificación de cuenta vía Resend.
//
// El contenido (asunto, copy, layout) vive en un Template del dashboard de Resend, no
// acá: este paquete solo referencia el template por ID y le pasa las variables
// (USERNAME, VERIFY_URL). El link de verificación se renderiza en el template como
// texto plano (no como botón/href) porque la REST API de Resend rompe las URLs que
// van dentro de un atributo href cuando vienen de una variable de template (bug
// abierto: https://github.com/resend/react-email/issues/3247).
package email

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

const resendEmailsURL = "https://api.resend.com/emails"

const httpTimeout = 10 * time.Second

// Config agrupa los parámetros de configuración del envío de mail transaccional.
type Config struct {
	APIKey                string
	FromAddress           string
	VerifyEmailTemplateID string
}

// Sender es lo que el resto del backend necesita para mandar el mail de verificación
// de cuenta.
type Sender interface {
	SendVerificationEmail(ctx context.Context, to, username, verifyURL string) error
}

// NewResendClient construye el cliente de envío de mail. Si cfg.APIKey está vacío
// (sin cuenta de Resend configurada) devuelve un mailer "de consola" que loguea el
// link de verificación en vez de mandarlo, para que el desarrollo local no dependa de
// tener una cuenta de Resend (ver docker-compose.yml).
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
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusMultipleChoices {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("resend respondió %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}
