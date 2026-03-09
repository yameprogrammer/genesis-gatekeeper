package monitor

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strconv"
	"strings"
)

// MemStatus represents the current state of Memory usage
type MemStatus struct {
	UsagePercent float64 `json:"usage_percent"`
	IsHigh       bool    `json:"is_high"`
	TotalMB      uint64  `json:"total_mb"`
	UsedMB       uint64  `json:"used_mb"`
	FreeMB       uint64  `json:"free_mb"`
	AvailableMB  uint64  `json:"available_mb"`
}

// GetMemStatus tries to read memory info using `free -m`.
func GetMemStatus(ctx context.Context, thresholdPercent float64) (stat *MemStatus, err error) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("Recovered panic in GetMemStatus", "panic", r)
			err = fmt.Errorf("N/A (Panic)")
		}
	}()

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	cmd := exec.CommandContext(ctx, "/system/bin/sh", "-c", "free -m")
	out, cmdErr := cmd.CombinedOutput()
	if cmdErr != nil {
		slog.Warn("Failed to execute free via sh -c", "error", cmdErr, "output", string(out))
		return nil, fmt.Errorf("Permission Denied (SIGSYS)")
	}

	status := &MemStatus{}
	lines := strings.Split(string(out), "\n")
	found := false

	for _, line := range lines {
		lowerLine := strings.ToLower(line)
		if strings.HasPrefix(lowerLine, "mem:") {
			fields := strings.Fields(line)
			if len(fields) >= 4 {
				// Mem:  total   used   free  shared  buffers cached
				total, _ := strconv.ParseUint(fields[1], 10, 64)
				used, _ := strconv.ParseUint(fields[2], 10, 64)
				free, _ := strconv.ParseUint(fields[3], 10, 64)
				
				status.TotalMB = total
				status.UsedMB = used
				status.FreeMB = free
				
				if len(fields) >= 7 { // Sometimes available is 7th
					avail, errVal := strconv.ParseUint(fields[6], 10, 64)
					if errVal == nil {
						status.AvailableMB = avail
					}
				}
				
				if total > 0 {
					status.UsagePercent = float64(used) / float64(total) * 100.0
					status.IsHigh = status.UsagePercent >= thresholdPercent
					found = true
				}
			}
		} else if strings.HasPrefix(lowerLine, "-/+ buffers/cache:") {
			fields := strings.Fields(line)
			if len(fields) >= 4 {
				used, _ := strconv.ParseUint(fields[2], 10, 64)
				status.UsedMB = used
				if status.TotalMB > 0 {
					status.UsagePercent = float64(used) / float64(status.TotalMB) * 100.0
				}
			}
		}
	}

	if !found {
		slog.Warn("Failed to parse free command output")
		return nil, fmt.Errorf("N/A (Parsing Failed)")
	}

	if status.AvailableMB == 0 {
		status.AvailableMB = status.FreeMB
	}

	return status, nil
}
