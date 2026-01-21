package monitoring

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/fxfuren/yandex-watcher-bot/internal/client"
	"github.com/fxfuren/yandex-watcher-bot/internal/config"
	"github.com/fxfuren/yandex-watcher-bot/internal/network"
	"github.com/fxfuren/yandex-watcher-bot/internal/notification"
	"github.com/fxfuren/yandex-watcher-bot/internal/types"
	"github.com/fxfuren/yandex-watcher-bot/pkg/logger"
)

// VMMonitor manages monitoring for a single VM
type VMMonitor struct {
	vm              *config.VM
	client          *client.YandexClient
	notifier        *notification.NotificationQueue
	minInterval     time.Duration
	maxInterval     time.Duration
	currentStatus   types.VMStatus
	lastStatusTime  time.Time
	lastAPICheck    time.Time     // Track last API check time
	gracePeriodUntil time.Time    // Skip checks until this time (for VM startup)
	mu              sync.RWMutex
	configMu        *sync.Mutex
	ipUpdateChan    chan string
}

// NewVMMonitor creates a new VM monitor
func NewVMMonitor(
	vm *config.VM,
	client          *client.YandexClient,
	notifier        *notification.NotificationQueue,
	minInterval, maxInterval time.Duration,
	configMu        *sync.Mutex,
	ipUpdateChan    chan string,
) *VMMonitor {
	return &VMMonitor{
		vm:              vm,
		client:          client,
		notifier:        notifier,
		minInterval:     minInterval,
		maxInterval:     maxInterval,
		currentStatus:   types.StatusUnknown,
		lastStatusTime:  time.Now(),
		configMu:        configMu,
		ipUpdateChan:    ipUpdateChan,
	}
}

// Start begins monitoring the VM
func (m *VMMonitor) Start(ctx context.Context) {
	logger.Info("Starting VM monitor",
		"vm", m.vm.Name,
		"url", m.vm.URL,
	)

	// Run first check asynchronously to allow all monitors to start in parallel
	go m.check(ctx)

	ticker := time.NewTicker(m.getCurrentInterval())
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("VM monitor stopping",
				"vm", m.vm.Name,
			)
			return
		case <-ticker.C:
			m.check(ctx)
			ticker.Reset(m.getCurrentInterval())
		}
	}
}

func (m *VMMonitor) check(ctx context.Context) {
	vmName := m.vm.Name
	currentStatus := m.getCurrentStatus()

	// Skip check if we're in grace period (VM is starting up)
	m.mu.RLock()
	gracePeriodUntil := m.gracePeriodUntil
	m.mu.RUnlock()

	if time.Now().Before(gracePeriodUntil) {
		timeLeft := time.Until(gracePeriodUntil).Round(time.Second)
		logger.Info("⏸️ Grace period active, skipping check",
			"vm", vmName,
			"status", currentStatus,
			"time_left", timeLeft,
		)
		return
	}

	logger.Info("🔍 Checking VM",
		"vm", vmName,
		"status", currentStatus,
	)

	// Strategy: Ping first (free), API only if needed (like Python version)
	// This saves ~90% of API calls when VMs are stable

	knownIP := m.vm.IP
	needAPICheck := false

	// 1. Try ping first if we have IP
	if knownIP != "" {
		pingSuccess, _ := network.PingHost(ctx, knownIP)

		if pingSuccess {
			if currentStatus != types.StatusRunning {
				// VM recovered! Update status and send notification
				oldStatus := currentStatus
				m.setStatus(types.StatusRunning)

				// Clear grace period since VM is now responding
				m.mu.Lock()
				m.gracePeriodUntil = time.Time{}
				m.mu.Unlock()

				// Only send notification if this is a real recovery (not initial startup)
				if oldStatus != types.StatusUnknown {
					logger.Info("✅ VM recovered via ping",
						"vm", vmName,
						"ip", knownIP,
						"old_status", oldStatus,
					)

					message := fmt.Sprintf("✅ ВОССТАНОВЛЕНИЕ: ВМ *%s* снова в строю.\n\nПроверка: Ping OK на %s", m.vm.Name, knownIP)
					m.notifier.Enqueue(notification.Notification{
						VMName:   m.vm.Name,
						Status:   types.StatusRunning,
						Message:  message,
						Priority: notification.PriorityCritical,
					})
				} else {
					logger.Info("✅ VM initialized as Running",
						"vm", vmName,
						"ip", knownIP,
					)
				}
			} else {
				logger.Info("🏓 Ping OK",
					"vm", vmName,
					"ip", knownIP,
				)
			}
			return // Ping OK, skip API
		} else {
			// Ping failed, need to check API
			logger.Warn("⚠️ Ping failed, checking API",
				"vm", vmName,
				"ip", knownIP,
			)
			needAPICheck = true
		}
	} else {
		// No IP yet, must check API
		needAPICheck = true
	}

	// 2. Check API if needed
	if needAPICheck {
		m.mu.Lock()
		m.lastAPICheck = time.Now()
		m.mu.Unlock()

		info, err := m.client.GetVMInfo(ctx, m.vm.URL)
		if err != nil {
			logger.Error("❌ Failed to get VM info",
				"vm", vmName,
				"error", err,
			)
			return
		}

		logger.Info("📡 API response",
			"vm", vmName,
			"status", info.Status,
			"ip", info.IP,
		)

		// Update IP if changed or discovered
		if info.IP != "" && info.IP != m.vm.IP {
			m.updateIP(info.IP)
		}

		// Handle status change based on API response
		m.handleStatusChange(ctx, info.Status)
	}
}

func (m *VMMonitor) handleRunningState(ctx context.Context, details string) {
	oldStatus := m.getCurrentStatus()

	if oldStatus != types.StatusRunning {
		m.setStatus(types.StatusRunning)

		if oldStatus != types.StatusUnknown {
			message := fmt.Sprintf("✅ ВОССТАНОВЛЕНИЕ: ВМ *%s* снова в строю.\n\n%s", m.vm.Name, details)
			m.notifier.Enqueue(notification.Notification{
				VMName:   m.vm.Name,
				Status:   types.StatusRunning,
				Message:  message,
				Priority: notification.PriorityNormal,
			})
			logger.Info("✅ VM recovered",
				"vm", m.vm.Name,
				"from", oldStatus,
				"details", details,
			)
		}
	}
}

func (m *VMMonitor) handleStatusChange(ctx context.Context, newStatus types.VMStatus) {
	oldStatus := m.getCurrentStatus()

	if oldStatus == newStatus {
		m.checkStuckStatus(ctx)
		return
	}

	if !IsValidTransition(oldStatus, newStatus) {
		logger.Warn("Invalid status transition",
			"vm", m.vm.Name,
			"from", oldStatus,
			"to", newStatus,
		)
	}

	m.setStatus(newStatus)

	// Add emoji based on status
	var statusEmoji string
	switch {
	case newStatus.IsCritical():
		statusEmoji = "🚨"
	case newStatus == types.StatusRunning:
		statusEmoji = "✅"
	case newStatus.IsTransitional():
		statusEmoji = "⏳"
	default:
		statusEmoji = "ℹ️"
	}

	logger.Info(statusEmoji+" VM status changed",
		"vm", m.vm.Name,
		"old_status", oldStatus,
		"new_status", newStatus,
	)

	if newStatus.ShouldStartVM() {
		m.handleCriticalStatus(ctx, newStatus)
		return
	}

	if newStatus == types.StatusRunning {
		// Clear grace period since VM is now running
		m.mu.Lock()
		m.gracePeriodUntil = time.Time{}
		m.mu.Unlock()

		if oldStatus != types.StatusUnknown {
			message := fmt.Sprintf("✅ ВОССТАНОВЛЕНИЕ: ВМ *%s* снова в строю.\n\nСтатус API: Running", m.vm.Name)
			m.notifier.Enqueue(notification.Notification{
				VMName:   m.vm.Name,
				Status:   types.StatusRunning,
				Message:  message,
				Priority: notification.PriorityCritical, // Always send recovery notifications
			})
		}
		return
	}

	if newStatus.IsTransitional() {
		logger.Info("⏳ VM in transitional state",
			"vm", m.vm.Name,
			"status", newStatus,
		)
		return
	}
}

func (m *VMMonitor) handleCriticalStatus(ctx context.Context, status types.VMStatus) {
	vmName := m.vm.Name

	logger.Error("💥 VM in critical status",
		"vm", vmName,
		"status", status,
	)

	var emoji string
	switch status {
	case types.StatusStopped:
		emoji = "🚨"
	case types.StatusCrashed:
		emoji = "💥"
	case types.StatusError:
		emoji = "⚠️"
	}

	message := fmt.Sprintf("%s СБОЙ: ВМ *%s* недоступна.\n\nСтатус: %s", emoji, vmName, status)
	m.notifier.Enqueue(notification.Notification{
		VMName:   vmName,
		Status:   status,
		Message:  message,
		Priority: notification.PriorityCritical,
	})

	m.startVM(ctx)
}

func (m *VMMonitor) startVM(ctx context.Context) {
	vmName := m.vm.Name
	logger.Info("🔧 Attempting to start VM",
		"vm", vmName,
	)

	err := client.WithRetry(ctx, 3, func() error {
		resp, err := m.client.StartVM(ctx, m.vm.URL)
		if err != nil {
			return err
		}

		if !resp.Success {
			return fmt.Errorf("%s", resp.Message)
		}

		if resp.IP != "" && resp.IP != m.vm.IP {
			m.updateIP(resp.IP)
		}

		if resp.WasAlreadyRunning {
			logger.Info("ℹ️ VM was already running",
				"vm", vmName,
			)
		} else {
			// VM is starting - set grace period to avoid unnecessary API calls
			gracePeriod := 60 * time.Second
			m.mu.Lock()
			m.gracePeriodUntil = time.Now().Add(gracePeriod)
			m.mu.Unlock()

			logger.Info("🚀 VM start initiated",
				"vm", vmName,
				"grace_period", gracePeriod,
			)

			message := fmt.Sprintf("🚀 Автозапуск: ВМ *%s* запускается через API.", vmName)
			m.notifier.Enqueue(notification.Notification{
				VMName:   vmName,
				Status:   types.StatusStarting,
				Message:  message,
				Priority: notification.PriorityCritical,
			})
		}

		return nil
	})

	if err != nil {
		logger.Error("❌ Failed to start VM",
			"vm", vmName,
			"error", err,
		)
	}
}

func (m *VMMonitor) checkStuckStatus(ctx context.Context) {
	status := m.getCurrentStatus()

	if !status.IsTransitional() {
		return
	}

	timeout := status.GetTimeout()
	if timeout == 0 {
		return
	}

	m.mu.RLock()
	timeSinceChange := time.Since(m.lastStatusTime)
	m.mu.RUnlock()

	if timeSinceChange > timeout {
		logger.Warn("⏰ VM stuck in transitional status",
			"vm", m.vm.Name,
			"status", status,
			"duration", timeSinceChange,
		)

		message := fmt.Sprintf("⚠️ ВНИМАНИЕ: ВМ *%s* застряла в статусе %s более %v",
			m.vm.Name, status, timeSinceChange.Round(time.Second))

		m.notifier.Enqueue(notification.Notification{
			VMName:   m.vm.Name,
			Status:   status,
			Message:  message,
			Priority: notification.PriorityNormal,
		})
	}
}

func (m *VMMonitor) getCurrentStatus() types.VMStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.currentStatus
}

func (m *VMMonitor) setStatus(status types.VMStatus) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.currentStatus = status
	m.lastStatusTime = time.Now()
}

func (m *VMMonitor) getCurrentInterval() time.Duration {
	status := m.getCurrentStatus()
	return status.GetCheckInterval(m.minInterval, m.maxInterval)
}

func (m *VMMonitor) updateIP(newIP string) {
	m.configMu.Lock()
	oldIP := m.vm.IP
	m.vm.IP = newIP
	m.configMu.Unlock()

	if oldIP != newIP {
		logger.Info("🌐 IP address updated",
			"vm", m.vm.Name,
			"old_ip", oldIP,
			"new_ip", newIP,
		)

		select {
		case m.ipUpdateChan <- m.vm.Name:
		default:
		}
	}
}
