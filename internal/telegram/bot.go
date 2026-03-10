package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/yameprogrammer/genesis-gatekeeper/internal/config"
	tele "gopkg.in/telebot.v3"
)

// Bot wraps the telebot instance and config
type Bot struct {
	api    *tele.Bot
	config *config.Config
}

// NewBot creates and configures a new Telegram bot instance
func NewBot(cfg *config.Config) (*Bot, error) {
	if cfg.Telegram.BotToken == "" {
		return nil, fmt.Errorf("bot token is missing in configuration")
	}

	// Custom HTTP Client for Termux DNS issues (prefer IPv4, fallback to 8.8.8.8)
	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	// Custom DNS Resolver
	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			// Force IPv4 and use Google DNS
			d := net.Dialer{
				Timeout: 10 * time.Second,
			}
			return d.DialContext(ctx, "udp4", "8.8.8.8:53")
		},
	}

	dialer.Resolver = resolver

	customTransport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			// Force IPv4 TCP connection
			return dialer.DialContext(ctx, "tcp4", addr)
		},
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	httpClient := &http.Client{
		Transport: customTransport,
		Timeout:   60 * time.Second,
	}

	pref := tele.Settings{
		Token:  cfg.Telegram.BotToken,
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
		Client: httpClient, // Use the custom HTTP client
	}

	api, err := tele.NewBot(pref)
	if err != nil {
		return nil, fmt.Errorf("failed to create telebot instance: %w", err)
	}

	bot := &Bot{
		api:    api,
		config: cfg,
	}

	bot.registerHandlers()

	return bot, nil
}

// Start begins the bot polling loop (blocks)
func (b *Bot) Start() {
	slog.Info("Starting Telegram bot poller...")
	b.api.Start()
}

// Stop stops the bot polling gracefully
func (b *Bot) Stop() {
	slog.Info("Stopping Telegram bot...")
	b.api.Stop()
}

// CheckWhitelist validates if the sender is in the configured Whitelist
func (b *Bot) CheckWhitelist(userID int64) bool {
	// If whitelist is empty, we might choose to allow everyone or no one.
	// For security, an empty whitelist means nobody is allowed unless explicitly configured.
	if len(b.config.Telegram.Whitelist) == 0 {
		return false
	}
	for _, id := range b.config.Telegram.Whitelist {
		if id == userID {
			return true
		}
	}
	return false
}

// SendAlert pushes a message to all whitelisted users using Markdown parsing
func (b *Bot) SendAlert(message string) {
	slog.Warn("Sending markdown alert", "message", message)
	b.broadcastText(message, tele.ModeMarkdown)
}

// SendAdminHTML pushes a message to all whitelisted users using HTML parsing
func (b *Bot) SendAdminHTML(message string) {
	slog.Warn("Sending HTML alert", "message", message)
	b.broadcastText(message, tele.ModeHTML)
}

func (b *Bot) broadcastText(message string, mode string) {
	for _, id := range b.config.Telegram.Whitelist {
		chat := &tele.Chat{ID: id}
		_, err := b.api.Send(chat, message, &tele.SendOptions{ParseMode: mode})
		if err != nil {
			slog.Error("Failed to broadcast message", "user_id", id, "error", err)
		}
	}
}
