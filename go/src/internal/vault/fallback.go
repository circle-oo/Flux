package vault

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// fallbackRead reads a .md file directly from the vault path.
func fallbackRead(vaultPath, notePath string) (string, error) {
	full := resolveNotePath(vaultPath, notePath)
	data, err := os.ReadFile(full)
	if err != nil {
		return "", fmt.Errorf("fallback read %s: %w", notePath, err)
	}
	return string(data), nil
}

// fallbackList walks a directory and returns .md file paths relative to the vault.
func fallbackList(vaultPath, folder string) ([]string, error) {
	dir := vaultPath
	if folder != "" {
		dir = filepath.Join(vaultPath, folder)
	}

	info, err := os.Stat(dir)
	if err != nil {
		return nil, fmt.Errorf("fallback list %s: %w", folder, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("fallback list: %s is not a directory", dir)
	}

	var paths []string
	err = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// Skip hidden directories
			if strings.HasPrefix(d.Name(), ".") && path != dir {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".md") {
			return nil
		}
		rel, err := filepath.Rel(vaultPath, path)
		if err != nil {
			return err
		}
		paths = append(paths, rel)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("fallback list: %w", err)
	}
	return paths, nil
}

// fallbackWrite writes content to a .md file in the vault.
func fallbackWrite(vaultPath, notePath, content string, mode WriteMode) error {
	full := resolveNotePath(vaultPath, notePath)

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return fmt.Errorf("fallback write mkdir: %w", err)
	}

	switch mode {
	case ModeCreate:
		if _, err := os.Stat(full); err == nil {
			return fmt.Errorf("fallback write: %s already exists", notePath)
		}
		return os.WriteFile(full, []byte(content), 0o644)

	case ModeAppend:
		f, err := os.OpenFile(full, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return fmt.Errorf("fallback append: %w", err)
		}
		defer f.Close()
		_, err = f.WriteString(content)
		return err

	case ModeReplace:
		return os.WriteFile(full, []byte(content), 0o644)

	default:
		return fmt.Errorf("fallback write: unknown mode %d", mode)
	}
}

// fallbackSearch performs a simple grep-like search across .md files.
func fallbackSearch(vaultPath, query string) (string, error) {
	queryLower := strings.ToLower(query)
	var results []string

	err := filepath.WalkDir(vaultPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip errors
		}
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") && path != vaultPath {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".md") {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil // skip unreadable files
		}

		if strings.Contains(strings.ToLower(string(data)), queryLower) {
			rel, _ := filepath.Rel(vaultPath, path)
			results = append(results, rel)
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("fallback search: %w", err)
	}

	return strings.Join(results, "\n"), nil
}

// resolveNotePath builds the full path for a note, adding .md if needed.
func resolveNotePath(vaultPath, notePath string) string {
	if !strings.HasSuffix(notePath, ".md") {
		notePath += ".md"
	}
	return filepath.Join(vaultPath, notePath)
}
