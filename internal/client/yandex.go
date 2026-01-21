package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/fxfuren/yandex-watcher-bot/internal/types"
	"golang.org/x/time/rate"
)

// YandexClient handles API communication with Yandex Cloud
type YandexClient struct {
	httpClient  *http.Client
	rateLimiter *rate.Limiter
}

// normalizeStatus converts API status (e.g., "RUNNING", "STOPPED") to our internal format
func normalizeStatus(apiStatus string) types.VMStatus {
	// API returns uppercase, we need proper case
	switch apiStatus {
	case "RUNNING":
		return types.StatusRunning
	case "STOPPED":
		return types.StatusStopped
	case "STARTING":
		return types.StatusStarting
	case "STOPPING":
		return types.StatusStopping
	case "CRASHED":
		return types.StatusCrashed
	case "ERROR":
		return types.StatusError
	case "PROVISIONING":
		return types.StatusProvisioning
	case "RESTARTING":
		return types.StatusRestarting
	case "UPDATING":
		return types.StatusUpdating
	case "DELETING":
		return types.StatusDeleting
	default:
		return types.StatusUnknown
	}
}

// NewYandexClient creates a new Yandex Cloud API client
func NewYandexClient() *YandexClient {
	return &YandexClient{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		// Rate limit: 10 requests per second with burst of 20
		rateLimiter: rate.NewLimiter(rate.Limit(10), 20),
	}
}

// VMInfo contains information about a VM
type VMInfo struct {
	Status            types.VMStatus
	IP                string
	NetworkInterfaces []NetworkInterface `json:"networkInterfaces"`
}

type NetworkInterface struct {
	PrimaryV4Address PrimaryV4Address `json:"primaryV4Address"`
}

type PrimaryV4Address struct {
	Address   string    `json:"address"`
	OneToOneNat *OneToOneNat `json:"oneToOneNat,omitempty"`
}

type OneToOneNat struct {
	Address string `json:"address"`
}

// StartVMResponse contains the response from start VM API
type StartVMResponse struct {
	Success bool
	Message string
	IP      string
	WasAlreadyRunning bool
}

// GetVMInfo retrieves the current status and IP of a VM
func (c *YandexClient) GetVMInfo(ctx context.Context, baseURL string) (*VMInfo, error) {
	// Wait for rate limiter
	if err := c.rateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limiter: %w", err)
	}

	url := baseURL + "/info"
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	var rawInfo struct {
		Status            string              `json:"status"`
		NetworkInterfaces []NetworkInterface `json:"networkInterfaces"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&rawInfo); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Normalize status: "RUNNING" -> "Running", "STOPPED" -> "Stopped"
	info := &VMInfo{
		Status:            normalizeStatus(rawInfo.Status),
		NetworkInterfaces: rawInfo.NetworkInterfaces,
	}

	// Extract IP address
	if len(info.NetworkInterfaces) > 0 {
		primary := info.NetworkInterfaces[0].PrimaryV4Address
		// Prefer public IP
		if primary.OneToOneNat != nil && primary.OneToOneNat.Address != "" {
			info.IP = primary.OneToOneNat.Address
		} else if primary.Address != "" {
			info.IP = primary.Address
		}
	}

	return info, nil
}

// StartVM attempts to start a VM
func (c *YandexClient) StartVM(ctx context.Context, baseURL string) (*StartVMResponse, error) {
	// Wait for rate limiter
	if err := c.rateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limiter: %w", err)
	}

	url := baseURL + "/start"
	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	result := &StartVMResponse{}

	// Status 200 means VM started successfully
	if resp.StatusCode == http.StatusOK {
		result.Success = true
		result.Message = "VM started successfully"
		return result, nil
	}

	// Try to parse error response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var errorResp struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		IP      string `json:"ip"`
	}

	if err := json.Unmarshal(body, &errorResp); err != nil {
		// Not JSON, return raw error
		result.Success = false
		result.Message = fmt.Sprintf("API error (%d): %s", resp.StatusCode, string(body))
		return result, nil
	}

	// Code 9 with "RUNNING" message means VM is already running
	if errorResp.Code == 9 && errorResp.Message == "RUNNING" {
		result.Success = true
		result.WasAlreadyRunning = true
		result.IP = errorResp.IP
		result.Message = "VM is already running"
		return result, nil
	}

	// Other error codes
	result.Success = false
	result.Message = fmt.Sprintf("API error (%d): %s", resp.StatusCode, errorResp.Message)
	return result, nil
}

// WithRetry wraps a function with exponential backoff retry logic
func WithRetry(ctx context.Context, maxRetries int, fn func() error) error {
	var lastErr error
	backoff := 1 * time.Second

	for attempt := 0; attempt < maxRetries; attempt++ {
		if err := fn(); err != nil {
			lastErr = err

			// Check if context is cancelled
			if ctx.Err() != nil {
				return ctx.Err()
			}

			// Wait with exponential backoff + jitter
			jitter := time.Duration(float64(backoff) * 0.2)
			sleep := backoff + jitter

			select {
			case <-time.After(sleep):
				backoff *= 2
				if backoff > 16*time.Second {
					backoff = 16 * time.Second
				}
				continue
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		return nil
	}

	return fmt.Errorf("max retries exceeded: %w", lastErr)
}
