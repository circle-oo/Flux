package orchestrator

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/circle-oo/flux/internal/models"
	"github.com/circle-oo/flux/internal/notifier"
	"github.com/google/uuid"
)

const (
	// RateLimitDuration is how long to pause after detecting a rate limit
	RateLimitDuration = 5 * time.Hour
	// RecentLimitWindow is the lookback period for "recently limited" checks
	RecentLimitWindow = 6 * time.Hour
)

// RateLimitHandler manages rate limit detection, state, and recovery.
// It implements the SubComponent interface so it can be registered with
// the Orchestrator tick loop.
type RateLimitHandler struct {
	db             *sql.DB
	notifier       *notifier.Discord
	ccusageCmd     string
	rateLimitUntil time.Time
	isLimited      bool
	mu             sync.RWMutex
}

// NewRateLimitHandler creates a new rate limit handler.
func NewRateLimitHandler(db *sql.DB, notifier *notifier.Discord, ccusageCmd string) *RateLimitHandler {
	return &RateLimitHandler{
		db:         db,
		notifier:   notifier,
		ccusageCmd: ccusageCmd,
	}
}

// Name implements SubComponent.
func (h *RateLimitHandler) Name() string {
	return "rate_limit_handler"
}

// Tick implements SubComponent. It calls CheckAndRecover each tick.
func (h *RateLimitHandler) Tick(_ context.Context) error {
	h.CheckAndRecover()
	return nil
}

// HandleRateLimit records a rate limit event and sets rate limit state.
// It first tries to query the actual reset time via ccusage CLI; if that
// fails, it falls back to the 5-hour default.
func (h *RateLimitHandler) HandleRateLimit() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Try to get actual reset time from ccusage CLI
	if h.ccusageCmd != "" {
		if resetTime, err := queryCCUsageResetTime(h.ccusageCmd); err == nil && !resetTime.IsZero() {
			h.rateLimitUntil = resetTime
			h.isLimited = true
			slog.Info("rate limit: using ccusage reset time", "reset_at", resetTime.Format(time.RFC3339))
		} else {
			if err != nil {
				slog.Warn("failed to query ccusage reset time, using fallback", "error", err)
			}
			h.rateLimitUntil = time.Now().Add(RateLimitDuration)
			h.isLimited = true
		}
	} else {
		h.rateLimitUntil = time.Now().Add(RateLimitDuration)
		h.isLimited = true
	}

	resumeTime := h.rateLimitUntil.Format("2006-01-02 15:04:05 MST")
	message := fmt.Sprintf("Rate limit detected. Pods will pause until %s", resumeTime)

	slog.Warn("rate limit triggered", "resume_at", resumeTime)

	// Send Discord notification
	if err := h.notifier.Send(notifier.LevelWarning, message); err != nil {
		slog.Error("failed to send rate limit notification", "error", err)
	}

	// Record event in database
	store := models.NewUsageStore(h.db)
	event := &models.RateLimitEvent{
		ID:         uuid.New().String(),
		TokensUsed: 0,
		ActivePods: 0,
	}

	if err := store.CreateRateLimitEvent(event); err != nil {
		slog.Error("failed to record rate limit event", "error", err)
		return fmt.Errorf("record rate limit event: %w", err)
	}

	return nil
}

// CheckAndRecover checks if the rate limit window has expired and clears state if so.
func (h *RateLimitHandler) CheckAndRecover() {
	h.mu.Lock()
	defer h.mu.Unlock()

	if !h.isLimited {
		return
	}

	if time.Now().After(h.rateLimitUntil) {
		h.isLimited = false
		h.rateLimitUntil = time.Time{}

		message := "Rate limit recovery complete. Resuming normal operation."
		slog.Info("rate limit recovery complete")

		if err := h.notifier.Send(notifier.LevelInfo, message); err != nil {
			slog.Error("failed to send recovery notification", "error", err)
		}
	}
}

// IsLimited returns true if the system is currently rate limited.
func (h *RateLimitHandler) IsLimited() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return h.isLimited && time.Now().Before(h.rateLimitUntil)
}

// RecentlyLimited returns true if a rate limit occurred in the last 6 hours.
func (h *RateLimitHandler) RecentlyLimited() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.isLimited && time.Now().Before(h.rateLimitUntil) {
		return true
	}

	if !h.rateLimitUntil.IsZero() {
		elapsed := time.Since(h.rateLimitUntil)
		return elapsed < RecentLimitWindow
	}

	return false
}

// DetectRateLimit analyzes execution results to detect rate limit conditions.
func DetectRateLimit(exitCode int, stderr string, jsonResult string) bool {
	if exitCode == 429 {
		return true
	}

	lower := strings.ToLower(stderr)
	rateLimitPatterns := []string{
		"rate limit",
		"too many requests",
		"429",
		"capacity",
		"try again",
		"overloaded",
	}

	for _, pattern := range rateLimitPatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}

	if jsonResult != "" {
		lowerJSON := strings.ToLower(jsonResult)
		hasError := strings.Contains(lowerJSON, `"is_error":true`) ||
			strings.Contains(lowerJSON, `"is_error": true`)

		if hasError {
			for _, pattern := range rateLimitPatterns {
				if strings.Contains(lowerJSON, pattern) {
					return true
				}
			}
		}
	}

	return false
}

// queryCCUsageResetTime runs the ccusage CLI and parses the output for a reset time.
func queryCCUsageResetTime(cmd string) (time.Time, error) {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return time.Time{}, fmt.Errorf("empty ccusage command")
	}

	c := exec.Command(parts[0], parts[1:]...)
	var stdout, stderr bytes.Buffer
	c.Stdout = &stdout
	c.Stderr = &stderr

	if err := c.Run(); err != nil {
		return time.Time{}, fmt.Errorf("run ccusage: %w (stderr: %s)", err, stderr.String())
	}

	output := strings.TrimSpace(stdout.String())

	// Try parsing as RFC3339 timestamp
	if t, err := time.Parse(time.RFC3339, output); err == nil {
		if t.After(time.Now()) {
			return t, nil
		}
	}

	// Try parsing as a Go duration and add to now
	if d, err := time.ParseDuration(output); err == nil && d > 0 {
		return time.Now().Add(d), nil
	}

	// Scan lines for reset-related content
	for _, line := range strings.Split(output, "\n") {
		lowerLine := strings.ToLower(line)
		if !strings.Contains(lowerLine, "reset") {
			continue
		}
		if t, err := parseResetTime(line); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("no reset time found in ccusage output")
}

// parseResetTime tries several common formats to extract a time from a line of text.
func parseResetTime(line string) (time.Time, error) {
	formats := []string{
		time.RFC3339,
		"2006-01-02 15:04:05 MST",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
	}

	fields := strings.Fields(line)
	for i := range fields {
		for end := i + 1; end <= len(fields) && end <= i+4; end++ {
			candidate := strings.Join(fields[i:end], " ")
			candidate = strings.Trim(candidate, ",:;()[]")
			for _, format := range formats {
				if t, err := time.Parse(format, candidate); err == nil {
					return t, nil
				}
			}
		}
	}

	return time.Time{}, fmt.Errorf("no parseable time in: %s", line)
}
