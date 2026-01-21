package notification

import (
	"testing"
	"time"

	"github.com/fxfuren/yandex-watcher-bot/internal/types"
)

func TestNotificationQueue_Enqueue(t *testing.T) {
	client := &TelegramClient{
		botToken:    "test",
		groupChatID: 123,
	}

	queue := NewNotificationQueue(client, 1)

	notif := Notification{
		VMName:   "test-vm",
		Status:   types.StatusStopped,
		Message:  "Test message",
		Priority: PriorityNormal,
	}

	queue.Enqueue(notif)
	queue.Enqueue(notif)

	key := "test-vm:" + string(types.StatusStopped)
	if !queue.deduplicator.IsDuplicate(key) {
		t.Error("Expected notification to be marked as duplicate")
	}

	queue.Stop()
}

func TestNotificationQueue_CriticalBypassesDedup(t *testing.T) {
	client := &TelegramClient{
		botToken:    "test",
		groupChatID: 123,
	}

	queue := NewNotificationQueue(client, 1)

	notif := Notification{
		VMName:   "test-vm",
		Status:   types.StatusCrashed,
		Message:  "Critical message",
		Priority: PriorityCritical,
	}

	queue.Enqueue(notif)
	queue.Enqueue(notif)
	queue.Enqueue(notif)

	queue.Stop()
}

func TestDeduplicator(t *testing.T) {
	window := 100 * time.Millisecond
	dedup := NewDeduplicator(window)
	defer dedup.Stop()

	key := "test-key"

	if dedup.IsDuplicate(key) {
		t.Error("Expected key to not be duplicate initially")
	}

	dedup.Mark(key)

	if !dedup.IsDuplicate(key) {
		t.Error("Expected key to be duplicate after marking")
	}

	time.Sleep(window + 10*time.Millisecond)

	if dedup.IsDuplicate(key) {
		t.Error("Expected key to not be duplicate after window expired")
	}
}

func TestDeduplicator_Cleanup(t *testing.T) {
	window := 50 * time.Millisecond
	dedup := NewDeduplicator(window)
	defer dedup.Stop()

	for i := 0; i < 10; i++ {
		dedup.Mark("key-" + string(rune(i)))
	}

	time.Sleep(window + 100*time.Millisecond)

	dedup.mu.RLock()
	count := len(dedup.recent)
	dedup.mu.RUnlock()

	if count > 0 {
		t.Errorf("Expected all keys to be cleaned up, got %d remaining", count)
	}
}

func BenchmarkDeduplicator_IsDuplicate(b *testing.B) {
	dedup := NewDeduplicator(5 * time.Minute)
	defer dedup.Stop()

	key := "test-key"
	dedup.Mark(key)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = dedup.IsDuplicate(key)
	}
}

func BenchmarkDeduplicator_Mark(b *testing.B) {
	dedup := NewDeduplicator(5 * time.Minute)
	defer dedup.Stop()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dedup.Mark("test-key")
	}
}
