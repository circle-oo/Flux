package orchestrator

import (
	"database/sql"
	"fmt"
	"log/slog"
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
type RateLimitHandler struct {
	db             *sql.DB
	notifier       *notifier.Discord
	rateLimitUntil time.Time
	isLimited      bool
	mu             sync.RWMutex
}

// NewRateLimitHandler creates a new rate limit handler.
func NewRateLimitHandler(db *sql.DB, notifier *notifier.Discord) *RateLimitHandler {
	return &RateLimitHandler{
		db:       db,
		notifier: notifier,
	}
}

// HandleRateLimit records a rate limit event and sets rate limit state.
// This is a non-blocking state-based response: pods check IsLimited() before requesting tasks.
func (h *RateLimitHandler) HandleRateLimit() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.rateLimitUntil = time.Now().Add(RateLimitDuration)
	h.isLimited = true

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
		TokensUsed: 0, // Not tracking individual event tokens
		ActivePods: 0, // Will be set by orchestrator if needed
	}

	if err := store.CreateRateLimitEvent(event); err != nil {
		slog.Error("failed to record rate limit event", "error", err)
		return fmt.Errorf("record rate limit event: %w", err)
	}

	return nil
}

// CheckAndRecover checks if the rate limit window has expired and clears state if so.
// This should be called periodically by the orchestrator.
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
// Pods should check this before requesting new tasks.
func (h *RateLimitHandler) IsLimited() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return h.isLimited && time.Now().Before(h.rateLimitUntil)
}

// RecentlyLimited returns true if a rate limit occurred in the last 6 hours.
// This can be used for model selection to suppress Opus usage.
func (h *RateLimitHandler) RecentlyLimited() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// If currently limited, return true
	if h.isLimited && time.Now().Before(h.rateLimitUntil) {
		return true
	}

	// Check if the most recent rate limit was within the lookback window
	if !h.rateLimitUntil.IsZero() {
		elapsed := time.Since(h.rateLimitUntil)
		return elapsed < RecentLimitWindow
	}

	return false
}

// DetectRateLimit analyzes execution results to detect rate limit conditions.
// This implements a three-stage detection strategy based on Phase 2A experiment findings.
func DetectRateLimit(exitCode int, stderr string, jsonResult string) bool {
	// Stage 1: Exit code check
	if exitCode == 429 {
		return true
	}

	// Stage 2: stderr pattern matching (case insensitive)
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

	// Stage 3: JSON result analysis (check for is_error: true with rate limit in result text)
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
