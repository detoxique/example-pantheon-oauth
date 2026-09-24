package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/pantheon-social/oauth-chat-bot/internal/application"
	"github.com/pantheon-social/oauth-chat-bot/internal/config"
	"github.com/pantheon-social/oauth-chat-bot/internal/pantheon"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	client, err := pantheon.New(ctx, pantheon.Config{
		ClientID: cfg.ClientID, ClientSecret: cfg.ClientSecret, ChannelID: cfg.ChannelID,
		AuthURL: cfg.AuthURL, TokenURL: cfg.TokenURL, APIBase: cfg.APIBase,
		RedirectURI: cfg.RedirectURI, ListenAddr: cfg.ListenAddr,
	})
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("OAuth scopes: %s", client.Scopes())

	bot := application.New(cfg.ChannelID, client, log.Default())
	if err := bot.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatal(err)
	}
}
