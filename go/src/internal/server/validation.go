package server

import (
	"errors"
	"fmt"
	"strings"
)

const (
	maxTitleLength       = 500
	maxDescriptionLength = 51200 // 50KB — task descriptions can be lengthy
	maxPromptLength      = 10240 // 10KB
)

var (
	// Shell injection patterns (check only in Prompt field where shell execution may occur)
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
		return fmt.Errorf("title exceeds maximum length of %d characters", maxTitleLength)
	}

	// Description validation
	if len(description) > maxDescriptionLength {
		return fmt.Errorf("description exceeds maximum length of %d characters (%d provided)", maxDescriptionLength, len(description))
	}

	return nil
}

// ValidatePrompt validates LLM prompt input and checks for shell injection
func ValidatePrompt(prompt string) error {
	if strings.TrimSpace(prompt) == "" {
		return errors.New("prompt is required")
	}
	if len(prompt) > maxPromptLength {
		return errors.New("prompt exceeds maximum length of 10KB")
	}

	// Check for shell injection patterns in prompts (where execution may occur)
	if containsSuspiciousShellPattern(prompt) {
		return errors.New("potentially malicious shell pattern detected in prompt")
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
