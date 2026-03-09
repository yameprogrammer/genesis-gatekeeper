package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"
)

// Config represents the full configuration of the Genesis-Gatekeeper application.
type Config struct {
	Telegram TelegramConfig `yaml:"telegram"`
	Monitor  MonitorConfig  `yaml:"monitor"`
}

// TelegramConfig holds configurations related to the Telegram Bot API.
type TelegramConfig struct {
	BotToken  string  `yaml:"bot_token"`
	Whitelist []int64 `yaml:"whitelist"` // List of allowed Telegram User IDs
}

// MonitorConfig holds interval and threshold settings.
type MonitorConfig struct {
	IntervalSeconds int `yaml:"interval_seconds"`
	Thresholds      ThresholdConfig `yaml:"thresholds"`
}

// ThresholdConfig holds the alert thresholds.
type ThresholdConfig struct {
	CPUPercent    float64 `yaml:"cpu_percent"`
	MemPercent    float64 `yaml:"mem_percent"`
	BatteryLow    int     `yaml:"battery_low"`     // e.g., 20 for 20%
	LatencyHighMs int     `yaml:"latency_high_ms"` // e.g., 500 for 500ms
}

// Default values
func DefaultConfig() *Config {
	return &Config{
		Telegram: TelegramConfig{
			BotToken:  "",
			Whitelist: []int64{},
		},
		Monitor: MonitorConfig{
			IntervalSeconds: 60,
			Thresholds: ThresholdConfig{
				CPUPercent:    90.0,
				MemPercent:    90.0,
				BatteryLow:    20,
				LatencyHighMs: 500,
			},
		},
	}
}

// Load reads the configuration from a YAML file.
// If the file doesn't exist, it can optionally return default config or an error.
func Load(configPath string) (*Config, error) {
	// 1. Try to load .env file. Ignore error if it doesn't exist, as we fallback to OS env vars.
	_ = godotenv.Load(".env")

	data, err := os.ReadFile(configPath)
	cfg := DefaultConfig()

	if err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
		// If it's a NotExist error, we just continue with default config + env vars
	} else {
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("failed to unmarshal config yaml: %w", err)
		}
	}

	// Environment Variable Override
	if token := os.Getenv("TG_BOT_TOKEN"); token != "" {
		cfg.Telegram.BotToken = token
	}

	// Trim space for the token regardless of whether it came from env or file
	cfg.Telegram.BotToken = strings.TrimSpace(cfg.Telegram.BotToken)

	// Log token preview and length
	tokenLen := len(cfg.Telegram.BotToken)
	if tokenLen > 4 {
		slog.Info("Loaded Bot Token", "length", tokenLen, "preview", cfg.Telegram.BotToken[:4]+"***")
	} else if tokenLen > 0 {
		slog.Info("Loaded Bot Token", "length", tokenLen, "preview", "***")
	} else {
		slog.Warn("Bot Token is empty")
	}

	if whitelistStr := os.Getenv("TG_WHITELIST"); whitelistStr != "" {
		var wl []int64
		for _, part := range strings.Split(whitelistStr, ",") {
			id, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64)
			if err == nil {
				wl = append(wl, id)
			}
		}
		if len(wl) > 0 {
			cfg.Telegram.Whitelist = wl
		}
	}

	// Basic validation
	if cfg.Monitor.IntervalSeconds <= 0 {
		cfg.Monitor.IntervalSeconds = 60
	}

	return cfg, nil
}
