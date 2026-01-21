package network

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestPingHost_Success(t *testing.T) {
	// Start a test server
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to start test server: %v", err)
	}
	defer listener.Close()

	// Get the port
	addr := listener.Addr().(*net.TCPAddr)

	ctx := context.Background()
	reachable, err := PingHost(ctx, "127.0.0.1", addr.Port, 1*time.Second)

	if err != nil {
		t.Fatalf("PingHost returned error: %v", err)
	}

	if !reachable {
		t.Error("Expected host to be reachable, got unreachable")
	}
}

func TestPingHost_Timeout(t *testing.T) {
	ctx := context.Background()

	// Use a non-routable IP address (should timeout)
	reachable, err := PingHost(ctx, "192.0.2.1", 9999, 100*time.Millisecond)

	if err != nil {
		t.Fatalf("PingHost returned error: %v", err)
	}

	if reachable {
		t.Error("Expected host to be unreachable, got reachable")
	}
}

func TestPingHost_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel immediately
	cancel()

	reachable, err := PingHost(ctx, "192.0.2.1", 9999, 5*time.Second)

	// Should return immediately due to cancelled context
	if reachable {
		t.Error("Expected host to be unreachable due to cancelled context")
	}

	// Error is acceptable (context cancelled)
	_ = err
}

func TestPingHostSSH(t *testing.T) {
	// This test will fail if there's no SSH server on localhost
	// Skip if not available
	ctx := context.Background()
	reachable, err := PingHostSSH(ctx, "127.0.0.1")

	if err != nil {
		t.Logf("PingHostSSH returned error (expected if no SSH server): %v", err)
	}

	t.Logf("SSH server reachable: %v", reachable)
}

func BenchmarkPingHost(b *testing.B) {
	// Start a test server
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		b.Fatalf("Failed to start test server: %v", err)
	}
	defer listener.Close()

	addr := listener.Addr().(*net.TCPAddr)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = PingHost(ctx, "127.0.0.1", addr.Port, 1*time.Second)
	}
}
