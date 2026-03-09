package monitor

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
)

// BatteryStatus represents the battery level and charging state
type BatteryStatus struct {
	Percentage  int     `json:"percentage"`
	Status      string  `json:"status"` // E.g., "CHARGING", "DISCHARGING", "FULL", etc
	Temperature float64 `json:"temperature"`
	IsLow       bool    `json:"is_low"`
}

// termuxBatteryOutput is the JSON structure returned by `termux-battery-status`
type termuxBatteryOutput struct {
	Health      string  `json:"health"`
	Percentage  int     `json:"percentage"`
	Plugged     string  `json:"plugged"`
	Status      string  `json:"status"`
	Temperature float64 `json:"temperature"`
	Current     int     `json:"current"`
}

// GetBatteryStatus tries to get battery info using an explicit shell invocation
func GetBatteryStatus(ctx context.Context, thresholdLow int) (stat *BatteryStatus, err error) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("Recovered panic in GetBatteryStatus", "panic", r)
			err = fmt.Errorf("N/A (Panic)")
		}
	}()

	cmd := exec.CommandContext(ctx, "/system/bin/sh", "-c", "termux-battery-status")
	out, errCmd := cmd.CombinedOutput()
	if errCmd != nil {
		slog.Warn("Failed to execute termux-battery-status via sh -c", "error", errCmd, "output", string(out))
		return nil, fmt.Errorf("Permission Denied (SIGSYS)")
	}

	var tOutput termuxBatteryOutput
	if unmarshalErr := json.Unmarshal(out, &tOutput); unmarshalErr != nil {
		slog.Warn("Failed to parse termux-battery-status output", "error", unmarshalErr)
		return nil, fmt.Errorf("N/A (Parse Error)")
	}

	return &BatteryStatus{
		Percentage:  tOutput.Percentage,
		Status:      tOutput.Status,
		Temperature: tOutput.Temperature,
		IsLow:       tOutput.Percentage <= thresholdLow,
	}, nil
}
