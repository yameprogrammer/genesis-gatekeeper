package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	"time"

	"github.com/yameprogrammer/genesis-gatekeeper/internal/config"
	"github.com/yameprogrammer/genesis-gatekeeper/internal/logger"
	"github.com/yameprogrammer/genesis-gatekeeper/internal/monitor"
	"github.com/yameprogrammer/genesis-gatekeeper/internal/telegram"
)

const pidFile = "genesis.pid"

func writePID() error {
	pid := os.Getpid()
	return os.WriteFile(pidFile, []byte(fmt.Sprintf("%d", pid)), 0644)
}

func removePID() {
	if err := os.Remove(pidFile); err != nil && !os.IsNotExist(err) {
		slog.Error("Failed to remove PID file", "error", err)
	}
}

func main() {
	// Panic Recovery for the main startup sequence
	defer func() {
		if r := recover(); r != nil {
			slog.Error("Genesis-Gatekeeper encountered a fatal panic!", "panic", r, "stack", string(debug.Stack()))
			removePID()
			os.Exit(1)
		}
	}()

	// 1. Initialize Logger
	logCloser, err := logger.Init("genesis", true) // moduleName "genesis", stdOutFallback "true"
	if err != nil { 
		fmt.Printf("Failed to initialize logger: %v\n", err)
		os.Exit(1) 
	}
	if logCloser != nil {
		defer logCloser.Close()
	}
	slog.Info("Starting Genesis-Gatekeeper...")

	// 1.5. Initialize Auth Logger
	if err := logger.InitAuthLogger(); err != nil {
		slog.Error("Failed to initialize Auth Logger", "error", err)
	} else {
		defer logger.CloseAuthLogger()
	}


	// 2. Daemon PID file creation
	if err := writePID(); err != nil {
		slog.Error("Failed to create PID file", "error", err)
		os.Exit(1)
	}
	defer removePID()

	// 3. Load Configuration
	cfg, err := config.Load("config.yaml")
	if err != nil {
		slog.Error("Failed to load config.yaml (attempting to use defaults or error)", "error", err)
		// We can still continue with defaults if Load returned them
		if cfg == nil {
			os.Exit(1)
		}
	}
	slog.Info("Configuration loaded successfully")

	// 4. Setup Context for Graceful Shutdown and Daemon signals
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	go func() {
		for sig := range sigCh {
			switch sig {
			case syscall.SIGHUP:
				slog.Info("Received SIGHUP, ignoring to prevent kill (or reload config here)...")
			case syscall.SIGINT, syscall.SIGTERM, os.Interrupt:
				slog.Info("Received shutdown signal, initiating graceful shutdown...", "signal", sig)
				cancel()
				return
			}
		}
	}()

	// 5. Initialize Telegram Bot
	bot, err := telegram.NewBot(cfg)
	if err != nil { 
		slog.Error("Failed to initialize telegram bot", "error", err) 
		os.Exit(1) 
	}
	slog.Info("Telegram bot initialized")

	go bot.Start()
	
	// 6. Start Tracking & Health Monitoring Routines
	monitor.StartHealthMonitor(ctx, cfg, bot)
	slog.Info("Health Monitor started")

	// 6.5 Start Security Report Cron
	telegram.StartSecurityCron(bot)

	// 7. Start Log Retention Worker
	logger.StartRetentionWorker(ctx)
	slog.Info("Log retention worker started")

	slog.Info("Genesis-Gatekeeper is running. Press Ctrl+C to stop.")

	// Wait for shutdown signal
	<-ctx.Done()

	// Graceful Shutdown Sequence
	slog.Info("Shutting down services...")
	
	_, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	telegram.StopSecurityCron()
	bot.Stop()
	
	slog.Info("Genesis-Gatekeeper stopped gracefully.")
	fmt.Println("Shutdown complete.")
}
