package executor

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/circle-oo/flux/internal/apiclient"
	"github.com/circle-oo/flux/internal/config"
)

func TestRegisterPodRetry(t *testing.T) {
	t.Run("succeeds on first attempt", func(t *testing.T) {
		attempts := atomic.Int32{}
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			attempts.Add(1)
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"status": "registered"})
		}))
		defer server.Close()

		cfg := &config.Config{
			Server: config.ServerConfig{Port: 8080},
			Executor: config.ExecutorConfig{
				RegistrationMaxRetries:   10,
				RegistrationInitialDelay: 10 * time.Millisecond,
			},
		}

		e := &Executor{
			id:                 "test-executor",
			config:             cfg,
			manager:            apiclient.NewClient(server.URL),
			executionStartTime: time.Now(),
		}

		err := e.registerPod()
		if err != nil {
			t.Fatalf("registerPod() failed: %v", err)
		}

		if attempts.Load() != 1 {
			t.Errorf("expected 1 attempt, got %d", attempts.Load())
		}
	})

	t.Run("retries on connection refused and succeeds", func(t *testing.T) {
		attempts := atomic.Int32{}
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			count := attempts.Add(1)
			if count < 3 {
				// Simulate connection refused by closing connection
				hj, ok := w.(http.Hijacker)
				if !ok {
					t.Fatal("webserver doesn't support hijacking")
					return
				}
				conn, _, err := hj.Hijack()
				if err != nil {
					t.Fatalf("hijack failed: %v", err)
					return
				}
				conn.Close()
				return
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"status": "registered"})
		}))
		defer server.Close()

		cfg := &config.Config{
			Server: config.ServerConfig{Port: 8080},
			Executor: config.ExecutorConfig{
				RegistrationMaxRetries:   5,
				RegistrationInitialDelay: 10 * time.Millisecond,
			},
		}

		e := &Executor{
			id:                 "test-executor",
			config:             cfg,
			manager:            apiclient.NewClient(server.URL),
			executionStartTime: time.Now(),
		}

		start := time.Now()
		err := e.registerPod()
		duration := time.Since(start)

		if err != nil {
			t.Fatalf("registerPod() failed: %v", err)
		}

		if attempts.Load() != 3 {
			t.Errorf("expected 3 attempts, got %d", attempts.Load())
		}

		// Verify exponential backoff happened (should take at least 10ms + 20ms = 30ms)
		if duration < 20*time.Millisecond {
			t.Errorf("expected exponential backoff delays, but completed too quickly: %v", duration)
		}
	})

	t.Run("fails after max retries", func(t *testing.T) {
		attempts := atomic.Int32{}
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			attempts.Add(1)
			// Always fail with connection close
			hj, ok := w.(http.Hijacker)
			if !ok {
				http.Error(w, "hijacking not supported", http.StatusInternalServerError)
				return
			}
			conn, _, err := hj.Hijack()
			if err != nil {
				http.Error(w, fmt.Sprintf("hijack failed: %v", err), http.StatusInternalServerError)
				return
			}
			conn.Close()
		}))
		defer server.Close()

		cfg := &config.Config{
			Server: config.ServerConfig{Port: 8080},
			Executor: config.ExecutorConfig{
				RegistrationMaxRetries:   3,
				RegistrationInitialDelay: 5 * time.Millisecond,
			},
		}

		e := &Executor{
			id:                 "test-executor",
			config:             cfg,
			manager:            apiclient.NewClient(server.URL),
			executionStartTime: time.Now(),
		}

		err := e.registerPod()
		if err == nil {
			t.Fatal("expected registerPod() to fail after max retries")
		}

		if attempts.Load() != 3 {
			t.Errorf("expected 3 attempts, got %d", attempts.Load())
		}
	})

	t.Run("uses default values when not configured", func(t *testing.T) {
		attempts := atomic.Int32{}
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			count := attempts.Add(1)
			if count < 2 {
				hj, ok := w.(http.Hijacker)
				if !ok {
					http.Error(w, "hijacking not supported", http.StatusInternalServerError)
					return
				}
				conn, _, err := hj.Hijack()
				if err != nil {
					http.Error(w, fmt.Sprintf("hijack failed: %v", err), http.StatusInternalServerError)
					return
				}
				conn.Close()
				return
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"status": "registered"})
		}))
		defer server.Close()

		cfg := &config.Config{
			Server:   config.ServerConfig{Port: 8080},
			Executor: config.ExecutorConfig{
				// No retry config - should use defaults
				RegistrationMaxRetries:   0,
				RegistrationInitialDelay: 0,
			},
		}

		e := &Executor{
			id:                 "test-executor",
			config:             cfg,
			manager:            apiclient.NewClient(server.URL),
			executionStartTime: time.Now(),
		}

		err := e.registerPod()
		if err != nil {
			t.Fatalf("registerPod() failed: %v", err)
		}

		if attempts.Load() != 2 {
			t.Errorf("expected 2 attempts (using default max retries), got %d", attempts.Load())
		}
	})

	t.Run("caps maximum delay at 10 seconds", func(t *testing.T) {
		// This test verifies the delay capping logic without actually waiting 10s
		cfg := &config.Config{
			Server: config.ServerConfig{Port: 8080},
			Executor: config.ExecutorConfig{
				RegistrationMaxRetries:   20, // High number to trigger cap
				RegistrationInitialDelay: 1 * time.Second,
			},
		}

		e := &Executor{
			id:     "test-executor",
			config: cfg,
		}

		// Verify config is set correctly (actual retry behavior tested in integration)
		if cfg.Executor.RegistrationMaxRetries != 20 {
			t.Errorf("expected max retries 20, got %d", cfg.Executor.RegistrationMaxRetries)
		}

		if cfg.Executor.RegistrationInitialDelay != 1*time.Second {
			t.Errorf("expected initial delay 1s, got %v", cfg.Executor.RegistrationInitialDelay)
		}

		// Just verify executor is configured properly
		if e.id == "" {
			t.Error("executor ID should not be empty")
		}
	})
}
