package network

import (
	"context"
	"os/exec"
	"runtime"
	"time"
)

const (
	// PingAttempts is the number of ping attempts before declaring failure
	PingAttempts = 3
	// PingTimeout is the timeout for each ping attempt
	PingTimeout = 2 * time.Second
)

// PingHost checks if a host is reachable using ICMP ping
// Sends multiple ping attempts to reduce false negatives from packet loss
func PingHost(ctx context.Context, host string) (bool, error) {
	for attempt := 1; attempt <= PingAttempts; attempt++ {
		// Create context with timeout for this attempt
		attemptCtx, cancel := context.WithTimeout(ctx, PingTimeout)

		ok := pingOnce(attemptCtx, host)
		cancel()

		if ok {
			return true, nil
		}

		// Check if parent context was cancelled
		if ctx.Err() != nil {
			return false, ctx.Err()
		}
	}

	// All attempts failed
	return false, nil
}

// pingOnce sends a single ping packet
func pingOnce(ctx context.Context, host string) bool {
	var cmd *exec.Cmd

	// Platform-specific ping command
	if runtime.GOOS == "windows" {
		// Windows: ping -n 1 -w 1500 <host>
		// -n 1: send 1 packet
		// -w 1500: timeout 1500ms
		cmd = exec.CommandContext(ctx, "ping", "-n", "1", "-w", "1500", host)
	} else {
		// Linux/Mac: ping -c 1 -W 2 <host>
		// -c 1: send 1 packet
		// -W 2: timeout 2 seconds
		cmd = exec.CommandContext(ctx, "ping", "-c", "1", "-W", "2", host)
	}

	// Run ping and check exit code
	err := cmd.Run()
	return err == nil
}
