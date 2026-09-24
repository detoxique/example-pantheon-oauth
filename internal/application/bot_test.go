package application

import (
	"context"
	"testing"

	"github.com/pantheon-social/oauth-chat-bot/internal/domain"
)

type platformStub struct {
	sent     []string
	titles   []string
	titleErr error
}

func (p *platformStub) CurrentUser(context.Context) (domain.CurrentUser, error) {
	return domain.CurrentUser{ID: "42"}, nil
}
func (p *platformStub) Subscribe(context.Context) (Subscription, error) { return Subscription{}, nil }
func (p *platformStub) SendMessage(_ context.Context, text string) error {
	p.sent = append(p.sent, text)
	return nil
}
func (p *platformStub) SetStreamTitle(_ context.Context, title string) error {
	p.titles = append(p.titles, title)
	return p.titleErr
}

type loggerStub struct{}

func (loggerStub) Printf(string, ...any) {}

func TestTitleRequiresRoleBadge(t *testing.T) {
	platform := &platformStub{}
	bot := New("42", platform, loggerStub{})
	bot.handleMessage(context.Background(), domain.ChatMessage{Text: "!title Новый эфир"})
	if len(platform.titles) != 0 {
		t.Fatal("unauthorized user changed title")
	}
	if len(platform.sent) != 1 || platform.sent[0] != "команда доступна только владельцу канала и модераторам" {
		t.Fatalf("unexpected reply: %v", platform.sent)
	}
}

func TestModeratorCanChangeTitle(t *testing.T) {
	platform := &platformStub{}
	bot := New("42", platform, loggerStub{})
	bot.handleMessage(context.Background(), domain.ChatMessage{
		Text:   "!title Новый эфир",
		Badges: []domain.Badge{{ID: domain.SystemBadgeID, Name: domain.ModeratorBadgeName}},
	})
	if len(platform.titles) != 1 || platform.titles[0] != "Новый эфир" {
		t.Fatalf("unexpected titles: %v", platform.titles)
	}
}

func TestOwnerCanChangeTitle(t *testing.T) {
	platform := &platformStub{}
	bot := New("42", platform, loggerStub{})
	bot.handleMessage(context.Background(), domain.ChatMessage{
		Text:   "!title Владелец в эфире",
		Badges: []domain.Badge{{ID: domain.SystemBadgeID, Name: domain.OwnerBadgeName}},
	})
	if len(platform.titles) != 1 || platform.titles[0] != "Владелец в эфире" {
		t.Fatalf("unexpected titles: %v", platform.titles)
	}
}

func TestCustomBadgeCannotGrantModeratorRole(t *testing.T) {
	platform := &platformStub{}
	bot := New("42", platform, loggerStub{})
	bot.handleMessage(context.Background(), domain.ChatMessage{
		Text:   "!title Новый эфир",
		Badges: []domain.Badge{{ID: "17", Name: domain.ModeratorBadgeName}},
	})
	if len(platform.titles) != 0 {
		t.Fatal("custom badge granted moderator role")
	}
}

func TestTitleRequiresLiveStream(t *testing.T) {
	platform := &platformStub{titleErr: domain.ErrStreamNotLive}
	bot := New("42", platform, loggerStub{})
	bot.handleMessage(context.Background(), domain.ChatMessage{
		Text:   "!title Новый эфир",
		Badges: []domain.Badge{{ID: domain.SystemBadgeID, Name: domain.OwnerBadgeName}},
	})
	if len(platform.sent) != 1 || platform.sent[0] != "сначала запустите стрим" {
		t.Fatalf("unexpected reply: %v", platform.sent)
	}
}
