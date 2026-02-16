package vault

import (
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/circle-oo/flux/internal/notesmd"
)

// operationalMode represents the vault's operational state.
type operationalMode int

const (
	modeFull     operationalMode = iota // notesmd-cli available
	modeFallback                        // direct file I/O
	modeDegraded                        // nothing works
)

const (
	retryAttempts = 3
	retryDelay    = 2 * time.Second
)

// Facade wraps notesmd.Client and provides multi-mode vault access.
// It implements both VaultReader and VaultWriter interfaces.
type Facade struct {
	client    *notesmd.Client
	writer    *Writer
	vaultPath string
	mode      operationalMode
	dailyMu   sync.Mutex
}

// NewFacade creates a Facade, detecting the best available mode.
func NewFacade(client *notesmd.Client, vaultPath string) *Facade {
	f := &Facade{
		client:    client,
		vaultPath: vaultPath,
		mode:      modeDegraded,
	}

	// Detect mode: try notesmd list first
	if client != nil {
		if _, err := client.List(""); err == nil {
			f.mode = modeFull
			f.writer = NewWriter(client)
			slog.Info("vault facade initialized", "mode", "full")
			return f
		}
		slog.Warn("notesmd-cli unavailable, trying fallback", "path", vaultPath)
	}

	// Try fallback: check vault path exists
	if vaultPath != "" {
		if _, err := fallbackList(vaultPath, ""); err == nil {
			f.mode = modeFallback
			slog.Info("vault facade initialized", "mode", "fallback")
			return f
		}
	}

	slog.Warn("vault facade initialized in degraded mode")
	return f
}

// Read returns the content of a note.
func (f *Facade) Read(path string) (string, error) {
	if f.mode == modeDegraded {
		return "", fmt.Errorf("vault is in degraded mode")
	}

	if f.mode == modeFull {
		return f.withRetry(func() (string, error) {
			return f.client.Print(path)
		})
	}

	return fallbackRead(f.vaultPath, path)
}

// List returns note paths under the given folder.
func (f *Facade) List(folder string) ([]string, error) {
	if f.mode == modeDegraded {
		return nil, fmt.Errorf("vault is in degraded mode")
	}

	if f.mode == modeFull {
		var result []string
		_, err := f.withRetry(func() (string, error) {
			var listErr error
			result, listErr = f.client.List(folder)
			if listErr != nil {
				return "", listErr
			}
			return "", nil
		})
		if err != nil {
			return nil, err
		}
		return result, nil
	}

	return fallbackList(f.vaultPath, folder)
}

// Search performs full-text search across vault notes.
func (f *Facade) Search(query string) (string, error) {
	if f.mode == modeDegraded {
		return "", fmt.Errorf("vault is in degraded mode")
	}

	if f.mode == modeFull {
		return f.withRetry(func() (string, error) {
			return f.client.SearchContent(query)
		})
	}

	return fallbackSearch(f.vaultPath, query)
}

// Frontmatter returns the frontmatter of a note.
func (f *Facade) Frontmatter(path string) (string, error) {
	if f.mode == modeDegraded {
		return "", fmt.Errorf("vault is in degraded mode")
	}

	if f.mode == modeFull {
		return f.withRetry(func() (string, error) {
			return f.client.Frontmatter(path)
		})
	}

	// Fallback: read full content and extract frontmatter
	content, err := fallbackRead(f.vaultPath, path)
	if err != nil {
		return "", err
	}
	return extractFrontmatter(content), nil
}

// Write writes content to a vault note with the given mode.
func (f *Facade) Write(path, content string, mode WriteMode) error {
	if f.mode == modeDegraded {
		return fmt.Errorf("vault is in degraded mode")
	}

	if f.mode == modeFull {
		err := f.writer.Write(path, content, mode)
		if err != nil {
			return err
		}
		// Post-write verification
		readBack, readErr := f.client.Print(path)
		if readErr != nil {
			slog.Warn("post-write verification read failed", "path", path, "error", readErr)
		} else if mode == ModeReplace && strings.TrimSpace(readBack) != strings.TrimSpace(content) {
			slog.Warn("post-write verification mismatch", "path", path)
		}
		return nil
	}

	return fallbackWrite(f.vaultPath, path, content, mode)
}

// DailyAppend appends content to today's daily note (thread-safe).
func (f *Facade) DailyAppend(content string) error {
	f.dailyMu.Lock()
	defer f.dailyMu.Unlock()

	if f.mode == modeDegraded {
		return fmt.Errorf("vault is in degraded mode")
	}

	today := time.Now().Format("2006-01-02")
	dailyPath := fmt.Sprintf("daily/%s", today)

	if f.mode == modeFull {
		return f.writer.Write(dailyPath, content, ModeAppend)
	}

	return fallbackWrite(f.vaultPath, dailyPath, content, ModeAppend)
}

// IsHealthy returns true if mode is Full or Fallback.
func (f *Facade) IsHealthy() bool {
	return f.mode != modeDegraded
}

// Mode returns the current operational mode as a string.
func (f *Facade) Mode() string {
	switch f.mode {
	case modeFull:
		return "full"
	case modeFallback:
		return "fallback"
	default:
		return "degraded"
	}
}

// Close shuts down the writer and flushes pending operations.
func (f *Facade) Close() {
	if f.writer != nil {
		f.writer.Close()
	}
}

// Delete removes a note from the vault.
func (f *Facade) Delete(path string) error {
	if f.mode == modeDegraded {
		return fmt.Errorf("vault is in degraded mode")
	}

	if f.mode == modeFull {
		_, err := f.withRetry(func() (string, error) {
			return "", f.client.Delete(path)
		})
		return err
	}

	full := resolveNotePath(f.vaultPath, path)
	return removeFile(full)
}

// Daily returns today's daily note content.
func (f *Facade) Daily() (string, error) {
	today := time.Now().Format("2006-01-02")
	dailyPath := fmt.Sprintf("daily/%s", today)
	return f.Read(dailyPath)
}

// withRetry retries an operation up to retryAttempts times.
func (f *Facade) withRetry(fn func() (string, error)) (string, error) {
	var lastErr error
	for i := 0; i < retryAttempts; i++ {
		result, err := fn()
		if err == nil {
			return result, nil
		}
		lastErr = err
		if i < retryAttempts-1 {
			time.Sleep(retryDelay)
		}
	}
	return "", fmt.Errorf("after %d attempts: %w", retryAttempts, lastErr)
}

// extractFrontmatter extracts YAML frontmatter between --- delimiters.
func extractFrontmatter(content string) string {
	if !strings.HasPrefix(content, "---") {
		return ""
	}
	end := strings.Index(content[3:], "---")
	if end < 0 {
		return ""
	}
	return strings.TrimSpace(content[3 : end+3])
}

// removeFile deletes a file from disk.
func removeFile(path string) error {
	return fmt.Errorf("fallback delete not supported: %s", path)
}
