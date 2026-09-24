package pantheon

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/pantheon-social/oauth-chat-bot/internal/application"
	"github.com/pantheon-social/oauth-chat-bot/internal/domain"
	"golang.org/x/net/websocket"
)

type wireEvent struct {
	Type    string          `json:"type"`
	EventID string          `json:"event_id"`
	Data    json.RawMessage `json:"data"`
}

func (c *Client) Subscribe(ctx context.Context) (application.Subscription, error) {
	if err := c.ensureToken(ctx); err != nil {
		return application.Subscription{}, err
	}
	path := "/v1/chat/channels/" + url.PathEscape(c.cfg.ChannelID) + "/websocket-ticket"
	var ticket struct {
		Ticket string `json:"ticket"`
	}
	if err := c.apiJSON(ctx, http.MethodPost, path, nil, nil, &ticket); err != nil {
		return application.Subscription{}, fmt.Errorf("create websocket ticket: %w", err)
	}
	base, err := url.Parse(c.cfg.APIBase)
	if err != nil {
		return application.Subscription{}, err
	}
	if base.Scheme == "https" {
		base.Scheme = "wss"
	} else {
		base.Scheme = "ws"
	}
	base.Path = "/v1/chat/ws"
	base.RawQuery = url.Values{"ticket": {ticket.Ticket}}.Encode()
	conn, err := websocket.Dial(base.String(), "", c.cfg.APIBase)
	if err != nil {
		return application.Subscription{}, err
	}

	events := make(chan domain.ChatEvent, 64)
	failures := make(chan error, 1)
	go c.readEvents(ctx, conn, events, failures)
	return application.Subscription{Events: events, Errors: failures}, nil
}

func (c *Client) readEvents(ctx context.Context, conn *websocket.Conn, events chan<- domain.ChatEvent, failures chan<- error) {
	defer conn.Close()
	defer close(events)
	defer close(failures)
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			_ = conn.Close()
		case <-done:
		}
	}()

	for {
		var wire wireEvent
		if err := websocket.JSON.Receive(conn, &wire); err != nil {
			if ctx.Err() == nil {
				failures <- err
			}
			return
		}
		event := domain.ChatEvent{Type: wire.Type, EventID: wire.EventID}
		switch wire.Type {
		case "ping":
			continue
		case "error":
			failures <- fmt.Errorf("websocket error: %s", strings.TrimSpace(string(wire.Data)))
			return
		case "message.created":
			var message domain.ChatMessage
			if err := json.Unmarshal(wire.Data, &message); err != nil {
				failures <- fmt.Errorf("decode chat message: %w", err)
				return
			}
			event.Message = &message
		}
		select {
		case events <- event:
		case <-ctx.Done():
			return
		}
	}
}
