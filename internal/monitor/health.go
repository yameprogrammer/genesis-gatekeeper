package monitor

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/yameprogrammer/genesis-gatekeeper/internal/config"
)

// BotHTMLInterface enables monitor to call Bot's SendAdminHTML to send HTML alerts
type BotHTMLInterface interface {
	SendAdminHTML(message string)
}

// Global cooldown map to suppress duplicate alerts within 30 minutes
var (
	alertCooldowns sync.Map
)

// isCooldownActive checks if an alert for a specific key is currently active (within 30m)
func isCooldownActive(alertKey string) bool {
	if val, exists := alertCooldowns.Load(alertKey); exists {
		lastAlertTime := val.(time.Time)
		if time.Since(lastAlertTime) < 30*time.Minute {
			return true // Cooldown is still active
		}
	}
	return false
}

// setCooldown records the time when an alert was sent
func setCooldown(alertKey string) {
	alertCooldowns.Store(alertKey, time.Now())
}

// StartHealthMonitor begins the 5-minute polling loop for continuous health checks
func StartHealthMonitor(ctx context.Context, cfg *config.Config, bot BotHTMLInterface) {
	// 5-minute polling interval
	ticker := time.NewTicker(5 * time.Minute)

	slog.Info("Health Monitor initialized (5m polling interval)")

	go func() {
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				slog.Info("Health Monitor shutting down")
				return
			case <-ticker.C:
				runHealthChecksAndAlert(ctx, cfg.Monitor.Thresholds, bot)
			}
		}
	}()
}

func runHealthChecksAndAlert(ctx context.Context, thresholds config.ThresholdConfig, bot BotHTMLInterface) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("Recovered panic in runHealthChecksAndAlert", "panic", r)
		}
	}()

	// 1. Battery Check (High Temp & Low Power)
	if batStat, err := GetBatteryStatus(ctx, thresholds.BatteryLow); err == nil {
		// Battery Temp > 45°C
		if batStat.Temperature > 45.0 {
			if !isCooldownActive("battery_temp") {
				msg := fmt.Sprintf("🔥 <b>CRITICAL: High Battery Temperature</b>\nTemp is %.1f°C (&gt; 45°C)!\nHardware protection advised.", batStat.Temperature)
				slog.Warn("Thermal threshold breached", "temp", batStat.Temperature)
				bot.SendAdminHTML(msg)
				setCooldown("battery_temp")
			}
		}

		// Battery Low: Discharging AND < 20%
		if batStat.Status == "DISCHARGING" && batStat.Percentage < 20 {
			if !isCooldownActive("battery_low") {
				msg := fmt.Sprintf("⚠️ <b>Battery Low Alert</b>\nBattery is discharging and critically low: %d%%.\nTemp: %.1f°C", batStat.Percentage, batStat.Temperature)
				slog.Warn("Low battery threshold breached", "percentage", batStat.Percentage)
				bot.SendAdminHTML(msg)
				setCooldown("battery_low")
			}
		}
	} else {
		slog.Error("Failed Battery check in health monitor", "error", err)
	}

	// 2. CPU Check (High CPU Usage > 85%)
	if cpuStat, err := GetCPUStatus(ctx, 85.0); err == nil {
		if cpuStat.UsagePercent > 85.0 {
			if !isCooldownActive("cpu_high") {
				msg := fmt.Sprintf("🔥 <b>High CPU Alert</b>\nUsage is %.1f%% (&gt; 85%%)\nLoad Average: %s", cpuStat.UsagePercent, cpuStat.LoadAvg)
				slog.Warn("CPU threshold breached", "usage", cpuStat.UsagePercent)
				bot.SendAdminHTML(msg)
				setCooldown("cpu_high")
			}
		}
	} else {
		slog.Error("Failed CPU check in health monitor", "error", err)
	}

	// 3. Memory Check (Low Avail Mem < 200MB)
	if memStat, err := GetMemStatus(ctx, 0); err == nil {
		if memStat.AvailableMB < 200 {
			if !isCooldownActive("mem_low") {
				msg := fmt.Sprintf("⚠️ <b>Low Available Memory Alert</b>\nOnly %dMB memory available (&lt; 200MB limit).\nTotal: %dMB, Used: %dMB", memStat.AvailableMB, memStat.TotalMB, memStat.UsedMB)
				slog.Warn("Memory threshold breached", "available_mb", memStat.AvailableMB)
				bot.SendAdminHTML(msg)
				setCooldown("mem_low")
			}
		}
	} else {
		slog.Error("Failed Memory check in health monitor", "error", err)
	}

	// 4. Disk Check (High Disk Usage > 90%)
	if diskStat, err := GetDiskStatus(ctx); err == nil {
		if diskStat.UsagePercent > 90.0 {
			if !isCooldownActive("disk_high") {
				msg := fmt.Sprintf("💽 <b>High Disk Usage Alert</b>\nUsage is %.1f%% (&gt; 90%% limit).\nTotal: %dMB, Used: %dMB", diskStat.UsagePercent, diskStat.TotalMB, diskStat.UsedMB)
				slog.Warn("Disk threshold breached", "usage_percent", diskStat.UsagePercent)
				bot.SendAdminHTML(msg)
				setCooldown("disk_high")
			}
		}
	} else {
		slog.Error("Failed Disk check in health monitor", "error", err)
	}
}
