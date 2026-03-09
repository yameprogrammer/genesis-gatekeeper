package monitor

import (
	"context"
	"fmt"
	"net"
	"time"
)

// NetStatus represents the network latency status
type NetStatus struct {
	LatencyMs int  `json:"latency_ms"`
	IsHigh    bool `json:"is_high"`
}

// GetNetStatus checks network latency by performing a TCP handshake to a reliable endpoint (e.g., 8.8.8.8:53).
// We use a TCP handshake instead of raw ICMP ping to avoid needing root/CAP_NET_RAW privileges on Termux.
func GetNetStatus(ctx context.Context, thresholdMs int) (*NetStatus, error) {
	target := "8.8.8.8:53"

	start := time.Now()
	
	// Create a dialer attached to the given context
	dialer := net.Dialer{
		Timeout:   3 * time.Second, // Max time to wait for handshake
	}

	conn, err := dialer.DialContext(ctx, "tcp", target)
	if err != nil {
		return nil, fmt.Errorf("failed TCP handshake to %s: %w", target, err)
	}
	defer conn.Close()

	latency := time.Since(start).Milliseconds()

	return &NetStatus{
		LatencyMs: int(latency),
		IsHigh:    int(latency) >= thresholdMs,
	}, nil
}
