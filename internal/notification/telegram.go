package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// TelegramClient handles sending notifications via Telegram
type TelegramClient struct {
	botToken    string
	groupChatID int64
	topicID     *int
	httpClient  *http.Client
}

// NewTelegramClient creates a new Telegram client
func NewTelegramClient(botToken string, groupChatID int64, topicID *int) *TelegramClient {
	return &TelegramClient{
		botToken:    botToken,
		groupChatID: groupChatID,
		topicID:     topicID,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// SendMessage sends a message to the configured group chat
func (t *TelegramClient) SendMessage(ctx context.Context, message string) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", t.botToken)

	payload := map[string]interface{}{
		"chat_id":    t.groupChatID,
		"text":       message,
		"parse_mode": "Markdown",
	}

	if t.topicID != nil {
		payload["message_thread_id"] = *t.topicID
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram API returned status %d", resp.StatusCode)
	}

	return nil
}
