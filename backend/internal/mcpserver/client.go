package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	// requestTimeout bounds one API call. Generous on purpose: Render's free
	// tier spins the API down when idle and the first request after that
	// waits for a cold start (ADR-0015).
	requestTimeout = 90 * time.Second
	// maxResponseBytes caps how much of a response body is read. The largest
	// real payloads (a full deck list, a game timeline) are far below it.
	maxResponseBytes = 8 << 20
	userAgent        = "commander-companion-mcp"
)

var (
	// ErrMissingCredentials is returned by NewClient when the email or the
	// password is empty.
	ErrMissingCredentials = errors.New("COMMANDER_EMAIL and COMMANDER_PASSWORD are required")
	// ErrInvalidBaseURL is returned by NewClient for a base URL that is not
	// an absolute http(s) URL.
	ErrInvalidBaseURL = errors.New("COMMANDER_API_URL must be an absolute http(s) URL")
)

// APIError is a non-2xx answer from the API, carrying the message of its
// ErrorResponse body. Its text is what the model reads, so it stays short.
type APIError struct {
	Status  int
	Message string
}

func (e *APIError) Error() string {
	if e.Message == "" {
		return fmt.Sprintf("API answered %d %s", e.Status, http.StatusText(e.Status))
	}
	return fmt.Sprintf("API answered %d: %s", e.Status, e.Message)
}

// isStatus reports whether err is an APIError with the given status.
func isStatus(err error, status int) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.Status == status
}

// ClientConfig is what NewClient needs to log in.
type ClientConfig struct {
	// BaseURL is the API root, including /api/v1.
	BaseURL  string
	Email    string
	Password string
	// HTTPClient is optional; tests pass the httptest server's client.
	HTTPClient *http.Client
}

// Client calls the Commander Companion REST API as one user. It logs in with
// email and password on first use and renews the session by itself: a 401
// triggers one refresh (or a fresh login when the refresh token is gone too)
// and one retry. Tokens live in memory only, never on disk.
//
// Renewal runs under a mutex, so concurrent tool calls never present the same
// refresh token twice — the API treats a replayed refresh token as theft and
// revokes the whole session family (ADR-0001, ADR-0028).
type Client struct {
	baseURL  string
	email    string
	password string
	http     *http.Client

	mu       sync.Mutex
	access   string
	refresh  string
	userID   string
	username string
	// loginErr latches a login the API rejected (wrong credentials, an
	// unverified email, a rate limit). Failed logins count against a
	// per-account limit (ADR-0025), so retrying them on every tool call would
	// lock the user out of the web and Android apps too. Restart to retry.
	loginErr error
}

// NewClient validates cfg and returns a client that has not logged in yet.
func NewClient(cfg *ClientConfig) (*Client, error) {
	if cfg.Email == "" || cfg.Password == "" {
		return nil, ErrMissingCredentials
	}
	u, err := url.Parse(cfg.BaseURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, ErrInvalidBaseURL
	}
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: requestTimeout}
	}
	return &Client{
		baseURL:  strings.TrimRight(cfg.BaseURL, "/"),
		email:    cfg.Email,
		password: cfg.Password,
		http:     httpClient,
	}, nil
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	User         struct {
		ID       string `json:"id"`
		Username string `json:"username"`
	} `json:"user"`
}

// Me returns the logged-in user's ID and username, logging in if needed.
func (c *Client) Me(ctx context.Context) (id, username string, err error) {
	if _, err := c.token(ctx); err != nil {
		return "", "", err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.userID, c.username, nil
}

// Get calls GET path with the query and decodes the JSON answer into out.
func (c *Client) Get(ctx context.Context, path string, query url.Values, out any) error {
	access, err := c.token(ctx)
	if err != nil {
		return err
	}
	err = c.do(ctx, http.MethodGet, path, query, nil, access, out)
	if !isStatus(err, http.StatusUnauthorized) {
		return err
	}
	if err := c.renew(ctx, access); err != nil {
		return err
	}
	c.mu.Lock()
	access = c.access
	c.mu.Unlock()
	return c.do(ctx, http.MethodGet, path, query, nil, access, out)
}

// Logout revokes the session's refresh token. Best effort: it is called on
// shutdown, when there is nobody left to report a failure to.
func (c *Client) Logout(ctx context.Context) {
	c.mu.Lock()
	refresh := c.refresh
	c.access, c.refresh = "", ""
	c.mu.Unlock()
	if refresh == "" {
		return
	}
	_ = c.do(ctx, http.MethodPost, pathLogout, nil, map[string]string{"refresh_token": refresh}, "", nil)
}

// token returns the current access token, logging in first if there is none.
func (c *Client) token(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.access == "" {
		if err := c.loginLocked(ctx); err != nil {
			return "", err
		}
	}
	return c.access, nil
}

// renew replaces an access token the API rejected. stale is the token that
// got the 401: if another call already renewed it, there is nothing to do.
func (c *Client) renew(ctx context.Context, stale string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.access != stale {
		return nil
	}
	if c.refresh != "" {
		var res tokenResponse
		err := c.do(ctx, http.MethodPost, pathRefresh, nil, map[string]string{"refresh_token": c.refresh}, "", &res)
		if err == nil {
			c.setTokensLocked(&res)
			return nil
		}
	}
	return c.loginLocked(ctx)
}

func (c *Client) loginLocked(ctx context.Context) error {
	if c.loginErr != nil {
		return c.loginErr
	}
	var res tokenResponse
	body := map[string]string{"email": c.email, "password": c.password}
	if err := c.do(ctx, http.MethodPost, pathLogin, nil, body, "", &res); err != nil {
		err = fmt.Errorf("logging in as %s: %w", c.email, err)
		if isStatus(err, http.StatusUnauthorized) || isStatus(err, http.StatusForbidden) ||
			isStatus(err, http.StatusTooManyRequests) {
			c.loginErr = fmt.Errorf("%w (not retried until the MCP server restarts)", err)
			return c.loginErr
		}
		return err
	}
	c.setTokensLocked(&res)
	return nil
}

func (c *Client) setTokensLocked(res *tokenResponse) {
	c.access, c.refresh = res.AccessToken, res.RefreshToken
	// /auth/refresh may omit the user; keep the one login returned.
	if res.User.ID != "" {
		c.userID, c.username = res.User.ID, res.User.Username
	}
}

// do sends one request. A nil out discards the body.
func (c *Client) do(
	ctx context.Context, method, path string, query url.Values, body any, access string, out any,
) error {
	req, err := c.newRequest(ctx, method, path, query, body, access)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("calling the API: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return fmt.Errorf("reading the API's answer: %w", err)
	}

	if resp.StatusCode >= http.StatusBadRequest {
		return newAPIError(resp.StatusCode, raw)
	}
	if out == nil || len(raw) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("decoding %s %s: %w", method, path, err)
	}
	return nil
}

func (c *Client) newRequest(
	ctx context.Context, method, path string, query url.Values, body any, access string,
) (*http.Request, error) {
	target := c.baseURL + path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("encoding request: %w", err)
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, target, reader)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if access != "" {
		req.Header.Set("Authorization", "Bearer "+access)
	}
	return req, nil
}

// newAPIError reads the ErrorResponse body of a failed call, if it has one.
func newAPIError(status int, body []byte) *APIError {
	apiErr := &APIError{Status: status}
	var payload struct {
		Message string `json:"message"`
	}
	if json.Unmarshal(body, &payload) == nil {
		apiErr.Message = payload.Message
	}
	return apiErr
}

// withID fills the {id} placeholder of an endpoint template.
func withID(template, id string) string {
	return strings.Replace(template, "{id}", url.PathEscape(id), 1)
}
