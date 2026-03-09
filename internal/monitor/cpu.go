package monitor

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strconv"
	"strings"
)

// CPUStatus represents the current state of CPU usage
type CPUStatus struct {
	UsagePercent float64 `json:"usage_percent"`
	UserPercent  float64 `json:"user_percent"`
	SysPercent   float64 `json:"sys_percent"`
	IdlePercent  float64 `json:"idle_percent"`
	IsHigh       bool    `json:"is_high"`
	LoadAvg      string  `json:"load_avg"`
}

// GetCPUStatus reads CPU usage using the `top` command and parsing the "Idle" value
func GetCPUStatus(ctx context.Context, thresholdPercent float64) (stat *CPUStatus, err error) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("Recovered panic in GetCPUStatus", "panic", r)
			err = fmt.Errorf("N/A (Panic)")
		}
	}()

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	status := &CPUStatus{
		LoadAvg: "N/A",
	}

	// 1. Get Load Average using uptime
	uptimeCmd := exec.CommandContext(ctx, "/system/bin/sh", "-c", "uptime")
	if uptimeOut, uErr := uptimeCmd.CombinedOutput(); uErr == nil {
		outStr := strings.TrimSpace(string(uptimeOut))
		idx := strings.Index(string(outStr), "load average:")
		if idx != -1 {
			status.LoadAvg = strings.TrimSpace(outStr[idx+len("load average:"):])
		}
	}

	// 2. Get CPU usage via top
	topCmd := exec.CommandContext(ctx, "/system/bin/sh", "-c", "top -n 1 -b")
	topOut, topErr := topCmd.CombinedOutput()
	if topErr != nil {
		slog.Warn("Failed to execute top command", "error", topErr, "output", string(topOut))
		return nil, fmt.Errorf("N/A")
	}

	lines := strings.Split(string(topOut), "\n")
	found := false
	for _, line := range lines {
		lowerLine := strings.ToLower(line)
		
		if strings.Contains(lowerLine, "%cpu") || strings.Contains(lowerLine, "idle") || strings.Contains(lowerLine, "id") {
			// clean "800%cpu 0%user" -> "800 cpu 0 user"
			cleanLine := strings.ReplaceAll(lowerLine, "%", " ")
			cleanLine = strings.ReplaceAll(cleanLine, ",", " ")
			fields := strings.Fields(cleanLine)

			var maxCpu float64 = 100.0
			var userVal, sysVal, idleVal float64

			for i, word := range fields {
				if word == "cpu" && i > 0 {
					if v, errVal := strconv.ParseFloat(fields[i-1], 64); errVal == nil {
						if v > 100.0 {
							maxCpu = v
						}
					}
				}
				if (word == "user" || word == "us") && i > 0 {
					if v, errVal := strconv.ParseFloat(fields[i-1], 64); errVal == nil {
						userVal = v
					}
				}
				if (word == "sys" || word == "sy") && i > 0 {
					if v, errVal := strconv.ParseFloat(fields[i-1], 64); errVal == nil {
						sysVal = v
					}
				}
				if (word == "idle" || word == "id") && i > 0 {
					if v, errVal := strconv.ParseFloat(fields[i-1], 64); errVal == nil {
						idleVal = v
						found = true
					}
				}
			}

			if found {
				status.UserPercent = (userVal / maxCpu) * 100.0
				status.SysPercent = (sysVal / maxCpu) * 100.0
				status.IdlePercent = (idleVal / maxCpu) * 100.0
				
				actualUsage := maxCpu - idleVal
				if actualUsage < 0 {
					actualUsage = 0
				}
				
				status.UsagePercent = (actualUsage / maxCpu) * 100.0
				status.IsHigh = status.UsagePercent >= thresholdPercent
				break
			}
		}
	}

	if !found {
		slog.Warn("Failed to parse CPU values from top output")
		return nil, fmt.Errorf("N/A")
	}

	return status, nil
}
