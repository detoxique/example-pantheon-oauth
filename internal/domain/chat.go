package domain

import (
	"errors"
	"strings"
)

var ErrStreamNotLive = errors.New("stream is not live")

const (
	SystemBadgeID      = "0"
	OwnerBadgeName     = "Владелец канала"
	ModeratorBadgeName = "Модератор"
)

type Badge struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type ChatMessage struct {
	MessageID   string  `json:"message_id"`
	ChannelID   string  `json:"channel_id"`
	UserID      string  `json:"user_id"`
	DisplayName string  `json:"display_name"`
	Login       string  `json:"login"`
	Text        string  `json:"text"`
	Badges      []Badge `json:"badges"`
}

func (m ChatMessage) CanManageStream() bool {
	for _, badge := range m.Badges {
		if badge.ID != SystemBadgeID {
			continue
		}
		if strings.EqualFold(badge.Name, OwnerBadgeName) || strings.EqualFold(badge.Name, ModeratorBadgeName) {
			return true
		}
	}
	return false
}

type ChatEvent struct {
	Type    string
	EventID string
	Message *ChatMessage
}

type CurrentUser struct {
	ID string `json:"user_id"`
}
