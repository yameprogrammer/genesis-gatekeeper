package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

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

// registerHandlers sets up middleweares and routes
func (b *Bot) registerHandlers() {
	// Middleware for Whitelist verification
	b.api.Use(func(next tele.HandlerFunc) tele.HandlerFunc {
		return func(c tele.Context) error {
			senderID := c.Sender().ID
			if !b.CheckWhitelist(senderID) {
				slog.Warn("Unauthorized access attempt", "user_id", senderID, "username", c.Sender().Username)
				// We can optionally ignore them entirely or send an Access Denied message
				return c.Send("Access Denied: You are not authorized to use this bot.")
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

		return c.Send(statusMsg, &tele.SendOptions{ParseMode: tele.ModeHTML})
	})
}
