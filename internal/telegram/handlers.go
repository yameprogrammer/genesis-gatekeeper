package telegram

import (
	"context"
	"fmt"
	"html"
	"log/slog"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/yameprogrammer/genesis-gatekeeper/internal/logger"
	"github.com/yameprogrammer/genesis-gatekeeper/internal/monitor"
	tele "gopkg.in/telebot.v3"
)

func formatWithCommas(n uint64) string {
	in := strconv.FormatUint(n, 10)
	var out []string
	for i := len(in); i > 0; i -= 3 {
		start := i - 3
		if start < 0 {
			start = 0
		}
		out = append([]string{in[start:i]}, out...)
	}
	return strings.Join(out, ",")
}

func drawProgressBar(percent float64, length int) string {
	filledCount := int((percent / 100.0) * float64(length))
	if filledCount < 0 { filledCount = 0 }
	if filledCount > length { filledCount = length }
	emptyCount := length - filledCount

	return fmt.Sprintf("[%s%s]", strings.Repeat("■", filledCount), strings.Repeat("□", emptyCount))
}

// Rate Limiter for Unauthorized Attempts
type attemptRecord struct {
	count     int
	firstSeen time.Time
}

var (
	unauthAttempts sync.Map
	rateLimitMut   sync.Mutex
)

// init starts a background goroutine to clean up old rate limit entries to prevent memory leaks
func init() {
	go func() {
		for {
			time.Sleep(10 * time.Minute)
			now := time.Now()
			unauthAttempts.Range(func(key, value interface{}) bool {
				record := value.(*attemptRecord)
				if now.Sub(record.firstSeen) > 5*time.Minute {
					unauthAttempts.Delete(key)
				}
				return true
			})
		}
	}()
}

// checkRateLimit returns true if the user exceeded the rate limit (5 tries / min)
func checkRateLimit(userID int64) bool {
	rateLimitMut.Lock()
	defer rateLimitMut.Unlock()

	now := time.Now()
	val, exists := unauthAttempts.Load(userID)
	
	if !exists {
		unauthAttempts.Store(userID, &attemptRecord{count: 1, firstSeen: now})
		return false
	}

	record := val.(*attemptRecord)
	
	// Reset if more than 1 minute has passed
	if now.Sub(record.firstSeen) > time.Minute {
		record.count = 1
		record.firstSeen = now
		return false
	}

	record.count++
	if record.count > 5 {
		return true // rate limited
	}

	return false
}

// registerHandlers sets up middleweares and routes
func (b *Bot) registerHandlers() {
	// Middleware for Whitelist verification
	b.api.Use(func(next tele.HandlerFunc) tele.HandlerFunc {
		return func(c tele.Context) error {
			senderID := c.Sender().ID
			if !b.CheckWhitelist(senderID) {
				// 1. Rate Limiter (Anti-DoS)
				if checkRateLimit(senderID) {
					slog.Warn("Rate limit exceeded for unauthorized user, dropping silently", "user_id", senderID)
					// Anti-Reconnaissance Jitter: 1~3 seconds random sleep
					jitterDuration := time.Duration(1000 + rand.Intn(2000)) * time.Millisecond
					time.Sleep(jitterDuration)
					return nil // Silent Drop
				}

				// 2. Input Sanitization
				sanitizedText := html.EscapeString(c.Text())

				// 3. Auth Logger & Silent Drop
				slog.Warn("Unauthorized access attempt", "user_id", senderID, "username", c.Sender().Username, "text", sanitizedText)
				logger.LogUnauthorized(c.Sender(), sanitizedText)
				
				// Silent Drop: we log it but don't send any reply back.
				return nil
			}
			return next(c)
		}
	})

	// /help command
	b.api.Handle("/help", func(c tele.Context) error {
		helpText := `🤖 <b>Genesis-Gatekeeper Menu</b>

/status - Get current device metrics (CPU, Mem, Bat, Ping)
/help - Show this menu`
		return c.Send(helpText, &tele.SendOptions{ParseMode: tele.ModeHTML})
	})

	// /status command
	b.api.Handle("/status", func(c tele.Context) error {
		slog.Info("Handling /status request from user", "user_id", c.Sender().ID)

		ctx, cancel := context.WithTimeout(context.Background(), 7*time.Second)
		defer cancel()

		var statusMsg string
		
		// CPU Status
		cpuStat, err := monitor.GetCPUStatus(ctx, b.config.Monitor.Thresholds.CPUPercent)
		if err != nil {
			statusMsg += "💻 <b>CPU:</b> N/A\n"
			statusMsg += "📈 <b>Load:</b> N/A\n"
		} else {
			statusMsg += fmt.Sprintf("💻 <b>CPU:</b> %.1f%% (u:%.1f, s:%.1f, i:%.1f)\n", 
				cpuStat.UsagePercent, cpuStat.UserPercent, cpuStat.SysPercent, cpuStat.IdlePercent)
			statusMsg += fmt.Sprintf("📈 <b>Load:</b> %s\n", cpuStat.LoadAvg)
		}

		// Mem Status
		memStat, err := monitor.GetMemStatus(ctx, b.config.Monitor.Thresholds.MemPercent)
		if err != nil {
			statusMsg += "🧠 <b>Mem:</b> N/A\n"
			statusMsg += "💡 <b>Avail:</b> N/A\n"
		} else {
			bar := drawProgressBar(memStat.UsagePercent, 5)
			statusMsg += fmt.Sprintf("🧠 <b>Mem:</b> %s %.0f%% (%sMB / %sMB)\n", 
				bar, memStat.UsagePercent, formatWithCommas(memStat.UsedMB), formatWithCommas(memStat.TotalMB))
			statusMsg += fmt.Sprintf("💡 <b>Avail:</b> %sMB\n", formatWithCommas(memStat.AvailableMB))
		}

		// Battery Status
		batStat, err := monitor.GetBatteryStatus(ctx, b.config.Monitor.Thresholds.BatteryLow)
		if err != nil {
			statusMsg += "🔋 <b>Bat:</b> N/A\n"
		} else {
			statusMsg += fmt.Sprintf("🔋 <b>Bat:</b> %d%% (%.1f°C, %s)\n", batStat.Percentage, batStat.Temperature, batStat.Status)
		}

		// Network Status
		netStat, err := monitor.GetNetStatus(ctx, b.config.Monitor.Thresholds.LatencyHighMs)
		if err != nil {
			statusMsg += "🌐 <b>Net:</b> N/A\n"
		} else {
			statusMsg += fmt.Sprintf("🌐 <b>Net:</b> %dms ping\n", netStat.LatencyMs)
		}

		// Disk Status
		diskStat, err := monitor.GetDiskStatus(ctx)
		if err != nil {
			statusMsg += "💽 <b>Disk:</b> N/A\n"
		} else {
			bar := drawProgressBar(diskStat.UsagePercent, 5)
			statusMsg += fmt.Sprintf("💽 <b>Disk:</b> %s %.0f%% (%sMB / %sMB)\n",
				bar, diskStat.UsagePercent, formatWithCommas(diskStat.UsedMB), formatWithCommas(diskStat.TotalMB))
			statusMsg += fmt.Sprintf("🔓 <b>Free:</b> %sMB\n", formatWithCommas(diskStat.FreeMB))
		}

		return c.Send(statusMsg, &tele.SendOptions{ParseMode: tele.ModeHTML})
	})
}
