package notification

import (
	"context"
	"sync"
	"time"

	"github.com/fxfuren/yandex-watcher-bot/internal/types"
	"github.com/fxfuren/yandex-watcher-bot/pkg/logger"
)

// Notification represents a message to be sent
type Notification struct {
	VMName   string
	Status   types.VMStatus
	Message  string
	Priority Priority
}

// Priority defines the importance of a notification
type Priority int

const (
	PriorityLow Priority = iota
	PriorityNormal
	PriorityCritical
)

// NotificationQueue manages a queue of notifications with deduplication
type NotificationQueue struct {
	client       *TelegramClient
	queue        chan Notification
	workers      int
	deduplicator *Deduplicator
	wg           sync.WaitGroup
	ctx          context.Context
	cancel       context.CancelFunc
}

// NewNotificationQueue creates a new notification queue
func NewNotificationQueue(client *TelegramClient, workers int) *NotificationQueue {
	ctx, cancel := context.WithCancel(context.Background())
	return &NotificationQueue{
		client:       client,
		queue:        make(chan Notification, 100),
		workers:      workers,
		deduplicator: NewDeduplicator(5 * time.Minute),
		ctx:          ctx,
		cancel:       cancel,
	}
}

// Start launches the worker goroutines
func (nq *NotificationQueue) Start() {
	for i := 0; i < nq.workers; i++ {
		nq.wg.Add(1)
		go nq.worker(i)
	}
}

// Stop gracefully stops the notification queue
func (nq *NotificationQueue) Stop() {
	close(nq.queue)
	nq.cancel()
	nq.wg.Wait()
}

// Enqueue adds a notification to the queue with deduplication
func (nq *NotificationQueue) Enqueue(notif Notification) {
	// Check if this notification was recently sent
	key := notif.VMName + ":" + string(notif.Status)

	// Critical notifications always go through
	if notif.Priority != PriorityCritical {
		if nq.deduplicator.IsDuplicate(key) {
			logger.Debug("Skipping duplicate notification",
				"vm", notif.VMName,
				"status", notif.Status,
			)
			return
		}
	}

	// Mark as sent
	nq.deduplicator.Mark(key)

	// Try to enqueue, don't block
	select {
	case nq.queue <- notif:
	default:
		logger.Warn("Notification queue full, dropping message",
			"vm", notif.VMName,
			"status", notif.Status,
		)
	}
}

func (nq *NotificationQueue) worker(id int) {
	defer nq.wg.Done()

	for notif := range nq.queue {
		// Create a timeout context for sending
		ctx, cancel := context.WithTimeout(nq.ctx, 10*time.Second)

		if err := nq.client.SendMessage(ctx, notif.Message); err != nil {
			logger.Error("❌ Failed to send Telegram alert",
				"worker", id,
				"vm", notif.VMName,
				"status", notif.Status,
				"error", err,
			)
		} else {
			// Make it visible that alert was sent
			var emoji string
			switch notif.Priority {
			case PriorityCritical:
				emoji = "🚨"
			case PriorityNormal:
				emoji = "✅"
			default:
				emoji = "📢"
			}
			logger.Info(emoji+" Telegram alert sent",
				"vm", notif.VMName,
				"status", notif.Status,
				"priority", notif.Priority,
			)
		}

		cancel()
	}
}

// Deduplicator prevents sending duplicate notifications within a time window
type Deduplicator struct {
	mu       sync.RWMutex
	recent   map[string]time.Time
	window   time.Duration
	cleanupTicker *time.Ticker
	done     chan struct{}
}

// NewDeduplicator creates a new deduplicator
func NewDeduplicator(window time.Duration) *Deduplicator {
	d := &Deduplicator{
		recent: make(map[string]time.Time),
		window: window,
		cleanupTicker: time.NewTicker(window),
		done:   make(chan struct{}),
	}
	go d.cleanup()
	return d
}

// IsDuplicate checks if a key was recently marked
func (d *Deduplicator) IsDuplicate(key string) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()

	lastSent, exists := d.recent[key]
	if !exists {
		return false
	}

	return time.Since(lastSent) < d.window
}

// Mark records that a key was sent
func (d *Deduplicator) Mark(key string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.recent[key] = time.Now()
}

func (d *Deduplicator) cleanup() {
	for {
		select {
		case <-d.cleanupTicker.C:
			d.mu.Lock()
			now := time.Now()
			for key, timestamp := range d.recent {
				if now.Sub(timestamp) > d.window {
					delete(d.recent, key)
				}
			}
			d.mu.Unlock()
		case <-d.done:
			d.cleanupTicker.Stop()
			return
		}
	}
}

// Stop stops the cleanup goroutine
func (d *Deduplicator) Stop() {
	close(d.done)
}
