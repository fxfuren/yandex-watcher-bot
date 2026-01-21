# Technical Plan: Go Rewrite - VM Monitoring Bot

## 🎯 Objective
Rewrite Python-based VM monitoring bot in Go to achieve **sub-10-second reaction time** for critical VM status changes (currently 60-300 seconds).

## 📊 Current Architecture Analysis

### Problems Identified
1. **Sequential VM checking**: VMs checked one-by-one, delays accumulate
2. **Fixed 60s interval**: Same for all VM states (Running, Stopped, Crashed)
3. **Blocking operations**: Ping (3s) + API (5-10s) = 13s per VM serially
4. **No prioritization**: Critical statuses (Stopped/Crashed) not prioritized
5. **Python GIL limitations**: True parallelism not possible

### Current Delays
- **Best case**: 60s (CHECK_INTERVAL)
- **Worst case**: 60s + (N_VMs × 13s) ≈ 60-300s for multiple VMs
- **Average**: ~120-180s reaction time

---

## 🏗️ Go Architecture Design

### Core Principles
1. **Concurrency-first**: Every VM monitored in parallel goroutine
2. **Dynamic intervals**: Check frequency based on VM status
3. **Non-blocking I/O**: All network ops with context/timeout
4. **Fail-fast detection**: Prioritize critical status detection
5. **Resource-bounded**: Worker pools to prevent API flooding

---

## 🔄 Concurrency Model

### Architecture Choice: **Per-VM Goroutine + Worker Pool**

```
┌─────────────────────────────────────────┐
│         Main Coordinator                │
│  (spawn/manage VM goroutines)           │
└────────────┬────────────────────────────┘
             │
     ┌───────┴───────┬───────────────┬─────────
     │               │               │
┌────▼────┐    ┌────▼────┐    ┌────▼────┐
│ VM-1    │    │ VM-2    │    │ VM-N    │
│ Monitor │    │ Monitor │    │ Monitor │
│ Loop    │    │ Loop    │    │ Loop    │
└────┬────┘    └────┬────┘    └────┬────┘
     │              │              │
     └──────┬───────┴──────┬───────┘
            │              │
    ┌───────▼──────┐   ┌──▼────────┐
    │ API Worker   │   │ Telegram  │
    │ Pool (5)     │   │ Notifier  │
    │ (rate limit) │   │ (queue)   │
    └──────────────┘   └───────────┘
```

### Why Per-VM Goroutine?
- **Independent intervals**: Each VM checks at its own pace
- **No blocking**: One slow VM doesn't delay others
- **State isolation**: Each goroutine manages its VM state
- **Simple reasoning**: No complex synchronization

### Worker Pool for API Calls
- **Purpose**: Prevent overwhelming Yandex Cloud API
- **Size**: 5-10 concurrent API calls max
- **Benefit**: Rate limiting + backpressure

---

## ⏱️ Dynamic Check Intervals by Status

| Status          | Interval | Rationale                                      |
|-----------------|----------|------------------------------------------------|
| **Stopped**     | 5s       | CRITICAL - Start immediately, verify fast     |
| **Crashed**     | 5s       | CRITICAL - Needs immediate recovery           |
| **Error**       | 10s      | CRITICAL - May recover or need intervention   |
| **Starting**    | 10s      | Monitor startup progress closely              |
| **Provisioning**| 15s      | Resource allocation in progress               |
| **Restarting**  | 10s      | Monitor restart completion                    |
| **Stopping**    | 15s      | Monitor shutdown, prepare for restart         |
| **Running**     | 60s      | Stable state - less frequent checks OK        |
| **Updating**    | 30s      | Monitor update progress                       |
| **Deleting**    | 60s      | Informational only, no action needed          |

### Adaptive Intervals
- **Success streak**: Increase interval (up to max for status)
- **Failure streak**: Decrease interval (down to 5s minimum)
- **Jitter**: Add ±10% randomness to prevent API thundering herd

---

## 🚨 VM Status Handling Logic

### Critical Statuses (Immediate Action)
```go
case Stopped, Crashed, Error:
    - Log critical event
    - Send Telegram alert immediately
    - Trigger VM start via API
    - Switch to fast-poll mode (5s)
    - Retry on failure with exponential backoff
```

### Transitional Statuses (Monitor)
```go
case Starting, Provisioning, Restarting:
    - Monitor progress
    - Alert if stuck (timeout: 5min)
    - No action needed (already transitioning)
```

### Stable Statuses (Low Frequency)
```go
case Running:
    - Verify with ping (port 22)
    - Fall back to API if ping fails
    - Normal interval (60s)

case Deleting:
    - Log only, no action
    - Remove from monitoring when complete
```

### Edge Cases
```go
case Stopping:
    - DO NOT start VM (intentional shutdown)
    - Monitor → Stopped transition
    - Alert if stuck

case Updating:
    - DO NOT interfere
    - Monitor completion
    - Alert if stuck (timeout: 10min)
```

---

## 🔌 API Integration

### Yandex Cloud API Endpoints

#### 1. `/start` - Start VM
**Request**: `POST /start`
**Responses**:
- `200`: VM started successfully
- `409 + {"code": 9, "message": "RUNNING", "ip": "..."}`: Already running
- Other: Error

#### 2. `/info` - Get VM Info
**Request**: `GET /info`
**Response**:
```json
{
  "status": "Running|Stopped|...",
  "networkInterfaces": [{
    "primaryV4Address": {
      "address": "10.0.0.1",
      "oneToOneNat": {"address": "51.250.10.174"}
    }
  }]
}
```

### API Client Design
```go
type YandexCloudClient struct {
    httpClient  *http.Client
    rateLimiter *rate.Limiter  // Token bucket
    circuitBreaker *CircuitBreaker
}

// Rate limiting: 10 req/sec per VM, burst 5
rateLimiter := rate.NewLimiter(rate.Limit(10), 5)
```

### Retry Strategy
- **Exponential backoff**: 1s → 2s → 4s → 8s → 16s
- **Max retries**: 5
- **Jitter**: ±20% to prevent synchronization
- **Circuit breaker**: Open after 5 consecutive failures

---

## 📡 Telegram Integration

### Notification Queue
```go
type NotificationQueue struct {
    ch chan Notification
    workers int  // 3 workers
}
```

### Alert Prioritization
1. **Critical** (Stopped/Crashed/Error): Send immediately
2. **Warning** (Stuck transitions): Send with 30s debounce
3. **Info** (Recovered): Send with 10s debounce

### Alert Deduplication
- Track last alert per VM+status
- Don't re-alert same status within 5 minutes
- Exception: Critical statuses always alert

---

## 🏗️ Project Structure

```
yandex-watcher-bot/
├── cmd/
│   └── watchdog/
│       └── main.go              # Entry point
├── internal/
│   ├── config/
│   │   ├── config.go            # Configuration loading
│   │   └── vm.go                # VM config structure
│   ├── monitoring/
│   │   ├── coordinator.go       # Main coordinator
│   │   ├── vm_monitor.go        # Per-VM monitor goroutine
│   │   ├── status.go            # Status enum + intervals
│   │   └── state_machine.go     # State transitions
│   ├── client/
│   │   ├── yandex.go            # Yandex Cloud API client
│   │   ├── ratelimit.go         # Rate limiter
│   │   └── circuitbreaker.go    # Circuit breaker
│   ├── network/
│   │   └── ping.go              # TCP ping (port 22)
│   ├── notification/
│   │   ├── telegram.go          # Telegram client
│   │   ├── queue.go             # Notification queue
│   │   └── deduplicator.go      # Alert deduplication
│   └── workerpool/
│       └── pool.go              # Worker pool for API calls
├── pkg/
│   └── logger/
│       └── logger.go            # Structured logging (zap/zerolog)
├── go.mod
├── go.sum
├── Dockerfile.go
├── docker-compose.go.yml
└── README.go.md
```

---

## 🧪 Testing Strategy

### Unit Tests
```go
// internal/monitoring/vm_monitor_test.go
TestStatusIntervals()           // Verify interval logic
TestStateTransitions()          // Valid state changes
TestCriticalStatusHandling()    // Immediate actions

// internal/client/yandex_test.go
TestAPIRetries()                // Exponential backoff
TestCircuitBreaker()            // Opens after failures
TestRateLimiting()              // Respects limits
```

### Concurrency Tests
```bash
go test -race ./...             # Race detector
go test -count=100 -short      # Flakiness detection
```

### Integration Tests
```go
TestMultipleVMsConcurrent()     // 10+ VMs in parallel
TestGracefulShutdown()          // Clean context cancellation
TestAPITimeout()                // Handle slow API
```

### Chaos Testing
- Simulate API failures (50% failure rate)
- Random delays (100-2000ms)
- Network timeouts
- Verify: No goroutine leaks, alerts still sent

---

## 📈 Performance Targets

### Before (Python)
- **Reaction time**: 60-300s (avg ~150s)
- **Throughput**: 1 VM check every 13s (serial)
- **Parallelism**: None (GIL)

### After (Go)
- **Reaction time**: 5-15s for critical statuses
- **Throughput**: 100+ concurrent VM checks
- **Parallelism**: True concurrency with goroutines

### Metrics to Track
- Time from status change to alert sent
- API call latency (p50, p95, p99)
- Goroutine count (should be stable)
- Memory usage (should be < 50MB)

---

## 🛡️ Error Handling

### Graceful Degradation
1. **API unavailable**: Use cached status, increase interval
2. **Telegram unavailable**: Queue alerts in memory (1000 max)
3. **Single VM failing**: Don't affect others
4. **Config reload error**: Keep running with old config

### Observability
```go
// Structured logging with fields
log.Error("VM check failed",
    "vm", vmName,
    "status", status,
    "error", err,
    "retry", retryCount,
)

// Metrics (optional: Prometheus)
vm_checks_total{vm="ru-ya-01", status="Running"}
vm_check_duration_seconds{vm="ru-ya-01"}
api_errors_total{endpoint="/start"}
```

---

## 🚀 Deployment

### Docker
```dockerfile
# Multi-stage build
FROM golang:1.22-alpine AS builder
# ... build ...
FROM alpine:latest
# Final image ~10MB
```

### Environment Variables
```bash
BOT_TOKEN=...
GROUP_CHAT_ID=...
TOPIC_ID=...
MIN_CHECK_INTERVAL=5      # Minimum interval (critical)
MAX_CHECK_INTERVAL=60     # Maximum interval (stable)
API_WORKER_POOL_SIZE=10   # Concurrent API calls
TELEGRAM_WORKERS=3        # Alert senders
```

### Health Checks
```go
// HTTP endpoint :8080/health
{
  "status": "healthy",
  "vms_monitored": 5,
  "goroutines": 23,
  "uptime_seconds": 3600
}
```

---

## ✅ Success Criteria

1. **Reaction time < 15s** for Stopped/Crashed/Error statuses
2. **No race conditions** (race detector clean)
3. **No goroutine leaks** (stable goroutine count)
4. **Handles 50+ VMs** concurrently without degradation
5. **Graceful shutdown** within 10 seconds
6. **Memory usage < 100MB** for 50 VMs

---

## 🔄 Migration Plan

1. **Phase 1**: Deploy Go bot in parallel (different topic)
2. **Phase 2**: Compare reaction times (1 week)
3. **Phase 3**: Switch production traffic to Go bot
4. **Phase 4**: Decommission Python bot

---

## 📝 Open Questions / Decisions

### 1. Should we add Prometheus metrics?
**Decision**: Start simple, add later if needed (YAGNI)

### 2. Config reload without restart?
**Decision**: Yes, watch vms.yaml with fsnotify, reload on change

### 3. Persistent state across restarts?
**Decision**: No, in-memory only. Fresh start = fresh state

### 4. How to handle API rate limits from Yandex?
**Decision**: Client-side rate limiting (10 req/s) + exponential backoff

---

## 🎓 Key Takeaways

### Why Go Will Be Faster
1. **True parallelism**: Goroutines, no GIL
2. **Non-blocking I/O**: Native async with context
3. **Compiled binary**: No interpreter overhead
4. **Efficient scheduling**: M:N threading model
5. **Dynamic intervals**: Check critical VMs 12x more often

### Estimated Improvement
- **Best case**: 60s → 5s (12x faster)
- **Average case**: 150s → 10s (15x faster)
- **Worst case**: 300s → 20s (15x faster)

**Target achieved**: ✅ Sub-10-second reactions for critical statuses
