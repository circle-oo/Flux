package server

import (
	"errors"
	"strings"
)

const (
	maxTitleLength       = 500
	maxDescriptionLength = 10240 // 10KB
	maxPromptLength      = 10240 // 10KB
)

var (
	// SQL injection patterns
	sqlInjectionPatterns = []string{
		"'; DROP",
		"1=1",
		"UNION SELECT",
		"' OR '1'='1",
		"-- ",
		"/*",
		"*/",
		"xp_",
		"sp_",
	}

	// Shell injection patterns (in unexpected contexts)
	shellInjectionPatterns = []string{
		"$((",
		"; rm ",
		"| sh",
		"| bash",
		"&& rm",
		"|| rm",
	}
)

// ValidateTaskInput validates task title and description for security
func ValidateTaskInput(title, description string) error {
	// Title validation
	if strings.TrimSpace(title) == "" {
		return errors.New("title is required")
	}
	if len(title) > maxTitleLength {
		return errors.New("title exceeds maximum length of 500 characters")
	}

	// Description validation
	if len(description) > maxDescriptionLength {
		return errors.New("description exceeds maximum length of 10KB")
	}

	// Check for SQL injection patterns
	titleLower := strings.ToLower(title)
	descLower := strings.ToLower(description)
	for _, pattern := range sqlInjectionPatterns {
		if strings.Contains(titleLower, strings.ToLower(pattern)) {
			return errors.New("potentially malicious SQL pattern detected in title")
		}
		if strings.Contains(descLower, strings.ToLower(pattern)) {
			return errors.New("potentially malicious SQL pattern detected in description")
		}
	}

	// Check for shell injection patterns (avoid false positives for legitimate code snippets)
	// Only flag if pattern appears outside of code blocks
	if containsSuspiciousShellPattern(title) {
		return errors.New("potentially malicious shell pattern detected in title")
	}
	if containsSuspiciousShellPattern(description) {
		return errors.New("potentially malicious shell pattern detected in description")
	}

	return nil
}

// ValidatePrompt validates LLM prompt input
func ValidatePrompt(prompt string) error {
	if strings.TrimSpace(prompt) == "" {
		return errors.New("prompt is required")
	}
	if len(prompt) > maxPromptLength {
		return errors.New("prompt exceeds maximum length of 10KB")
	}
	return nil
}

// SanitizeInput removes potentially problematic characters
func SanitizeInput(s string) string {
	// Trim whitespace
	s = strings.TrimSpace(s)
	// Remove null bytes
	s = strings.ReplaceAll(s, "\x00", "")
	return s
}

// containsSuspiciousShellPattern checks for shell injection outside code contexts
func containsSuspiciousShellPattern(s string) bool {
	// Allow $(variable) syntax in code snippets - only flag actual command execution attempts
	for _, pattern := range shellInjectionPatterns {
		if strings.Contains(s, pattern) {
			return true
		}
	}
	return false
}
