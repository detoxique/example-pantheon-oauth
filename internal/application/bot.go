package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/pantheon-social/oauth-chat-bot/internal/domain"
)

const (
	greetingCommand = "!привет"
	greetingReply   = "привет от API"
	titleCommand    = "!title"
	maxTitleLength  = 200
)

type Subscription struct {
	Events <-chan domain.ChatEvent
	Errors <-chan error
}

type Platform interface {
	CurrentUser(context.Context) (domain.CurrentUser, error)
	Subscribe(context.Context) (Subscription, error)
	SendMessage(context.Context, string) error
	SetStreamTitle(context.Context, string) error
}

type Logger interface {
	Printf(string, ...any)
}

type Bot struct {
	channelID string
	platform  Platform
	log       Logger
	seen      map[string]struct{}
	order     []string
}

func New(channelID string, platform Platform, logger Logger) *Bot {
	return &Bot{channelID: channelID, platform: platform, log: logger, seen: make(map[string]struct{})}
}

func (b *Bot) Run(ctx context.Context) error {
	user, err := b.platform.CurrentUser(ctx)
	if err != nil {
		return fmt.Errorf("get OAuth user: %w", err)
	}
	if user.ID != b.channelID {
		return fmt.Errorf("OAuth user %s is not channel owner %s", user.ID, b.channelID)
	}

	for ctx.Err() == nil {
		subscription, err := b.platform.Subscribe(ctx)
		if err == nil {
			err = b.consume(ctx, subscription)
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		b.log.Printf("соединение с чатом потеряно: %v; повтор через 2 секунды", err)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	return ctx.Err()
}

func (b *Bot) consume(ctx context.Context, subscription Subscription) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err, ok := <-subscription.Errors:
			if !ok {
				return errors.New("chat subscription closed")
			}
			return err
		case event, ok := <-subscription.Events:
			if !ok {
				return errors.New("chat event stream closed")
			}
			if event.Type == "connection.ready" {
				b.log.Printf("бот читает канал %s", b.channelID)
				continue
			}
			if event.Type != "message.created" || event.Message == nil || b.duplicate(event.EventID) {
				continue
			}
			b.handleMessage(ctx, *event.Message)
		}
	}
}

func (b *Bot) handleMessage(ctx context.Context, message domain.ChatMessage) {
	b.log.Printf("%s: %s", first(message.DisplayName, message.Login), message.Text)
	text := strings.TrimSpace(message.Text)
	switch {
	case text == greetingCommand:
		b.reply(ctx, greetingReply)
	case text == titleCommand:
		b.reply(ctx, "использование: !title Название")
	case strings.HasPrefix(text, titleCommand+" "):
		title := strings.TrimSpace(strings.TrimPrefix(text, titleCommand))
		if utf8.RuneCountInString(title) > maxTitleLength {
			b.reply(ctx, "название не должно быть длиннее 200 символов")
			return
		}
		if !message.CanManageStream() {
			b.reply(ctx, "команда доступна только владельцу канала и модераторам")
			return
		}
		if err := b.platform.SetStreamTitle(ctx, title); err != nil {
			b.log.Printf("не удалось изменить название стрима: %v", err)
			if errors.Is(err, domain.ErrStreamNotLive) {
				b.reply(ctx, "сначала запустите стрим")
				return
			}
			b.reply(ctx, "не удалось изменить название стрима")
			return
		}
		b.reply(ctx, "название стрима изменено: "+title)
	}
}

func (b *Bot) reply(ctx context.Context, text string) {
	if err := b.platform.SendMessage(ctx, text); err != nil {
		b.log.Printf("не удалось ответить в чат: %v", err)
	}
}

func (b *Bot) duplicate(id string) bool {
	if id == "" {
		return false
	}
	if _, ok := b.seen[id]; ok {
		return true
	}
	b.seen[id] = struct{}{}
	b.order = append(b.order, id)
	if len(b.order) > 1000 {
		delete(b.seen, b.order[0])
		b.order = b.order[1:]
	}
	return false
}

func first(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return "unknown"
}
