// Package notify sends operational notifications to the team's Slack, such as a
// message every time a new account signs up (useful while the user base is small
// enough to want to know about each one).
//
// It uses a Slack incoming webhook: no bot, no OAuth, just a URL that posts to one
// channel. Sending is fire-and-forget: a notification never slows down or fails the
// request that triggered it.
package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

const httpTimeout = 10 * time.Second

// Signup methods, shown in the notification.
const (
	SignupMethodPassword = "email/password"
	SignupMethodGoogle   = "Google"
)

// ErrSlackRequestFailed indicates that the Slack webhook responded with an error status
// (revoked webhook, deleted channel, etc.).
var ErrSlackRequestFailed = errors.New("slack request failed")

// SignupNotifier is what the rest of the backend needs to announce a new account.
type SignupNotifier interface {
	NotifySignup(username, method string)
}

// NewSlackNotifier builds the signup notifier. If webhookURL is empty (local
// development, CI, or simply not configured) it returns a notifier that does nothing.
func NewSlackNotifier(webhookURL string) SignupNotifier {
	if webhookURL == "" {
		return noopNotifier{}
	}
	return &slackNotifier{
		webhookURL: webhookURL,
		httpClient: &http.Client{Timeout: httpTimeout},
	}
}

type noopNotifier struct{}

// NotifySignup does nothing (see NewSlackNotifier).
func (noopNotifier) NotifySignup(string, string) {}

type slackNotifier struct {
	webhookURL string
	httpClient *http.Client
}

type slackMessage struct {
	Text string `json:"text"`
}

// NotifySignup posts the message in the background and only logs failures: the
// account already exists, and losing a notification is not worth an error.
func (n *slackNotifier) NotifySignup(username, method string) {
	text := fmt.Sprintf(":tada: Nuevo usuario registrado: *%s* (%s)", escapeMrkdwn(username), method)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), httpTimeout)
		defer cancel()
		if err := n.post(ctx, text); err != nil {
			log.Printf("could not send the signup notification to Slack: %v", err)
		}
	}()
}

func (n *slackNotifier) post(ctx context.Context, text string) error {
	payload, err := json.Marshal(slackMessage{Text: text})
	if err != nil {
		return fmt.Errorf("marshaling slack message: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.webhookURL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("building slack request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("calling slack: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("%w: status %d", ErrSlackRequestFailed, resp.StatusCode)
	}
	return nil
}

// escapeMrkdwn escapes the three characters Slack treats as control characters in
// message text. The username is chosen by whoever signs up, so without this a name
// like "<!channel>" would ping the whole channel.
func escapeMrkdwn(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(s)
}
