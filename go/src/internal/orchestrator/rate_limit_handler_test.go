package orchestrator

import (
	"testing"
	"time"

	"github.com/circle-oo/flux/internal/models"
	"github.com/circle-oo/flux/internal/notifier"
	"github.com/circle-oo/flux/internal/testutil"
)

func TestHandleRateLimit(t *testing.T) {
	db := testutil.NewTestDB(t)
	discord := notifier.NewDiscord("") // Empty webhook for test
	handler := NewRateLimitHandler(db, discord)

	// Initially not limited
	if handler.IsLimited() {
		t.Error("handler should not be limited initially")
	}

	// Trigger rate limit
	if err := handler.HandleRateLimit(); err != nil {
		t.Fatalf("HandleRateLimit failed: %v", err)
	}

	// Should now be limited
	if !handler.IsLimited() {
		t.Error("handler should be limited after HandleRateLimit")
	}

	// Check that rateLimitUntil is set correctly (within 5 hours from now)
	handler.mu.RLock()
	expectedUntil := time.Now().Add(RateLimitDuration)
	timeDiff := handler.rateLimitUntil.Sub(expectedUntil).Abs()
	handler.mu.RUnlock()

	if timeDiff > time.Second {
		t.Errorf("rateLimitUntil not set correctly, diff: %v", timeDiff)
	}

	// Verify event was recorded in database
	store := models.NewUsageStore(db)
	events, err := store.ListRateLimitEvents()
	if err != nil {
		t.Fatalf("ListRateLimitEvents failed: %v", err)
	}

	if len(events) != 1 {
		t.Errorf("expected 1 rate limit event, got %d", len(events))
	}
}

func TestIsLimited(t *testing.T) {
	db := testutil.NewTestDB(t)
	discord := notifier.NewDiscord("")
	handler := NewRateLimitHandler(db, discord)

	// Initially not limited
	if handler.IsLimited() {
		t.Error("handler should not be limited initially")
	}

	// Set rate limit manually for testing
	handler.mu.Lock()
	handler.isLimited = true
	handler.rateLimitUntil = time.Now().Add(1 * time.Hour)
	handler.mu.Unlock()

	// Should be limited within the window
	if !handler.IsLimited() {
		t.Error("handler should be limited within the 1-hour window")
	}

	// Set rate limit in the past
	handler.mu.Lock()
	handler.rateLimitUntil = time.Now().Add(-1 * time.Second)
	handler.mu.Unlock()

	// Should not be limited after the window expires
	if handler.IsLimited() {
		t.Error("handler should not be limited after window expires")
	}
}

func TestCheckAndRecover(t *testing.T) {
	db := testutil.NewTestDB(t)
	discord := notifier.NewDiscord("")
	handler := NewRateLimitHandler(db, discord)

	// Set rate limit that has expired
	handler.mu.Lock()
	handler.isLimited = true
	handler.rateLimitUntil = time.Now().Add(-1 * time.Second)
	handler.mu.Unlock()

	// Run recovery check
	handler.CheckAndRecover()

	// Should now be recovered
	handler.mu.RLock()
	isLimited := handler.isLimited
	handler.mu.RUnlock()

	if isLimited {
		t.Error("handler should be recovered after CheckAndRecover")
	}

	// Verify IsLimited returns false
	if handler.IsLimited() {
		t.Error("IsLimited should return false after recovery")
	}
}

func TestCheckAndRecover_NoRecoveryIfNotExpired(t *testing.T) {
	db := testutil.NewTestDB(t)
	discord := notifier.NewDiscord("")
	handler := NewRateLimitHandler(db, discord)

	// Set rate limit that has not expired
	handler.mu.Lock()
	handler.isLimited = true
	handler.rateLimitUntil = time.Now().Add(1 * time.Hour)
	handler.mu.Unlock()

	// Run recovery check
	handler.CheckAndRecover()

	// Should still be limited
	if !handler.IsLimited() {
		t.Error("handler should still be limited if window has not expired")
	}
}

func TestRecentlyLimited(t *testing.T) {
	db := testutil.NewTestDB(t)
	discord := notifier.NewDiscord("")
	handler := NewRateLimitHandler(db, discord)

	// Initially not recently limited
	if handler.RecentlyLimited() {
		t.Error("handler should not be recently limited initially")
	}

	// Set rate limit that is currently active
	handler.mu.Lock()
	handler.isLimited = true
	handler.rateLimitUntil = time.Now().Add(1 * time.Hour)
	handler.mu.Unlock()

	// Should be recently limited
	if !handler.RecentlyLimited() {
		t.Error("handler should be recently limited when currently active")
	}

	// Set rate limit that ended 3 hours ago (within 6-hour window)
	handler.mu.Lock()
	handler.isLimited = false
	handler.rateLimitUntil = time.Now().Add(-3 * time.Hour)
	handler.mu.Unlock()

	// Should still be recently limited
	if !handler.RecentlyLimited() {
		t.Error("handler should be recently limited within 6-hour window")
	}

	// Set rate limit that ended 7 hours ago (outside 6-hour window)
	handler.mu.Lock()
	handler.rateLimitUntil = time.Now().Add(-7 * time.Hour)
	handler.mu.Unlock()

	// Should not be recently limited
	if handler.RecentlyLimited() {
		t.Error("handler should not be recently limited after 6-hour window")
	}
}

func TestDetectRateLimit(t *testing.T) {
	tests := []struct {
		name       string
		exitCode   int
		stderr     string
		jsonResult string
		want       bool
	}{
		{
			name:     "exit code 429",
			exitCode: 429,
			stderr:   "",
			want:     true,
		},
		{
			name:     "stderr contains 'rate limit'",
			exitCode: 1,
			stderr:   "Error: rate limit exceeded",
			want:     true,
		},
		{
			name:     "stderr contains 'too many requests'",
			exitCode: 1,
			stderr:   "Too many requests. Please try again later.",
			want:     true,
		},
		{
			name:     "stderr contains '429'",
			exitCode: 1,
			stderr:   "HTTP 429 status code received",
			want:     true,
		},
		{
			name:     "stderr contains 'capacity'",
			exitCode: 1,
			stderr:   "Service at capacity, please wait",
			want:     true,
		},
		{
			name:     "stderr contains 'try again'",
			exitCode: 1,
			stderr:   "Please try again in a few minutes",
			want:     true,
		},
		{
			name:     "stderr contains 'overloaded'",
			exitCode: 1,
			stderr:   "System overloaded",
			want:     true,
		},
		{
			name:       "json result with is_error and rate limit",
			exitCode:   1,
			stderr:     "",
			jsonResult: `{"is_error":true,"result":"rate limit exceeded"}`,
			want:       true,
		},
		{
			name:       "json result with is_error and too many requests",
			exitCode:   1,
			stderr:     "",
			jsonResult: `{"is_error": true, "result": "Too many requests"}`,
			want:       true,
		},
		{
			name:       "json result with is_error but no rate limit pattern",
			exitCode:   1,
			stderr:     "",
			jsonResult: `{"is_error":true,"result":"unknown error"}`,
			want:       false,
		},
		{
			name:     "case insensitive matching",
			exitCode: 1,
			stderr:   "RATE LIMIT EXCEEDED",
			want:     true,
		},
		{
			name:     "no rate limit indicators",
			exitCode: 1,
			stderr:   "Some other error occurred",
			want:     false,
		},
		{
			name:     "success case",
			exitCode: 0,
			stderr:   "",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectRateLimit(tt.exitCode, tt.stderr, tt.jsonResult)
			if got != tt.want {
				t.Errorf("DetectRateLimit() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConcurrentIsLimited(t *testing.T) {
	db := testutil.NewTestDB(t)
	discord := notifier.NewDiscord("")
	handler := NewRateLimitHandler(db, discord)

	// Set rate limit
	handler.mu.Lock()
	handler.isLimited = true
	handler.rateLimitUntil = time.Now().Add(1 * time.Hour)
	handler.mu.Unlock()

	// Spawn multiple goroutines checking IsLimited concurrently
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				handler.IsLimited()
			}
			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// If we reach here without deadlock or data race, the test passes
	if !handler.IsLimited() {
		t.Error("handler should still be limited after concurrent reads")
	}
}
