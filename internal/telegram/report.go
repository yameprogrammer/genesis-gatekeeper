package telegram

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/yameprogrammer/genesis-gatekeeper/internal/logger"
)

var securityCron *cron.Cron

// StartSecurityCron initializes and starts the daily summary report scheduler.
func StartSecurityCron(b *Bot) {
	// Schedule for KST (UTC+9)
	kst := time.FixedZone("KST", 9*3600)
	securityCron = cron.New(cron.WithLocation(kst))

	// 0 9 * * * = Every day at 09:00 KST
	_, err := securityCron.AddFunc("0 9 * * *", func() {
		slog.Info("Cron triggered: Generating Daily Security Report")
		report := GenerateSecurityReport()
		b.SendAdminHTML(report)
	})

	if err != nil {
		slog.Error("Failed to add security cron job", "error", err)
		return
	}

	securityCron.Start()
	slog.Info("Security Cron Scheduler started (Daily at 09:00 KST)")
}

// StopSecurityCron gracefully stops the cron scheduler.
func StopSecurityCron() {
	if securityCron != nil {
		ctx := securityCron.Stop()
		<-ctx.Done()
		slog.Info("Security Cron Scheduler stopped.")
	}
}

// GenerateSecurityReport fetches yesterday's stats and formats them in HTML.
func GenerateSecurityReport() string {
	kst := time.FixedZone("KST", 9*3600)

	// Normally a daily report generated at 09:00 covers the last 24H (yesterday).
	// For simplicity, we query the stats for "yesterday" relative to the generation time.
	yesterday := time.Now().In(kst).AddDate(0, 0, -1)

	stats, err := logger.GetDailyStats(yesterday)
	if err != nil {
		slog.Error("Failed to fetch daily stats for report", "error", err)
		return fmt.Sprintf("<b>🛡️ Security Daily Briefing</b>\n⚠️ Error generating report: %v", err)
	}

	dateStr := yesterday.Format("2006-01-02")

	report := "<b>🛡️ Security Daily Briefing</b>\n"
	report += "--------------------------\n"
	report += fmt.Sprintf("📅 <b>Date:</b> %s\n", dateStr)
	report += fmt.Sprintf("⚠️ <b>Unauthorized Attempts:</b> %d\n\n", stats.TotalAttempts)

	if stats.TotalAttempts > 0 {
		report += "🚨 <b>Top Intruder IDs:</b>\n"
		for i, intruder := range stats.TopIntruders {
			report += fmt.Sprintf("  %d. %s\n", i+1, intruder)
		}
	} else {
		report += "✅ No unauthorized access attempts detected.\n"
	}

	return report
}
