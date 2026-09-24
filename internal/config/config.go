package config

import (
	"bufio"
	"fmt"
	"net/url"
	"os"
	"strings"
)

type Config struct {
	ClientID     string
	ClientSecret string
	ChannelID    string
	AuthURL      string
	TokenURL     string
	APIBase      string
	RedirectURI  string
	ListenAddr   string
}

func Load() (Config, error) {
	loadDotEnv(".env")
	cfg := Config{
		ClientID: os.Getenv("PANTHEON_CLIENT_ID"), ClientSecret: os.Getenv("PANTHEON_CLIENT_SECRET"),
		ChannelID:   os.Getenv("PANTHEON_CHANNEL_ID"),
		AuthURL:     env("PANTHEON_AUTH_URL", "https://пантеон.com/api/oauth/authorize"),
		TokenURL:    env("PANTHEON_TOKEN_URL", "https://api.pantheone.ru/oauth/token"),
		APIBase:     strings.TrimRight(env("PANTHEON_API_BASE", "https://api.pantheone.ru"), "/"),
		RedirectURI: env("PANTHEON_REDIRECT_URI", "http://localhost:1230/callback"),
		ListenAddr:  env("LISTEN_ADDR", "localhost:1230"),
	}
	if cfg.ClientID == "" || cfg.ChannelID == "" {
		return Config{}, fmt.Errorf("PANTHEON_CLIENT_ID and PANTHEON_CHANNEL_ID are required")
	}
	if _, err := url.ParseRequestURI(cfg.RedirectURI); err != nil {
		return Config{}, fmt.Errorf("invalid PANTHEON_REDIRECT_URI: %w", err)
	}
	return cfg, nil
}

func env(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func loadDotEnv(path string) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		name := strings.TrimSpace(parts[0])
		if _, exists := os.LookupEnv(name); exists {
			continue
		}
		_ = os.Setenv(name, strings.Trim(strings.TrimSpace(parts[1]), "\"'"))
	}
}
