package server

import (
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/circle-oo/flux/internal/config"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

const (
	defaultSessionTTL = 24 * time.Hour
	rateLimitWindow   = 1 * time.Hour
	authCleanupEvery  = 1 * time.Hour
	maxFailedAttempts = 5
)

// AuthManager handles session-based authentication.
type AuthManager struct {
	config       config.AuthConfig
	passwordHash []byte
	sessions     map[string]time.Time
	sessionTTL   time.Duration
	mu           sync.RWMutex

	// Login rate limiting: IP -> []failedAttemptTime
	loginAttempts   map[string][]time.Time
	loginAttemptsMu sync.Mutex
}

// NewAuthManager creates a new AuthManager. It hashes the plaintext password at startup.
func NewAuthManager(cfg config.AuthConfig) *AuthManager {
	sessionTTL := parseSessionTTL(cfg.SessionExpiry)
	am := &AuthManager{
		config:        cfg,
		sessions:      make(map[string]time.Time),
		sessionTTL:    sessionTTL,
		loginAttempts: make(map[string][]time.Time),
	}

	if cfg.Password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(cfg.Password), bcrypt.DefaultCost)
		if err != nil {
			slog.Error("failed to hash password", "error", err)
		} else {
			am.passwordHash = hash
		}
	}

	go am.cleanupLoop()

	return am
}

// ValidateSession checks if a session token is valid.
func (am *AuthManager) ValidateSession(token string) bool {
	am.mu.RLock()
	createdAt, exists := am.sessions[token]
	sessionTTL := am.sessionTTL
	am.mu.RUnlock()
	if !exists {
		return false
	}
	if sessionTTL <= 0 {
		return true
	}
	if time.Since(createdAt) <= sessionTTL {
		return true
	}

	am.InvalidateSession(token)
	return false
}

// CreateSession creates a new session and returns the token.
func (am *AuthManager) CreateSession() string {
	token := uuid.New().String()
	am.mu.Lock()
	am.sessions[token] = time.Now()
	am.mu.Unlock()
	return token
}

// InvalidateSession removes a session.
func (am *AuthManager) InvalidateSession(token string) {
	am.mu.Lock()
	delete(am.sessions, token)
	am.mu.Unlock()
}

// CheckPassword compares the given password against the stored bcrypt hash.
func (am *AuthManager) CheckPassword(password string) bool {
	if len(am.passwordHash) == 0 {
		return false
	}
	return bcrypt.CompareHashAndPassword(am.passwordHash, []byte(password)) == nil
}

// IsRateLimited checks if the given IP has exceeded login attempt limits.
// Returns true if the IP is rate-limited (5 failed attempts in the last hour).
func (am *AuthManager) IsRateLimited(ip string) bool {
	am.loginAttemptsMu.Lock()
	defer am.loginAttemptsMu.Unlock()

	cutoff := time.Now().Add(-rateLimitWindow)
	attempts := am.loginAttempts[ip]

	// Prune old attempts
	valid := attempts[:0]
	for _, t := range attempts {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}
	am.loginAttempts[ip] = valid

	return len(valid) >= maxFailedAttempts
}

// RecordFailedLogin records a failed login attempt for the given IP.
func (am *AuthManager) RecordFailedLogin(ip string) {
	am.loginAttemptsMu.Lock()
	defer am.loginAttemptsMu.Unlock()
	am.loginAttempts[ip] = append(am.loginAttempts[ip], time.Now())
}

func parseSessionTTL(raw string) time.Duration {
	value := strings.TrimSpace(strings.ToLower(raw))
	if value == "" {
		return defaultSessionTTL
	}
	if value == "none" || value == "0" {
		return 0
	}

	ttl, err := time.ParseDuration(raw)
	if err != nil {
		slog.Warn("invalid auth session_expiry; using default", "value", raw, "default", defaultSessionTTL, "error", err)
		return defaultSessionTTL
	}
	if ttl <= 0 {
		return defaultSessionTTL
	}
	return ttl
}

func (am *AuthManager) cleanupLoop() {
	ticker := time.NewTicker(authCleanupEvery)
	defer ticker.Stop()

	for range ticker.C {
		am.cleanupStaleData(time.Now())
	}
}

func (am *AuthManager) cleanupStaleData(now time.Time) {
	attemptCutoff := now.Add(-rateLimitWindow)

	am.loginAttemptsMu.Lock()
	for ip, attempts := range am.loginAttempts {
		valid := attempts[:0]
		for _, t := range attempts {
			if t.After(attemptCutoff) {
				valid = append(valid, t)
			}
		}
		if len(valid) == 0 {
			delete(am.loginAttempts, ip)
			continue
		}
		am.loginAttempts[ip] = valid
	}
	am.loginAttemptsMu.Unlock()

	if am.sessionTTL <= 0 {
		return
	}

	sessionCutoff := now.Add(-am.sessionTTL)
	am.mu.Lock()
	for token, createdAt := range am.sessions {
		if createdAt.Before(sessionCutoff) {
			delete(am.sessions, token)
		}
	}
	am.mu.Unlock()
}

// authMiddleware returns middleware that requires a valid session cookie.
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.config.Server.Auth.Enabled {
			next.ServeHTTP(w, r)
			return
		}

		cookie, err := r.Cookie("flux_session")
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		if !s.auth.ValidateSession(cookie.Value) {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// handleLogin handles POST /api/auth/login
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Password string `json:"password"`
	}
	if err := readJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Extract client IP for rate limiting
	ip, _, _ := extractIP(r)

	if s.auth.IsRateLimited(ip) {
		writeError(w, http.StatusTooManyRequests, "too many login attempts")
		return
	}

	if !s.auth.CheckPassword(req.Password) {
		s.auth.RecordFailedLogin(ip)
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	token := s.auth.CreateSession()

	http.SetCookie(w, &http.Cookie{
		Name:     "flux_session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleLogout handles POST /api/auth/logout
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("flux_session")
	if err == nil {
		s.auth.InvalidateSession(cookie.Value)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "flux_session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// extractIP gets the client IP from the request using only RemoteAddr.
// X-Forwarded-For is not trusted since Flux is designed for local/Tailscale use.
func extractIP(r *http.Request) (string, string, error) {
	host, port, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		// Fall back to RemoteAddr directly if SplitHostPort fails
		return r.RemoteAddr, "", nil
	}
	return host, port, nil
}
