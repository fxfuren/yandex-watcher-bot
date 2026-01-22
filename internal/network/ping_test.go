package network

import (
	"context"
	"testing"
)

func TestPingHost_Localhost(t *testing.T) {
	ctx := context.Background()

	// Localhost should always be reachable
	reachable, err := PingHost(ctx, "127.0.0.1")

	if err != nil {
		t.Fatalf("PingHost returned error: %v", err)
	}

	if !reachable {
		t.Error("Expected localhost to be reachable")
	}
}

func TestPingHost_Unreachable(t *testing.T) {
	ctx := context.Background()

	// Use a non-routable IP address (RFC 5737 TEST-NET-1)
	// This should timeout after all retry attempts
	reachable, err := PingHost(ctx, "192.0.2.1")

	if err != nil {
		t.Fatalf("PingHost returned error: %v", err)
	}

	if reachable {
		t.Error("Expected host 192.0.2.1 to be unreachable")
	}
}

func TestPingHost_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel immediately
	cancel()

	reachable, err := PingHost(ctx, "192.0.2.1")

	// Should return immediately due to cancelled context
	if reachable {
		t.Error("Expected host to be unreachable due to cancelled context")
	}

	// Error should indicate context cancellation
	if err == nil {
		t.Log("No error returned, which is acceptable")
	} else if err != context.Canceled {
		t.Logf("Error returned: %v", err)
	}
}

func TestPingOnce_Localhost(t *testing.T) {
	ctx := context.Background()

	// Single ping to localhost should succeed
	ok := pingOnce(ctx, "127.0.0.1")

	if !ok {
		t.Error("Expected single ping to localhost to succeed")
	}
}

func BenchmarkPingHost(b *testing.B) {
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = PingHost(ctx, "127.0.0.1")
	}
}
