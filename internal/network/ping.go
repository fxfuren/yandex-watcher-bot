package network

import (
	"context"
	"os/exec"
	"runtime"
	"time"
)

// PingHost checks if a host is reachable using ICMP ping
func PingHost(ctx context.Context, host string) (bool, error) {
	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	var cmd *exec.Cmd

	// Platform-specific ping command
	if runtime.GOOS == "windows" {
		// Windows: ping -n 1 -w 1000 <host>
		// -n 1: send 1 packet
		// -w 1000: timeout 1000ms
		cmd = exec.CommandContext(ctx, "ping", "-n", "1", "-w", "1000", host)
	} else {
		// Linux/Mac: ping -c 1 -W 1 <host>
		// -c 1: send 1 packet
		// -W 1: timeout 1 second
		cmd = exec.CommandContext(ctx, "ping", "-c", "1", "-W", "1", host)
	}

	// Run ping and check exit code
	err := cmd.Run()
	if err != nil {
		// Ping failed (timeout or unreachable)
		return false, nil
	}

	// Ping successful
	return true, nil
}
