package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fxfuren/yandex-watcher-bot/internal/client"
	"github.com/fxfuren/yandex-watcher-bot/internal/config"
	"github.com/fxfuren/yandex-watcher-bot/internal/monitoring"
	"github.com/fxfuren/yandex-watcher-bot/internal/notification"
	"github.com/fxfuren/yandex-watcher-bot/pkg/logger"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env file
	_ = godotenv.Load()

	logger.Info("🚀 Yandex VM Watchdog Bot starting...")

	// Load configuration
	cfg, err := config.Load("vms.yaml")
	if err != nil {
		logger.Critical("Failed to load configuration",
			"error", err,
		)
		os.Exit(1)
	}

	logger.Info("Configuration loaded",
		"vm_count", len(cfg.VMs),
		"min_interval", cfg.MinCheckInterval,
		"max_interval", cfg.MaxCheckInterval,
	)

	// Create clients
	yandexClient := client.NewYandexClient()
	telegramClient := notification.NewTelegramClient(cfg.BotToken, cfg.GroupChatID, cfg.TopicID)

	// Create notification queue
	notifier := notification.NewNotificationQueue(telegramClient, cfg.TelegramWorkers)
	notifier.Start()

	// Create coordinator
	coordinator := monitoring.NewCoordinator(cfg, yandexClient, notifier)

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		logger.Info("⚠️ Received signal, shutting down gracefully...",
			"signal", sig.String(),
		)
		cancel()
	}()

	// Start monitoring
	coordinator.Start(ctx)

	// Wait for context cancellation
	<-ctx.Done()

	logger.Info("⏳ Waiting for all monitors to stop...")

	// Give monitors time to finish their current checks
	done := make(chan struct{})
	go func() {
		coordinator.Wait()
		close(done)
	}()

	select {
	case <-done:
		logger.Info("✅ All monitors stopped cleanly")
	case <-time.After(10 * time.Second):
		logger.Warn("⚠️ Forceful shutdown after timeout")
	}

	// Stop notification queue
	notifier.Stop()

	logger.Info("👋 Yandex VM Watchdog Bot stopped")
}
