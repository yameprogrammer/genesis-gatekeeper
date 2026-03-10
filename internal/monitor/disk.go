package monitor

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

// DiskStatus represents the usage metrics of the main partition (e.g. /data)
type DiskStatus struct {
	TotalMB uint64 `json:"total_mb"`
	UsedMB  uint64 `json:"used_mb"`
	FreeMB  uint64 `json:"free_mb"`
	UsagePercent float64 `json:"usage_percent"`
}

// GetDiskStatus fetches disk usage using df -h and falls back to syscall.Statfs
func GetDiskStatus(ctx context.Context) (*DiskStatus, error) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("Recovered panic in GetDiskStatus", "panic", r)
		}
	}()

	status := &DiskStatus{}
	
	// 1. Try shell wrapper first
	cmd := exec.CommandContext(ctx, "/system/bin/sh", "-c", "df -m /data | grep /data | awk '{print $2, $3, $4, $5}'")
	out, err := cmd.CombinedOutput()
	
	if err == nil {
		fields := strings.Fields(strings.TrimSpace(string(out)))
		if len(fields) >= 4 {
			total, _ := strconv.ParseUint(fields[0], 10, 64)
			used, _ := strconv.ParseUint(fields[1], 10, 64)
			free, _ := strconv.ParseUint(fields[2], 10, 64)
			percStr := strings.TrimRight(fields[3], "%")
			perc, _ := strconv.ParseFloat(percStr, 64)
			
			status.TotalMB = total
			status.UsedMB = used
			status.FreeMB = free
			status.UsagePercent = perc
			return status, nil
		}
	}

	slog.Warn("Shell wrapper for disk failed, falling back to syscall.Statfs", "error", err)

	// 2. Fallback to syscall.Statfs preventing complete failure
	var stat syscall.Statfs_t
	if err := syscall.Statfs("/data", &stat); err != nil {
		return nil, fmt.Errorf("statfs failed on /data: %w", err)
	}

	// Calculate in MB
	blockSize := uint64(stat.Bsize)
	status.TotalMB = (stat.Blocks * blockSize) / (1024 * 1024)
	status.FreeMB = (stat.Bavail * blockSize) / (1024 * 1024)
	
	if status.TotalMB > status.FreeMB {
		status.UsedMB = status.TotalMB - status.FreeMB
		status.UsagePercent = (float64(status.UsedMB) / float64(status.TotalMB)) * 100.0
	}

	return status, nil
}
