// Package shiori implements the client for shiori.ai: refresh-token auth,
// the model catalog, and the streaming chat endpoint.
//
// shiori.ai runs on a Convex backend. A long-lived refresh token (stored in
// the browser as localStorage.__convexAuthRefreshToken_*) is exchanged for a
// short-lived JWT, which authorises every API call as a bearer token.
package shiori

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const defaultBaseURL = "https://www.shiori.ai"

// Client talks to shiori.ai on behalf of one account. It caches the
// short-lived JWT and transparently refreshes it using the refresh token.
//
// shiori.ai sits behind Cloudflare, so a bare HTTP client is challenged.
// When Cookie is set, every request carries that Cookie header, which must
// include the cf_clearance value and the auth cookies copied from a real
// logged-in browser session.
type Client struct {
	BaseURL      string
	RefreshToken string
	Cookie       string
	UserAgent    string
	HTTP         *http.Client

	mu      sync.Mutex
	jwt     string
	jwtExp  time.Time
	refresh string // rotated refresh token, if the server issues a new one
	now     func() time.Time
}

// NewClient builds a client for one refresh token. cookie is optional and,
// when provided, is sent on every request (needed to pass Cloudflare).
// proxyURL is an optional upstream HTTP(S) proxy.
func NewClient(refreshToken, baseURL, cookie, proxyURL string) *Client {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = defaultBaseURL
	}
	c := &Client{
		BaseURL:      strings.TrimRight(baseURL, "/"),
		RefreshToken: strings.TrimSpace(refreshToken),
		Cookie:       strings.TrimSpace(cookie),
		UserAgent:    "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/146.0.0.0 Safari/537.36",
		now:          time.Now,
	}
	transport := &http.Transport{
		Proxy:                 proxyFunc(proxyURL),
		DialContext:           (&net.Dialer{Timeout: 15 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          32,
		MaxIdleConnsPerHost:   8,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 60 * time.Second,
	}
	c.HTTP = &http.Client{
		Transport: &cookieTransport{base: transport, cookie: c.Cookie},
	}
	return c
}

// proxyFunc returns a proxy selector: the explicit proxy when set, otherwise
// the standard environment-based lookup.
func proxyFunc(proxyURL string) func(*http.Request) (*url.URL, error) {
	if strings.TrimSpace(proxyURL) == "" {
		return http.ProxyFromEnvironment
	}
	u, err := url.Parse(proxyURL)
	if err != nil {
		return http.ProxyFromEnvironment
	}
	return http.ProxyURL(u)
}

// cookieTransport injects a Cookie header on every outgoing request so the
// configured browser session (Cloudflare clearance + auth cookies) is used.
type cookieTransport struct {
	base   http.RoundTripper
	cookie string
}

func (t *cookieTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.cookie != "" {
		req = req.Clone(req.Context())
		if existing := req.Header.Get("Cookie"); existing != "" {
			req.Header.Set("Cookie", existing+"; "+t.cookie)
		} else {
			req.Header.Set("Cookie", t.cookie)
		}
	}
	return t.base.RoundTrip(req)
}

// APIError is a structured upstream failure.
type APIError struct {
	HTTPStatus int
	Msg        string
	// Cloudflare is true when the response is a Cloudflare challenge page
	// rather than a real API response. This means the request lacked a valid
	// cf_clearance cookie and the operator must supply one.
	Cloudflare bool
}

func (e *APIError) Error() string {
	return fmt.Sprintf("shiori api: http %d: %s", e.HTTPStatus, e.Msg)
}

// looksLikeCloudflare reports whether a response body is a Cloudflare
// managed-challenge / block page.
func looksLikeCloudflare(body string) bool {
	l := strings.ToLower(body)
	return strings.Contains(l, "just a moment") ||
		strings.Contains(l, "cf-chl") ||
		strings.Contains(l, "cloudflare") ||
		strings.Contains(l, "attention required")
}

type authResponse struct {
	Tokens struct {
		Token        string `json:"token"`
		RefreshToken string `json:"refreshToken"`
	} `json:"tokens"`
}

// Token returns a valid JWT. When the configured credential is already a JWT
// (three dot-separated segments) it is returned directly, since it cannot be
// refreshed; otherwise the refresh token is exchanged for a new JWT when the
// cached one is absent or near expiry.
func (c *Client) Token(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if isJWT(c.RefreshToken) {
		return c.RefreshToken, nil
	}
	if c.jwt != "" && c.now().Before(c.jwtExp) {
		return c.jwt, nil
	}
	if err := c.signInLocked(ctx); err != nil {
		return "", err
	}
	return c.jwt, nil
}

// isJWT reports whether s looks like a JSON Web Token.
func isJWT(s string) bool {
	return strings.Count(s, ".") == 2 && strings.HasPrefix(s, "eyJ")
}

// signInLocked exchanges the current refresh token for a fresh JWT. Callers
// hold c.mu.
func (c *Client) signInLocked(ctx context.Context) error {
	refresh := c.refresh
	if refresh == "" {
		refresh = c.RefreshToken
	}
	body, err := json.Marshal(map[string]any{
		"action": "auth:signIn",
		"args":   map[string]any{"refreshToken": refresh},
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/api/auth", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Origin", c.BaseURL)
	req.Header.Set("Referer", c.BaseURL+"/")
	req.Header.Set("User-Agent", c.UserAgent)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return newAPIError(resp.StatusCode, data)
	}
	var ar authResponse
	if err := json.Unmarshal(data, &ar); err != nil {
		return fmt.Errorf("shiori: bad auth response: %w", err)
	}
	if ar.Tokens.Token == "" {
		return fmt.Errorf("shiori: auth returned no token")
	}
	c.jwt = ar.Tokens.Token
	c.jwtExp = jwtExpiry(ar.Tokens.Token, c.now())
	if ar.Tokens.RefreshToken != "" {
		c.refresh = ar.Tokens.RefreshToken // the server may rotate it
	}
	return nil
}

// Invalidate forces the next call to refresh the JWT, used after a 401.
func (c *Client) Invalidate() {
	c.mu.Lock()
	c.jwt = ""
	c.jwtExp = time.Time{}
	c.mu.Unlock()
}

// jwtExpiry reads the exp claim, falling back to one hour when it is absent,
// minus a 60s safety margin.
func jwtExpiry(token string, now time.Time) time.Time {
	parts := strings.Split(token, ".")
	if len(parts) == 3 {
		if payload, err := base64RawURLDecode(parts[1]); err == nil {
			var claims struct {
				Exp int64 `json:"exp"`
			}
			if json.Unmarshal(payload, &claims) == nil && claims.Exp > 0 {
				return time.Unix(claims.Exp, 0).Add(-60 * time.Second)
			}
		}
	}
	return now.Add(time.Hour - time.Minute)
}

func base64RawURLDecode(s string) ([]byte, error) {
	return decodeSegment(s)
}

// Catalog group: one provider group with its models.
type CatalogGroup struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Models      []Model `json:"models"`
}

// Model is one entry of the model catalog.
type Model struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	Reasoning     bool   `json:"reasoning"`
	ToolCalling   bool   `json:"toolCalling"`
	MultiModal    bool   `json:"multiModal"`
	Coding        bool   `json:"coding"`
	IsEnabled     bool   `json:"isEnabled"`
	IsPremium     bool   `json:"isPremiumModel"`
	ContextWindow int    `json:"contextWindow"`
	Released      string `json:"released"`
}

// Catalog fetches the public model catalog (no auth required).
func (c *Client) Catalog(ctx context.Context) ([]CatalogGroup, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/api/models/catalog", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.UserAgent)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, newAPIError(resp.StatusCode, data)
	}
	var groups []CatalogGroup
	if err := json.Unmarshal(data, &groups); err != nil {
		return nil, fmt.Errorf("shiori: bad catalog response: %w", err)
	}
	return groups, nil
}

// ChatInput describes one chat turn.
type ChatInput struct {
	ModelID         string
	ChatID          string
	Reasoning       bool
	ReasoningEffort string
	Text            string
	MessageID       string
	Trigger         string // "submit-message"
}

// StreamCallbacks receive parsed chat events. They run synchronously from the
// read loop, so they must not block.
type StreamCallbacks struct {
	// OnStart fires once with the server's message metadata.
	OnStart func(messageID string, meta map[string]any)
	// OnReasoningDelta fires for each reasoning text chunk.
	OnReasoningDelta func(text string)
	// OnTextDelta fires for each answer text chunk.
	OnTextDelta func(text string)
	// OnFinish fires once with the finish reason and metadata.
	OnFinish func(finishReason string, meta map[string]any)
}

// errAuthExpired signals that the caller should refresh and retry once.
var errAuthExpired = &APIError{HTTPStatus: http.StatusUnauthorized, Msg: "Authentication required"}

// Chat POSTs one turn to /api/chat and streams events to cb. On a 401 it
// refreshes the JWT once and retries.
func (c *Client) Chat(ctx context.Context, in ChatInput, cb *StreamCallbacks) error {
	err := c.chatOnce(ctx, in, cb)
	var ae *APIError
	if errorsAs(err, &ae) && ae.HTTPStatus == http.StatusUnauthorized {
		c.Invalidate()
		return c.chatOnce(ctx, in, cb)
	}
	return err
}

func (c *Client) chatOnce(ctx context.Context, in ChatInput, cb *StreamCallbacks) error {
	token, err := c.Token(ctx)
	if err != nil {
		return err
	}

	msgID := in.MessageID
	if msgID == "" {
		msgID = newUUID()
	}
	trigger := in.Trigger
	if trigger == "" {
		trigger = "submit-message"
	}
	effort := in.ReasoningEffort
	if effort == "" {
		if in.Reasoning {
			effort = "medium"
		} else {
			effort = "none"
		}
	}
	body, err := json.Marshal(map[string]any{
		"modelId":          in.ModelID,
		"chatId":           in.ChatID,
		"reasoningEnabled": in.Reasoning,
		"reasoningEffort":  effort,
		"id":               in.ChatID,
		"message": map[string]any{
			"id":    msgID,
			"role":  "user",
			"parts": []any{map[string]any{"type": "text", "text": in.Text}},
		},
		"trigger": trigger,
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Origin", c.BaseURL)
	req.Header.Set("Referer", c.BaseURL+"/")
	req.Header.Set("User-Agent", c.UserAgent)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return newAPIError(resp.StatusCode, data)
	}
	return readEventStream(resp.Body, cb)
}

// newAPIError builds an APIError, flagging Cloudflare challenge pages so the
// HTTP layer can return an actionable message instead of a generic auth error.
func newAPIError(status int, body []byte) *APIError {
	raw := string(body)
	if looksLikeCloudflare(raw) {
		return &APIError{
			HTTPStatus: http.StatusBadGateway,
			Msg:        "blocked by Cloudflare; set SHIORI_COOKIE to a valid cf_clearance cookie from a logged-in browser session",
			Cloudflare: true,
		}
	}
	msg := truncate(raw, 300)
	if status == http.StatusUnauthorized {
		msg = "Authentication required"
	}
	return &APIError{HTTPStatus: status, Msg: msg}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
