package pantheon

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
	"time"

	"github.com/pantheon-social/oauth-chat-bot/internal/domain"
)

const RequiredScopes = "chat:read chat:write streams:manage user:read"

type Config struct {
	ClientID, ClientSecret, ChannelID string
	AuthURL, TokenURL, APIBase        string
	RedirectURI, ListenAddr           string
}

type tokens struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
	Scope        string `json:"scope"`
	expiresAt    time.Time
}

type Client struct {
	cfg    Config
	http   *http.Client
	tokens tokens
}

func New(ctx context.Context, cfg Config) (*Client, error) {
	client := &Client{cfg: cfg, http: &http.Client{Timeout: 15 * time.Second}}
	value, err := authorize(ctx, cfg)
	if err != nil {
		return nil, err
	}
	if err := requireScopes(value.Scope); err != nil {
		return nil, err
	}
	client.tokens = value
	return client, nil
}

func (c *Client) Scopes() string { return c.tokens.Scope }

func (c *Client) CurrentUser(ctx context.Context) (domain.CurrentUser, error) {
	var user domain.CurrentUser
	if err := c.apiJSON(ctx, http.MethodGet, "/v1/me", nil, nil, &user); err != nil {
		return domain.CurrentUser{}, err
	}
	return user, nil
}

func (c *Client) SendMessage(ctx context.Context, text string) error {
	headers := http.Header{"Idempotency-Key": {randomToken(16)}}
	path := "/v1/chat/channels/" + url.PathEscape(c.cfg.ChannelID) + "/messages"
	return c.apiJSON(ctx, http.MethodPost, path, map[string]string{"text": text}, headers, nil)
}

func (c *Client) SetStreamTitle(ctx context.Context, title string) error {
	err := c.apiJSON(ctx, http.MethodPatch, "/v1/me/stream", map[string]string{"title": title}, nil, nil)
	var httpErr *HTTPError
	if errors.As(err, &httpErr) && httpErr.StatusCode == http.StatusConflict {
		return domain.ErrStreamNotLive
	}
	return err
}

func (c *Client) apiJSON(ctx context.Context, method, path string, body any, headers http.Header, target any) error {
	if err := c.ensureToken(ctx); err != nil {
		return err
	}
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.cfg.APIBase+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.tokens.AccessToken)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for name, values := range headers {
		for _, value := range values {
			req.Header.Add(name, value)
		}
	}
	response, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode/100 != 2 {
		return responseError(response)
	}
	if target != nil {
		return json.NewDecoder(response.Body).Decode(target)
	}
	return nil
}

func (c *Client) ensureToken(ctx context.Context) error {
	if time.Until(c.tokens.expiresAt) >= 30*time.Second {
		return nil
	}
	if c.tokens.RefreshToken == "" {
		return fmt.Errorf("access token expired and no refresh token was returned")
	}
	value, err := tokenRequest(ctx, c.cfg, url.Values{
		"grant_type": {"refresh_token"}, "client_id": {c.cfg.ClientID}, "refresh_token": {c.tokens.RefreshToken},
	})
	if err != nil {
		return fmt.Errorf("refresh token: %w", err)
	}
	if value.RefreshToken == "" {
		value.RefreshToken = c.tokens.RefreshToken
	}
	if err := requireScopes(value.Scope); err != nil {
		return err
	}
	c.tokens = value
	return nil
}

func requireScopes(granted string) error {
	available := make(map[string]bool)
	for _, scope := range strings.Fields(granted) {
		available[scope] = true
	}
	for _, scope := range strings.Fields(RequiredScopes) {
		if !available[scope] {
			return fmt.Errorf("OAuth application did not grant required scope %s", scope)
		}
	}
	return nil
}

type HTTPError struct {
	StatusCode int
	Body       string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Body)
}

func responseError(response *http.Response) error {
	payload, _ := io.ReadAll(io.LimitReader(response.Body, 32<<10))
	return &HTTPError{StatusCode: response.StatusCode, Body: strings.TrimSpace(string(payload))}
}
