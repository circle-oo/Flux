package notifier

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// NotificationLevel represents the severity of a notification.
type NotificationLevel string

const (
	LevelInfo     NotificationLevel = "INFO"
	LevelWarning  NotificationLevel = "WARNING"
	LevelCritical NotificationLevel = "CRITICAL"
)

// Discord sends notifications via Discord webhook.
type Discord struct {
	webhookURL string
	http       *http.Client
}

// NewDiscord creates a new Discord notifier.
func NewDiscord(webhookURL string) *Discord {
	return &Discord{
		webhookURL: webhookURL,
		http: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Send sends a notification to Discord.
// If the webhook URL is empty, it logs and returns nil (graceful degradation).
func (d *Discord) Send(level NotificationLevel, message string) error {
	if d.webhookURL == "" {
		slog.Warn("discord webhook URL not configured, skipping notification",
			"level", string(level), "message", message)
		return nil
	}

	payload := map[string]string{
		"content": fmt.Sprintf("[%s] %s", level, message),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal discord payload: %w", err)
	}

	resp, err := d.http.Post(d.webhookURL, "application/json", bytes.NewReader(body))
	if err != nil {
		slog.Error("discord webhook request failed", "error", err)
		return nil // Graceful: don't crash
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		slog.Error("discord webhook returned error", "status", resp.StatusCode)
		return nil // Graceful: don't crash
	}

	return nil
}
