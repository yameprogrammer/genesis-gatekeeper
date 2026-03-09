package monitor

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/yameprogrammer/genesis-gatekeeper/internal/config"
)

// BotInterface enables monitor to call Bot's SendAlert without cyclical imports
type BotInterface interface {
	SendAlert(message string)
}

// StartAll begins the polling loop for all hardware checks
func StartAll(ctx context.Context, cfg *config.Config, bot BotInterface) {
	interval := time.Duration(cfg.Monitor.IntervalSeconds) * time.Second
	ticker := time.NewTicker(interval)

	slog.Info("Monitor Orchestrator initialized", "interval_seconds", cfg.Monitor.IntervalSeconds)

	go func() {
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				slog.Info("Monitor Orchestrator shutting down")
				return
			case <-ticker.C:
				runChecksAndAlert(ctx, cfg.Monitor.Thresholds, bot)
			}
		}
	}()
}

func runChecksAndAlert(ctx context.Context, thresholds config.ThresholdConfig, bot BotInterface) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("Recovered panic in runChecksAndAlert", "panic", r)
		}
	}()

	// CPU Check
	if cpuStat, err := GetCPUStatus(ctx, thresholds.CPUPercent); err == nil {
		if cpuStat.IsHigh {
			msg := fmt.Sprintf("⚠️ <b>High CPU Alert:</b> %.1f%% (>%.1f%%) [u:%.1f, s:%.1f, i:%.1f]\nLoad: %s", 
				cpuStat.UsagePercent, 
				thresholds.CPUPercent, 
				cpuStat.UserPercent, 
				cpuStat.SysPercent, 
				cpuStat.IdlePercent,
				cpuStat.LoadAvg)
			slog.Warn("Threshold breached", "metric", "CPU", "value", cpuStat.UsagePercent)
			bot.SendAlert(msg)
		}
	} else {
		slog.Error("Failed CPU check in poller", "error", err)
	}

	// Memory Check
	if memStat, err := GetMemStatus(ctx, thresholds.MemPercent); err == nil {
		if memStat.IsHigh {
			msg := fmt.Sprintf("⚠️ <b>High Memory Alert:</b> %.1f%% (>%.1f%%)\n(%dMB/%dMB, Avail:%dMB)", 
				memStat.UsagePercent, thresholds.MemPercent, memStat.UsedMB, memStat.TotalMB, memStat.AvailableMB)
			slog.Warn("Threshold breached", "metric", "Memory", "value", memStat.UsagePercent)
			bot.SendAlert(msg)
		}
	} else {
		slog.Error("Failed Memory check in poller", "error", err)
	}

	// Battery Check
	if batStat, err := GetBatteryStatus(ctx, thresholds.BatteryLow); err == nil {
		if batStat.IsLow && batStat.Status != "CHARGING" {
			msg := fmt.Sprintf("⚠️ <b>Low Battery Alert:</b> %d%% (<%d%%)\nTemp: %.1f°C, Status: %s", 
				batStat.Percentage, thresholds.BatteryLow, batStat.Temperature, batStat.Status)
			slog.Warn("Threshold breached", "metric", "Battery", "value", batStat.Percentage)
			bot.SendAlert(msg)
		}
	} else {
		slog.Error("Failed Battery check in poller", "error", err)
	}

	// Network Latency Check
	if netStat, err := GetNetStatus(ctx, thresholds.LatencyHighMs); err == nil {
		if netStat.IsHigh {
			msg := fmt.Sprintf("⚠️ <b>High Network Latency:</b> %dms (>%dms)", 
				netStat.LatencyMs, thresholds.LatencyHighMs)
			slog.Warn("Threshold breached", "metric", "Network", "value", netStat.LatencyMs)
			bot.SendAlert(msg)
		}
	} else {
		slog.Error("Failed Network check in poller", "error", err)
	}
}
