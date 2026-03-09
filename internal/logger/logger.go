package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

const (
	logDir       = "logs"
	retentionDays = 30
)

// Init sets up the JSON structured logger writing to a daily rotated file in the logs directory.
// Instead of complex rotate libraries, we'll write to a current app.log or daily log and use retention worker.
// Returning the io.Closer so main can defer close it.
func Init(moduleName string, stdOutFallback bool) (io.Closer, error) {
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log dir: %w", err)
	}

	// For simplicity, writing to a single app.log file.
	// Production systems often append to YYYY-MM-DD.log.
	logPath := filepath.Join(logDir, fmt.Sprintf("%s.log", moduleName))
	
	file, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	var writer io.Writer = file
	if stdOutFallback {
		writer = io.MultiWriter(os.Stdout, file)
	}

	handler := slog.NewJSONHandler(writer, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	
	logger := slog.New(handler)
	slog.SetDefault(logger)

	return file, nil
}

// StartRetentionWorker runs periodically to delete logs older than retentionDays (30 days)
func StartRetentionWorker(ctx context.Context) {
	ticker := time.NewTicker(24 * time.Hour) // Run once a day
	go func() {
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				slog.Info("Log retention worker shutting down")
				return
			case <-ticker.C:
				cleanupOldLogs()
			}
		}
	}()
	// Also run once immediately at startup
	go cleanupOldLogs()
}

func cleanupOldLogs() {
	entries, err := os.ReadDir(logDir)
	if err != nil {
		slog.Error("Failed to read log directory for cleanup", "error", err)
		return
	}

	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	deletedCount := 0

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			slog.Error("Failed to get log file info", "file", entry.Name(), "error", err)
			continue
		}

		// If modified time is before the cutoff of 30 days ago
		if info.ModTime().Before(cutoff) {
			path := filepath.Join(logDir, entry.Name())
			if err := os.Remove(path); err != nil {
				slog.Error("Failed to delete old log file", "file", path, "error", err)
			} else {
				deletedCount++
			}
		}
	}

	if deletedCount > 0 {
		slog.Info("Completed log cleanup", "deleted_files", deletedCount)
	}
}
