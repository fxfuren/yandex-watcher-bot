package monitoring

import (
	"context"
	"sync"
	"time"

	"github.com/fxfuren/yandex-watcher-bot/internal/client"
	"github.com/fxfuren/yandex-watcher-bot/internal/config"
	"github.com/fxfuren/yandex-watcher-bot/internal/notification"
	"github.com/fxfuren/yandex-watcher-bot/pkg/logger"
)

// Coordinator manages all VM monitors
type Coordinator struct {
	config       *config.Config
	client       *client.YandexClient
	notifier     *notification.NotificationQueue
	monitors     []*VMMonitor
	configMu     sync.Mutex
	ipUpdateChan chan string
	wg           sync.WaitGroup
}

// NewCoordinator creates a new coordinator
func NewCoordinator(cfg *config.Config, client *client.YandexClient, notifier *notification.NotificationQueue) *Coordinator {
	return &Coordinator{
		config:       cfg,
		client:       client,
		notifier:     notifier,
		monitors:     make([]*VMMonitor, 0, len(cfg.VMs)),
		ipUpdateChan: make(chan string, 10),
	}
}

// Start begins monitoring all VMs
func (c *Coordinator) Start(ctx context.Context) {
	if len(c.config.VMs) == 0 {
		logger.Warn("No VMs configured for monitoring")
		return
	}

	logger.Info("Starting VM monitoring",
		"vm_count", len(c.config.VMs),
		"min_interval", c.config.MinCheckInterval,
		"max_interval", c.config.MaxCheckInterval,
	)

	// Create monitors for each VM
	for i := range c.config.VMs {
		monitor := NewVMMonitor(
			&c.config.VMs[i],
			c.client,
			c.notifier,
			c.config.MinCheckInterval,
			c.config.MaxCheckInterval,
			&c.configMu,
			c.ipUpdateChan,
		)
		c.monitors = append(c.monitors, monitor)

		// Start each monitor in its own goroutine
		c.wg.Add(1)
		go func(m *VMMonitor) {
			defer c.wg.Done()
			m.Start(ctx)
		}(monitor)
	}

	// Start IP update saver
	c.wg.Add(1)
	go c.ipUpdateSaver(ctx)

	logger.Info("All VM monitors started")
}

// Wait blocks until all monitors have stopped
func (c *Coordinator) Wait() {
	c.wg.Wait()
	logger.Info("All VM monitors stopped")
}

// ipUpdateSaver periodically saves IP updates to config file
func (c *Coordinator) ipUpdateSaver(ctx context.Context) {
	defer c.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	pendingUpdates := false

	for {
		select {
		case <-ctx.Done():
			// Save any pending updates before exiting
			if pendingUpdates {
				c.saveConfig()
			}
			return

		case <-c.ipUpdateChan:
			pendingUpdates = true

		case <-ticker.C:
			if pendingUpdates {
				c.saveConfig()
				pendingUpdates = false
			}
		}
	}
}

func (c *Coordinator) saveConfig() {
	c.configMu.Lock()
	defer c.configMu.Unlock()

	if err := c.config.SaveVMs("vms.yaml"); err != nil {
		logger.Error("Failed to save VM config",
			"error", err,
		)
	} else {
		logger.Debug("VM config saved successfully")
	}
}
